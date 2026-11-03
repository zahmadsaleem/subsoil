use anyhow::Result;
use reqwest::{Client, Method, Response};
use serde_json::{json, Value};
use std::sync::Arc;
use subsoil::{
    auth::ShareBinding,
    policy::{Grant, GrantSet},
    registry::{self, Registry},
    release::digest,
};

const REPO: &str = "project/referrers/model";
const REPORT: &str = "application/vnd.example.ids-report.v1+json";

struct Fixture {
    temp: tempfile::TempDir,
    registry: Arc<Registry>,
    http: Client,
    host: String,
    writer: String,
    server: tokio::task::JoinHandle<Result<()>>,
}

impl Drop for Fixture {
    fn drop(&mut self) {
        self.server.abort();
    }
}

impl Fixture {
    async fn new() -> Result<Self> {
        let temp = tempfile::tempdir()?;
        let registry = Registry::open(temp.path().join("registry"))?;
        registry.auth.add_user(
            "writer",
            "referrers-test-password",
            &[format!("{REPO}:pull,push"), "other/model:pull,push".into()],
        )?;
        let writer = registry
            .auth
            .issue("writer", &["subsoil:catalog:grants".into()], 900)?
            .0;
        registry.set_cors_origins(&["http://localhost:3000".into()])?;
        let listener = registry::bind("127.0.0.1:0".parse()?).await?;
        let host = listener.local_addr()?.to_string();
        registry.set_public_url(&format!("http://{host}"))?;
        let server = tokio::spawn(registry::serve(listener, registry.clone()));
        Ok(Self {
            temp,
            registry,
            http: Client::new(),
            host,
            writer,
            server,
        })
    }

    async fn put(&self, repo: &str, reference: &str, manifest: &Value) -> Result<Response> {
        let upload = self
            .http
            .post(format!("http://{}/v2/{repo}/blobs/uploads/", self.host))
            .bearer_auth(&self.writer)
            .query(&[("digest", digest(b"{}"))])
            .body("{}")
            .send()
            .await?;
        assert_eq!(upload.status(), 201);
        Ok(self
            .http
            .put(format!(
                "http://{}/v2/{repo}/manifests/{reference}",
                self.host
            ))
            .bearer_auth(&self.writer)
            .header("Origin", "http://localhost:3000")
            .json(manifest)
            .send()
            .await?)
    }

    async fn list(&self, path: &str, token: &str) -> Result<Response> {
        Ok(self
            .http
            .get(format!("http://{}{path}", self.host))
            .bearer_auth(token)
            .header("Origin", "http://localhost:3000")
            .send()
            .await?)
    }

    async fn reader(&self, tags: &[&str]) -> Result<String> {
        self.registry
            .auth
            .add_user("reader", "referrers-test-password", &[])?;
        self.registry.auth.add_rule(
            "reader",
            &Grant {
                repositories: vec![REPO.into()],
                tags: Some(tags.iter().map(|tag| (*tag).into()).collect()),
                digests: None,
                actions: vec!["pull".into()],
            },
        )?;
        Ok(self
            .registry
            .auth
            .issue("reader", &[format!("repository:{REPO}:pull")], 900)?
            .0)
    }
}

fn manifest(subject: Option<&Value>, kind: Option<&str>, label: &str) -> Value {
    let mut value = json!({
        "schemaVersion": 2, "mediaType": registry::MANIFEST_TYPE,
        "config": {"mediaType": "application/vnd.example.config.v1+json", "digest": digest(b"{}"), "size": 2},
        "layers": [], "annotations": {"org.opencontainers.image.title": label}
    });
    if let Some(subject) = subject {
        value["subject"] = subject.clone();
    }
    if let Some(kind) = kind {
        value["artifactType"] = kind.into();
    }
    value
}

fn descriptor(manifest: &Value) -> Value {
    let bytes = serde_json::to_vec(manifest).unwrap();
    json!({"mediaType": registry::MANIFEST_TYPE, "digest": digest(&bytes), "size": bytes.len()})
}

