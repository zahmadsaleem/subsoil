//! Native OCI 1.1 discovery derived from authorized, immutable manifests.
use super::*;
use crate::auth::Claims;
use std::collections::BTreeSet;

const INDEX_TYPE: &str = "application/vnd.oci.image.index.v1+json";

/// A subject is a weak association: its target need not exist here yet.
pub(super) fn subject(manifest: &Value) -> Result<Option<&str>> {
    let Some(subject) = manifest.get("subject") else {
        return Ok(None);
    };
    let value = subject["digest"]
        .as_str()
        .context("Subject requires a SHA-256 digest")?;
    digest_hex(value)?;
    ensure!(
        subject["size"].as_u64().is_some()
            && subject["mediaType"].as_str().is_some_and(
                |kind| kind.contains('/') && kind.bytes().all(|b| b.is_ascii_graphic())
            ),
        "Subject requires a nonnegative size and media type"
    );
    // These fields become the referrer's discovery descriptor. Reject malformed
    // metadata on new attachments rather than poisoning another client's list.
    ensure!(
        manifest["config"]["mediaType"]
            .as_str()
            .is_some_and(|kind| !kind.is_empty()),
        "Referrer requires a config media type"
    );
    if let Some(kind) = manifest.get("artifactType") {
        ensure!(kind.is_string(), "Artifact type must be a string");
    }
    if let Some(annotations) = manifest.get("annotations") {
        ensure!(
            annotations
                .as_object()
                .is_some_and(|annotations| annotations.values().all(Value::is_string)),
            "Annotations must be a map of strings"
        );
    }
    Ok(Some(value))
}

pub(super) fn list(
    registry: &Registry,
    claims: &Claims,
    repo: &str,
    value: &str,
    query: &HashMap<String, String>,
) -> std::result::Result<Response, ApiError> {
    digest_hex(value)
        .map_err(|error| ApiError(StatusCode::BAD_REQUEST, "DIGEST_INVALID", error.to_string()))?;
    let limit = query
        .get("n")
        .map(|n| n.parse::<usize>())
        .transpose()
        .map_err(ApiError::bad)?
        .unwrap_or(100);
    if !(1..=1000).contains(&limit) {
        return Err(ApiError::bad("Page size must be between 1 and 1000"));
    }
    let after = query.get("last").map(String::as_str).unwrap_or("");
    if !after.is_empty() {
        digest_hex(after)?;
    }
    let filter = query.get("artifactType");
    let limits = registry.auth.limits(claims)?;
    let roots: BTreeSet<_> = registry
        .visible_roots(&limits, repo)?
        .into_iter()
        .map(|(_, digest)| digest)
        .collect();
    let mut budget = access::MAX_POLICY_BYTES;
    let mut descriptors = Vec::new();
    for root in roots {
        if root.as_str() <= after {
            continue;
        }
        let file = registry
            .repo(repo)
            .join("manifests")
            .join(digest_hex(&root)?);
        let size = usize::try_from(file.metadata()?.len()).map_err(ApiError::bad)?;
        if size > 1024 * 1024 || size > budget {
            return Err(ApiError::bad("Referrers discovery content budget exceeded"));
        }
        budget -= size;
        let bytes = fs::read(file)?;
        if digest(&bytes) != root {
            return Err(ApiError::bad("Stored manifest integrity failure"));
        }
        let manifest: Value = serde_json::from_slice(&bytes).map_err(ApiError::bad)?;
        if !matches!(subject(&manifest), Ok(Some(subject)) if subject == value) {
            continue;
        }
        let kind = manifest["artifactType"]
            .as_str()
            .filter(|kind| !kind.is_empty())
            .or_else(|| manifest["config"]["mediaType"].as_str())
            .context("Missing referrer artifact type")?;
        if filter.is_some_and(|filter| filter != kind) {
            continue;
        }
        let mut descriptor = json!({
            "mediaType": MANIFEST_TYPE, "digest": root, "size": size,
            "artifactType": kind
        });
        if let Some(annotations) = manifest.get("annotations") {
            descriptor["annotations"] = annotations.clone();
        }
        descriptors.push(descriptor);
        if descriptors.len() > limit {
            break;
        }
    }
    let more = descriptors.len() > limit;
    descriptors.truncate(limit);
    let mut metadata = vec![
        ("content-type", INDEX_TYPE.into()),
        ("cache-control", "no-store".into()),
    ];
    if filter.is_some() {
        metadata.push(("oci-filters-applied", "artifactType".into()));
    }
    if more {
        let mut url = reqwest::Url::parse("http://local").expect("static URL");
        url.set_path(&format!("/v2/{repo}/referrers/{value}"));
        url.query_pairs_mut()
            .append_pair("n", &limit.to_string())
            .append_pair(
                "last",
                descriptors.last().unwrap()["digest"].as_str().unwrap(),
            );
        if let Some(filter) = filter {
            url.query_pairs_mut().append_pair("artifactType", filter);
        }
        metadata.push((
            "link",
            format!("<{}?{}>; rel=\"next\"", url.path(), url.query().unwrap()),
        ));
    }
    Ok(response(
        StatusCode::OK,
        serde_json::to_vec(&json!({
            "schemaVersion": 2, "mediaType": INDEX_TYPE, "manifests": descriptors
        }))
        .map_err(ApiError::bad)?,
        &metadata,
    ))
}
