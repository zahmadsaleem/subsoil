//! Read-only workflow API. Every file and version is fetched from Zot through OCI APIs.
use super::*;
use serde::Deserialize;

impl Service {
    async fn upstream(
        &self,
        authorization: &str,
        path: &str,
    ) -> std::result::Result<reqwest::Response, ApiError> {
        let response = self
            .http
            .get(format!("{}{path}", self.configuration.registry_upstream))
            .header(header::AUTHORIZATION, authorization)
            .header(header::ACCEPT, MANIFEST_TYPE)
            .send()
            .await
            .map_err(|e| ApiError::from(anyhow::Error::from(e)))?;
        if !response.status().is_success() {
            let status = response.status();
            return Err(ApiError(
                if status == StatusCode::UNAUTHORIZED
                    || status == StatusCode::FORBIDDEN
                    || status == StatusCode::NOT_FOUND
                {
                    status
                } else {
                    StatusCode::BAD_GATEWAY
                },
                "FILE_STORE_REQUEST_FAILED",
                "These files are unavailable. Sign in again or ask the owner for access.".into(),
            ));
        }
        Ok(response)
    }
    pub(super) async fn source_manifest(
        &self,
        authorization: &str,
        repo: &str,
        reference: &str,
    ) -> std::result::Result<(String, Vec<u8>), ApiError> {
        valid_repo(repo)?;
        if reference.starts_with("sha256:") {
            digest_hex(reference)?;
        } else {
            valid_tag(reference)?;
        }
        let response = self
            .upstream(authorization, &format!("/v2/{repo}/manifests/{reference}"))
            .await?;
        let bytes = body(response, 1024 * 1024).await?;
        let fingerprint = digest(&bytes);
        if reference.starts_with("sha256:") && fingerprint != reference {
            return Err(ApiError::bad("The file store returned another version"));
        }
        Ok((fingerprint, bytes))
    }
    async fn names(
        &self,
        authorization: &str,
        repository: Option<&str>,
    ) -> std::result::Result<Vec<String>, ApiError> {
        let base = repository
            .map(|r| format!("/v2/{r}/tags/list"))
            .unwrap_or_else(|| "/v2/_catalog".into());
        let field = if repository.is_some() {
            "tags"
        } else {
            "repositories"
        };
        let mut names = Vec::new();
        let mut last = String::new();
        for _ in 0..1000 {
            let mut url = reqwest::Url::parse("http://local").expect("static URL");
            url.set_path(&base);
            url.query_pairs_mut()
                .append_pair("n", "100")
                .append_pair("last", &last);
            let response = self
                .upstream(
                    authorization,
                    &format!("{}?{}", url.path(), url.query().unwrap()),
                )
                .await?;
            let more = response.headers().contains_key(header::LINK);
            let value: Value = serde_json::from_slice(&body(response, 1024 * 1024).await?)
                .map_err(|_| ApiError::bad("The file list is invalid"))?;
            let page = value[field]
                .as_array()
                .context("File-store listing was invalid")?;
            for item in page {
                names.push(item.as_str().context("Invalid file-store name")?.to_owned());
            }
            if !more {
                return Ok(names);
            }
            let next = names.last().context("Empty file-store page")?;
            if *next <= last {
                return Err(ApiError::bad("File-store pagination did not advance"));
            }
            last.clone_from(next);
        }
        Err(ApiError::bad(
            "The folder has too many pages. Open a specific version instead",
        ))
    }
}
async fn body(mut response: reqwest::Response, limit: usize) -> Result<Vec<u8>> {
    ensure!(
        response.content_length().unwrap_or(0) <= limit as u64,
        "File exceeds the supported size"
    );
    let mut bytes = Vec::new();
    while let Some(chunk) = response.chunk().await? {
        ensure!(
            bytes.len() + chunk.len() <= limit,
            "File exceeds the supported size"
        );
        bytes.extend_from_slice(&chunk);
    }
    Ok(bytes)
}
#[derive(Deserialize, Default)]
struct Page {
    n: Option<usize>,
    last: Option<String>,
    q: Option<String>,
}
fn page(
    mut names: Vec<String>,
    input: &Page,
    path: &str,
    field: &str,
) -> std::result::Result<Response, ApiError> {
    let limit = input.n.unwrap_or(20);
    if limit == 0 || limit > 1000 {
        return Err(ApiError::bad("Choose a page size between 1 and 1000"));
    }
    let filter = input.q.as_deref().unwrap_or("").to_lowercase();
    names.sort();
    names.dedup();
    names.retain(|name| {
        name.as_str() > input.last.as_deref().unwrap_or("") && name.to_lowercase().contains(&filter)
    });
    let more = names.len() > limit;
    names.truncate(limit);
    let mut value = json!({});
    value[field] = json!(names);
    let mut response = Json(value).into_response();
    if more {
        let mut url = reqwest::Url::parse("http://local").expect("static URL");
        url.set_path(path);
        url.query_pairs_mut()
            .append_pair("n", &limit.to_string())
            .append_pair("last", names.last().unwrap())
            .append_pair("q", &filter);
        response.headers_mut().insert(
            header::LINK,
            format!("<{}?{}>; rel=\"next\"", url.path(), url.query().unwrap())
                .parse()
                .unwrap(),
        );
    }
    Ok(response)
}
pub(super) fn router() -> Router<Arc<Service>> {
    Router::new()
        .route("/files/_catalog", get(catalog))
        .route("/files/{*path}", get(file))
        .route("/workflow/config", get(config))
}
async fn config(State(service): State<Arc<Service>>) -> std::result::Result<Json<Value>, ApiError> {
    Ok(Json(
        json!({"registry":service.configuration.registry,"handoff_protocol":"oci-handoff/1","authorization_server":service.configuration.authorization_server,"client_id":service.configuration.client_id,"scopes":service.configuration.scopes}),
    ))
}
async fn catalog(
    State(service): State<Arc<Service>>,
    headers: HeaderMap,
    Query(input): Query<Page>,
) -> std::result::Result<Response, ApiError> {
    let token = authorization(&headers)?;
    let names = service.names(&token, None).await?;

    page(names, &input, "/files/_catalog", "repositories")
}
async fn file(
    State(service): State<Arc<Service>>,
    RoutePath(path): RoutePath<String>,
    headers: HeaderMap,
    Query(input): Query<Page>,
) -> std::result::Result<Response, ApiError> {
    let token = authorization(&headers)?;
    if let Some(repo) = path.strip_suffix("/tags/list") {
        valid_repo(repo)?;
        return page(
            service.names(&token, Some(repo)).await?,
            &input,
            &format!("/files/{repo}/tags/list"),
            "tags",
        );
    }
    if let Some((repo, reference)) = path.rsplit_once("/manifests/") {
        let (fingerprint, bytes) = service.source_manifest(&token, repo, reference).await?;
        return Ok((
            [
                (header::CONTENT_TYPE.as_str(), MANIFEST_TYPE),
                ("docker-content-digest", fingerprint.as_str()),
            ],
            bytes,
        )
            .into_response());
    }
    if let Some((repo, fingerprint)) = path.rsplit_once("/blobs/") {
        valid_repo(repo)?;
        digest_hex(fingerprint)?;
        let bytes = body(
            service
                .upstream(&token, &format!("/v2/{repo}/blobs/{fingerprint}"))
                .await?,
            512 * 1024 * 1024,
        )
        .await?;
        if digest(&bytes) != fingerprint {
            return Err(ApiError::bad(
                "The downloaded file failed its integrity check",
            ));
        }
        return Ok(([(header::CONTENT_TYPE, "application/octet-stream")], bytes).into_response());
    }
    Err(ApiError::bad("Choose a folder, version or file"))
}
pub(super) async fn ready(State(service): State<Arc<Service>>) -> StatusCode {
    match service
        .http
        .get(format!("{}/v2/", service.configuration.registry_upstream))
        .send()
        .await
    {
        Ok(r) if r.status().is_success() || r.status() == StatusCode::UNAUTHORIZED => {
            StatusCode::OK
        }
        _ => StatusCode::SERVICE_UNAVAILABLE,
    }
}