#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn referrers_are_standard_filtered_paged_and_persistent() -> Result<()> {
    let mut f = Fixture::new().await?;
    let model = manifest(None, Some("application/x-step"), "Model");
    let subject = descriptor(&model);
    let value = subject["digest"].as_str().unwrap();
    let path = format!("/v2/{REPO}/referrers/{value}");
    let report = manifest(Some(&subject), Some(REPORT), "IDS report");
    let report_digest = descriptor(&report)["digest"].as_str().unwrap().to_owned();
    // Attachment before its subject is legal, and digest-only publication is discoverable.
    for reference in [&report_digest, &report_digest, "report-1", "report-1"] {
        let response = f.put(REPO, reference, &report).await?;
        assert_eq!(response.status(), 201);
        assert_eq!(response.headers()["oci-subject"], value);
        assert!(response.headers()["access-control-expose-headers"]
            .to_str()?
            .contains("oci-subject"));
    }
    let before = f.list(&path, &f.writer).await?;
    assert_eq!(before.status(), 200);
    let before: Value = before.json().await?;
    assert_eq!(before["manifests"].as_array().unwrap().len(), 1);
    let response = f.put(REPO, "model-1", &model).await?;
    assert_eq!(response.status(), 201);
    assert!(!response.headers().contains_key("oci-subject"));
    let fallback = manifest(Some(&subject), None, "Fallback config type");
    assert_eq!(f.put(REPO, "fallback", &fallback).await?.status(), 201);
    let empty_type = manifest(Some(&subject), Some(""), "Empty artifact type");
    assert_eq!(f.put(REPO, "empty-type", &empty_type).await?.status(), 201);
    // The same subject in another repository never appears in this repository's list.
    assert_eq!(
        f.put("other/model", "foreign", &report).await?.status(),
        201
    );
    let response = f.list(&path, &f.writer).await?;
    assert_eq!(
        response.headers()["content-type"],
        "application/vnd.oci.image.index.v1+json"
    );
    let all: Value = response.json().await?;
    assert_eq!(all["schemaVersion"], 2);
    assert_eq!(all["manifests"].as_array().unwrap().len(), 3);
    let item = all["manifests"]
        .as_array()
        .unwrap()
        .iter()
        .find(|d| d["digest"] == report_digest)
        .unwrap();
    assert_eq!(item["mediaType"], registry::MANIFEST_TYPE);
    assert_eq!(item["size"], serde_json::to_vec(&report)?.len());
    assert_eq!(item["artifactType"], REPORT);
    assert_eq!(item["annotations"], report["annotations"]);
    let response = f
        .list(
            &format!("{path}?artifactType={}&n=1", REPORT.replace('+', "%2B")),
            &f.writer,
        )
        .await?;
    assert_eq!(response.headers()["oci-filters-applied"], "artifactType");
    assert!(!response.headers().contains_key("link"));
    assert!(response.headers()["access-control-expose-headers"]
        .to_str()?
        .contains("oci-filters-applied"));
    assert_eq!(response.json::<Value>().await?["manifests"], json!([item]));
    let response = f
        .list(
            &format!("{path}?artifactType=application/vnd.example.config.v1%2Bjson&n=1"),
            &f.writer,
        )
        .await?;
    let link = response.headers()["link"].to_str()?.to_owned();
    assert!(link.contains("artifactType=application%2Fvnd.example.config.v1%2Bjson"));
    let first: Value = response.json().await?;
    let next = link.split('>').next().unwrap().trim_start_matches('<');
    let second = f.list(next, &f.writer).await?;
    assert!(!second.headers().contains_key("link"));
    let second: Value = second.json().await?;
    assert_ne!(
        first["manifests"][0]["digest"],
        second["manifests"][0]["digest"]
    );
    assert_eq!(
        first["manifests"][0]["artifactType"],
        "application/vnd.example.config.v1+json"
    );
    for suffix in ["?artifactType=application/unknown", ""] {
        let query = if suffix.is_empty() {
            format!("/v2/{REPO}/referrers/{}", digest(b"missing"))
        } else {
            format!("{path}{suffix}")
        };
        let response = f.list(&query, &f.writer).await?;
        assert_eq!(response.status(), 200);
        assert_eq!(response.json::<Value>().await?["manifests"], json!([]));
    }
    // Stored subject manifests remain discoverable after restart.
    f.server.abort();
    let reopened = Registry::open(f.temp.path().join("registry"))?;
    let listener = registry::bind("127.0.0.1:0".parse()?).await?;
    f.host = listener.local_addr()?.to_string();
    f.server = tokio::spawn(registry::serve(listener, reopened));
    assert_eq!(f.list(&path, &f.writer).await?.json::<Value>().await?, all);
    Ok(())
}

