//! Proposed OCI handoff: a content locator, independent of credentials and policy.
use super::*;
use crate::applications::Intent;
use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine};
use serde::{Deserialize, Serialize};

pub const PROTOCOL: &str = "oci-handoff/1";

#[derive(Serialize)]
struct Source {
    registry: String,
    repository: String,
    digest: String,
}

#[derive(Serialize)]
struct Handoff {
    protocol: &'static str,
    source: Source,
    intent: Intent,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Request {
    application_id: String,
    repository: String,
    reference: String,
    intent: Intent,
}

#[derive(Deserialize)]
struct Selection {
    repository: String,
    reference: String,
}

pub(super) fn router() -> Router<Arc<Service>> {
    Router::new()
        .route("/handoff/applications", get(applications))
        .route("/handoff/launches", axum::routing::post(launch))
        .route("/handoff/controls.js", get(controls))
        .route(
            "/handoff/controls.css",
            get(|| async {
                (
                    [(header::CONTENT_TYPE, "text/css; charset=utf-8")],
                    include_str!("handoff_ui.css"),
                )
            }),
        )
}

async fn controls() -> impl IntoResponse {
    (
        [(header::CONTENT_TYPE, "text/javascript; charset=utf-8")],
        include_str!("handoff_ui.js"),
    )
}

async fn applications(
    State(registry): State<Arc<Service>>,
    headers: HeaderMap,
    Query(selection): Query<Selection>,
) -> std::result::Result<Response, ApiError> {
    let claims = authorization(&headers)?;
    let (digest, _) = registry
        .source_manifest(&claims, &selection.repository, &selection.reference)
        .await?;
    let configuration = &registry.configuration;
    // Format interpretation belongs to the chosen application; no profile registry is required.
    let applications: Vec<_> = configuration
        .applications
        .iter()
        .map(|app| {
            json!({"id":app.id,"name":app.name,
        "launches":{"view":{"intent":"view"},"edit":{"intent":"edit"}}})
        })
        .collect();
    Ok(
        Json(json!({"protocol":PROTOCOL,"digest":digest,"applications":applications}))
            .into_response(),
    )
}

async fn launch(
    State(registry): State<Arc<Service>>,
    headers: HeaderMap,
    Json(request): Json<Request>,
) -> std::result::Result<Response, ApiError> {
    let claims = authorization(&headers)?;
    let (digest, _) = registry
        .source_manifest(&claims, &request.repository, &request.reference)
        .await?;
    let configuration = &registry.configuration;
    let app = configuration
        .applications
        .iter()
        .find(|a| a.id == request.application_id)
        .context("Choose a registered application")?;
    let handoff = Handoff {
        protocol: PROTOCOL,
        source: Source {
            registry: registry.configuration.registry.clone(),
            repository: request.repository,
            digest,
        },
        intent: request.intent,
    };
    let mut url = reqwest::Url::parse(&app.launch_url).map_err(ApiError::bad)?;
    url.set_fragment(Some(&format!(
        "handoff={}",
        URL_SAFE_NO_PAD.encode(serde_json::to_vec(&handoff).map_err(ApiError::bad)?)
    )));
    let mut response = Json(json!({"protocol":PROTOCOL,"launch_url":url.as_str()})).into_response();
    response
        .headers_mut()
        .insert(header::CACHE_CONTROL, "no-store".parse().unwrap());
    response
        .headers_mut()
        .insert("referrer-policy", "no-referrer".parse().unwrap());
    Ok(response)
}
