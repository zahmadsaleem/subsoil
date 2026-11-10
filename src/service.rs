//! Browsing and application handoffs over an independent OCI registry. Identity belongs to the configured authorization server.
use crate::{
    registry::{valid_repo, valid_tag, MANIFEST_TYPE},
    release::{digest, digest_hex},
};
use anyhow::{ensure, Context, Result};
use axum::{
    extract::{DefaultBodyLimit, Path as RoutePath, Query, State},
    http::{header, HeaderMap, StatusCode},
    response::{IntoResponse, Response},
    routing::get,
    Json, Router,
};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::sync::Arc;
use tokio::net::TcpListener;
mod brand;
mod explorer;
mod files;
mod handoff;

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct Configuration {
    pub registry: String,
    pub registry_upstream: String,
    pub authorization_server: String,
    pub client_id: String,
    pub scopes: Vec<String>,
    pub applications: Vec<crate::applications::Application>,
}
pub struct Service {
    configuration: Configuration,
    http: reqwest::Client,
}
impl Service {
    pub fn new(configuration: Configuration) -> Result<Arc<Self>> {
        crate::applications::web_url(&configuration.registry)?;
        let public = reqwest::Url::parse(&configuration.registry)?;
        ensure!(
            configuration.registry == public.origin().ascii_serialization(),
            "Use a registry origin without a trailing slash"
        );
        let issuer = crate::applications::web_url(&configuration.authorization_server)?;
        ensure!(
            issuer.query().is_none(),
            "Use the exact authorization issuer URL"
        );
        let upstream = reqwest::Url::parse(&configuration.registry_upstream)?;
        ensure!(
            matches!(upstream.scheme(), "http" | "https")
                && upstream.host_str().is_some()
                && upstream.username().is_empty()
                && upstream.password().is_none()
                && upstream.path() == "/"
                && upstream.query().is_none()
                && upstream.fragment().is_none(),
            "Configure an exact registry upstream origin"
        );
        ensure!(
            !configuration.client_id.is_empty() && !configuration.scopes.is_empty(),
            "Configure the browser's OAuth client and scopes"
        );
        let mut ids = std::collections::HashSet::new();
        for app in &configuration.applications {
            ensure!(
                !app.id.is_empty() && ids.insert(&app.id) && !app.name.trim().is_empty(),
                "Choose unique application IDs and names"
            );
            crate::applications::web_url(&app.launch_url)?;
        }
        Ok(Arc::new(Self {
            configuration,
            http: reqwest::Client::builder()
                .redirect(reqwest::redirect::Policy::none())
                .timeout(std::time::Duration::from_secs(30))
                .build()?,
        }))
    }
}
pub fn router(service: Arc<Service>) -> Router {
    Router::new()
        .merge(handoff::router())
        .merge(files::router())
        .merge(brand::router())
        .merge(explorer::router())
        .route("/healthz", get(|| async { StatusCode::OK }))
        .route("/readyz", get(files::ready))
        .route(
            "/",
            get(|| async { axum::response::Redirect::to("/explore") }),
        )
        .layer(DefaultBodyLimit::max(32 * 1024))
        .with_state(service)
}
pub async fn serve(listener: TcpListener, service: Arc<Service>) -> Result<()> {
    serve_with_shutdown(listener, service, std::future::pending()).await
}
pub async fn serve_with_shutdown(
    listener: TcpListener,
    service: Arc<Service>,
    shutdown: impl std::future::Future<Output = ()> + Send + 'static,
) -> Result<()> {
    axum::serve(listener, router(service))
        .with_graceful_shutdown(shutdown)
        .await?;
    Ok(())
}
#[derive(Debug)]
struct ApiError(StatusCode, &'static str, String);
impl ApiError {
    fn bad(message: impl ToString) -> Self {
        Self(
            StatusCode::BAD_REQUEST,
            "INVALID_REQUEST",
            message.to_string(),
        )
    }
}
impl From<anyhow::Error> for ApiError {
    fn from(_: anyhow::Error) -> Self {
        Self(
            StatusCode::BAD_GATEWAY,
            "FILE_STORE_UNAVAILABLE",
            "Could not read these files. Refresh or ask the owner for access.".into(),
        )
    }
}
impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        (self.0, Json(json!({"error":self.1,"message":self.2}))).into_response()
    }
}
fn authorization(headers: &HeaderMap) -> std::result::Result<String, ApiError> {
    headers
        .get(header::AUTHORIZATION)
        .and_then(|h| h.to_str().ok())
        .filter(|h| h.starts_with("Bearer ") && h.len() > 7)
        .map(str::to_owned)
        .ok_or_else(|| {
            ApiError(
                StatusCode::UNAUTHORIZED,
                "SIGN_IN_REQUIRED",
                "Sign in to browse your files".into(),
            )
        })
}
