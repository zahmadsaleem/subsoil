use anyhow::Result;
use axum::{
    http::{HeaderMap, StatusCode},
    routing::get,
    Json, Router,
};
use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine};
use serde_json::{json, Value};
use subsoil::{
    applications::Application,
    registry,
    release::digest,
    service::{self, Configuration, Service},
};

#[tokio::test]
async fn handoff_reads_an_independent_registry_and_contains_only_a_content_locator() -> Result<()> {
    let bytes = serde_json::to_vec(
        &json!({"schemaVersion":2,"mediaType":registry::MANIFEST_TYPE,"config":{"mediaType":"application/vnd.oci.empty.v1+json","digest":digest(b"{}"),"size":2},"layers":[]}),
    )?;
    let fingerprint = digest(&bytes);
    let expected = bytes.clone();
    let zot = Router::new()
        .route(
            "/v2/project/model/manifests/issued",
            get(move |headers: HeaderMap| {
                let bytes = expected.clone();
                async move {
                    if headers
                        .get("authorization")
                        .is_some_and(|h| h == "Bearer external-access")
                    {
                        (StatusCode::OK, bytes)
                    } else {
                        (StatusCode::UNAUTHORIZED, Vec::new())
                    }
                }
            }),
        )
        .route(
            "/v2/_catalog",
            get(|| async { Json(json!({"repositories":["project/model"]})) }),
        );
    let backend = registry::bind("127.0.0.1:0".parse()?).await?;
    let backend_url = format!("http://{}", backend.local_addr()?);
    let backend_task = tokio::spawn(async move { axum::serve(backend, zot).await });
    let service = Service::new(Configuration {
        registry: "http://localhost:5001".into(),
        registry_upstream: backend_url,
        authorization_server: "http://localhost:8080/realms/demo".into(),
        client_id: "subsoil-browser".into(),
        scopes: vec!["repository:project/model:pull".into()],
        applications: vec![Application {
            id: "ifc-lite".into(),
            name: "IFC Lite".into(),
            launch_url: "http://localhost:3000/open-artifact".into(),
        }],
    })?;
    let listener = registry::bind("127.0.0.1:0".parse()?).await?;
    let origin = format!("http://{}", listener.local_addr()?);
    let server = tokio::spawn(service::serve(listener, service));
    struct Stop(
        tokio::task::JoinHandle<Result<()>>,
        tokio::task::JoinHandle<std::io::Result<()>>,
    );
    impl Drop for Stop {
        fn drop(&mut self) {
            self.0.abort();
            self.1.abort();
        }
    }
    let _stop = Stop(server, backend_task);
    let http = reqwest::Client::new();
    let selection = json!({"application_id":"ifc-lite","repository":"project/model","reference":"issued","intent":"edit"});
    for (credential, status) in [
        (None, 401),
        (Some("wrong"), 401),
        (Some("external-access"), 200),
    ] {
        let mut request = http
            .post(format!("{origin}/handoff/launches"))
            .json(&selection);
        if let Some(token) = credential {
            request = request.bearer_auth(token);
        }
        let response = request.send().await?;
        assert_eq!(response.status().as_u16(), status);
        if status == 200 {
            assert_eq!(response.headers()["cache-control"], "no-store");
            let reply: Value = response.json().await?;
            let url = reqwest::Url::parse(reply["launch_url"].as_str().unwrap())?;
            assert_eq!(url.origin().ascii_serialization(), "http://localhost:3000");
            let payload = URL_SAFE_NO_PAD
                .decode(url.fragment().unwrap().strip_prefix("handoff=").unwrap())?;
            let payload: Value = serde_json::from_slice(&payload)?;
            assert_eq!(
                payload,
                json!({"protocol":"oci-handoff/1","source":{"registry":"http://localhost:5001","repository":"project/model","digest":fingerprint},"intent":"edit"})
            );
        }
    }
    let manifest = http
        .get(format!("{origin}/files/project/model/manifests/issued"))
        .bearer_auth("external-access")
        .send()
        .await?;
    assert_eq!(manifest.headers()["docker-content-digest"], fingerprint);
    assert_eq!(manifest.bytes().await?.as_ref(), bytes.as_slice());
    // Subsoil has neither an OCI storage API nor a local login/token issuer.
    for path in ["/v2/", "/oauth/token", "/token"] {
        assert_eq!(
            http.post(format!("{origin}{path}")).send().await?.status(),
            404
        );
    }
    Ok(())
}