#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn referrers_enforce_each_artifacts_permissions_and_public_policy() -> Result<()> {
    let f = Fixture::new().await?;
    let model = manifest(None, Some("application/x-step"), "Model");
    let subject = descriptor(&model);
    let value = subject["digest"].as_str().unwrap();
    let path = format!("/v2/{REPO}/referrers/{value}");
    assert_eq!(f.put(REPO, "model-1", &model).await?.status(), 201);
    let public = manifest(Some(&subject), Some(REPORT), "Public report");
    let hidden = manifest(Some(&subject), Some(REPORT), "Private report");
    assert_eq!(f.put(REPO, "report-1", &public).await?.status(), 201);
    assert_eq!(f.put(REPO, "private-report", &hidden).await?.status(), 201);
    let reader = f.reader(&["model-1", "report-1"]).await?;
    let visible: Value = f.list(&path, &reader).await?.json().await?;
    assert_eq!(visible["manifests"].as_array().unwrap().len(), 1);
    assert_eq!(
        visible["manifests"][0]["digest"],
        descriptor(&public)["digest"]
    );
    assert!(!visible.to_string().contains("Private report"));
    // Sharing the source alone does not implicitly share its reports.
    let share = f
        .registry
        .create_share(
            "writer",
            &GrantSet {
                version: 1,
                registry: f.host.clone(),
                grants: vec![Grant {
                    repositories: vec![REPO.into()],
                    tags: Some(vec!["model-1".into()]),
                    digests: None,
                    actions: vec!["pull".into()],
                }],
            },
            900,
            ShareBinding::Snapshot,
        )
        .await?;
    let (_, claims, _) = f.registry.auth.redeem_share(&share.secret)?;
    let snapshot = f
        .registry
        .auth
        .exchange(&claims, &[format!("repository:{REPO}:pull")])?
        .0;
    assert_eq!(
        f.list(&path, &snapshot).await?.json::<Value>().await?["manifests"],
        json!([])
    );
    f.registry.auth.set_public_grants(&GrantSet {
        version: 1,
        registry: f.host.clone(),
        grants: vec![Grant {
            repositories: vec![REPO.into()],
            tags: Some(vec!["model-1".into()]),
            digests: None,
            actions: vec!["pull".into()],
        }],
    })?;
    let anonymous: Value = f
        .http
        .get(format!(
            "http://{}/token?scope=repository:{REPO}:pull",
            f.host
        ))
        .send()
        .await?
        .json()
        .await?;
    let token = anonymous["token"].as_str().unwrap();
    assert_eq!(
        f.list(&path, token).await?.json::<Value>().await?["manifests"],
        json!([])
    );
    let restricted = f
        .registry
        .auth
        .issue("reader", &["repository:other/model:pull".into()], 900)?
        .0;
    assert_eq!(f.list(&path, &restricted).await?.status(), 403);
    assert_eq!(
        f.http
            .get(format!("http://{}{path}", f.host))
            .send()
            .await?
            .status(),
        401
    );
    Ok(())
}

#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn referrers_reject_invalid_subjects_requests_and_tag_replacement() -> Result<()> {
    let f = Fixture::new().await?;
    let model = manifest(None, None, "Model");
    let subject = descriptor(&model);
    for invalid in [
        Value::Null,
        json!({"digest": "../bad", "mediaType": registry::MANIFEST_TYPE, "size": 1}),
        json!({"digest": subject["digest"], "mediaType": registry::MANIFEST_TYPE, "size": -1}),
        json!({"digest": subject["digest"], "size": 1}),
    ] {
        let invalid = manifest(Some(&invalid), Some(REPORT), "Invalid report");
        let response = f.put(REPO, "invalid", &invalid).await?;
        assert_eq!(response.status(), 400);
        assert_eq!(
            response.json::<Value>().await?["errors"][0]["code"],
            "MANIFEST_INVALID"
        );
    }
    let report = manifest(Some(&subject), Some(REPORT), "Report");
    for (field, value) in [
        ("annotations", json!({"bad": 1})),
        ("artifactType", json!(7)),
        ("config", json!({"digest": digest(b"{}"), "size": 2})),
    ] {
        let mut invalid = report.clone();
        invalid[field] = value;
        let response = f.put(REPO, "invalid-metadata", &invalid).await?;
        assert_eq!(response.status(), 400);
        assert_eq!(
            response.json::<Value>().await?["errors"][0]["code"],
            "MANIFEST_INVALID"
        );
    }
    assert_eq!(f.put(REPO, "report-1", &report).await?.status(), 201);
    let changed = manifest(Some(&subject), Some(REPORT), "Changed report");
    assert_eq!(f.put(REPO, "report-1", &changed).await?.status(), 409);
    for suffix in [
        "bad".to_owned(),
        format!("{}?n=0", subject["digest"].as_str().unwrap()),
        format!("{}?n=1001", subject["digest"].as_str().unwrap()),
        format!("{}?last=bad", subject["digest"].as_str().unwrap()),
    ] {
        assert_eq!(
            f.list(&format!("/v2/{REPO}/referrers/{suffix}"), &f.writer)
                .await?
                .status(),
            400
        );
    }
    assert_eq!(
        f.http
            .request(
                Method::POST,
                format!(
                    "http://{}/v2/{REPO}/referrers/{}",
                    f.host,
                    subject["digest"].as_str().unwrap()
                )
            )
            .bearer_auth(&f.writer)
            .send()
            .await?
            .status(),
        405
    );
    Ok(())
}
