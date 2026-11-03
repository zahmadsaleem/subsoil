//! Run with SUBSOIL_TEST_ORAS and SUBSOIL_TEST_ZOT set to executable paths.
use anyhow::{ensure, Context, Result};
use serde_json::{json, Value};
use std::{
    fs,
    path::{Path, PathBuf},
    process::{Child, Command, Stdio},
    time::Duration,
};
use subsoil::{
    registry::{self, Registry},
    release::digest,
};

const REPORT: &str = "application/vnd.example.ids-report.v1+json";
const PASSWORD: &str = "oras-referrers-test-password";
const CREATED: &str = "org.opencontainers.image.created=2026-01-01T00:00:00Z";

struct Server {
    host: String,
    subsoil: Option<tokio::task::JoinHandle<Result<()>>>,
    zot: Option<Child>,
}

impl Drop for Server {
    fn drop(&mut self) {
        if let Some(task) = &self.subsoil {
            task.abort();
        }
        if let Some(child) = &mut self.zot {
            let _ = child.kill();
            let _ = child.wait();
        }
    }
}

async fn subsoil(root: &Path) -> Result<Server> {
    let registry = Registry::open(root)?;
    registry
        .auth
        .add_user("writer", PASSWORD, &["project/model:pull,push".into()])?;
    let listener = registry::bind("127.0.0.1:0".parse()?).await?;
    Ok(Server {
        host: listener.local_addr()?.to_string(),
        subsoil: Some(tokio::spawn(registry::serve(listener, registry))),
        zot: None,
    })
}

async fn zot(binary: &Path, root: &Path) -> Result<Server> {
    fs::create_dir_all(root)?;
    let listener = std::net::TcpListener::bind("127.0.0.1:0")?;
    let port = listener.local_addr()?.port();
    drop(listener);
    let config = root.join("config.json");
    fs::write(
        &config,
        serde_json::to_vec(&json!({
            "distSpecVersion": "1.1.1",
            "storage": {"rootDirectory": root.join("data")},
            "http": {"address": "127.0.0.1", "port": port.to_string()},
            "log": {"level": "error"}
        }))?,
    )?;
    let log = fs::File::create(root.join("zot.log"))?;
    let mut server = Server {
        host: format!("127.0.0.1:{port}"),
        subsoil: None,
        zot: Some(
            Command::new(binary)
                .arg("serve")
                .arg(&config)
                .stdout(Stdio::from(log.try_clone()?))
                .stderr(Stdio::from(log))
                .spawn()?,
        ),
    };
    let http = reqwest::Client::new();
    for _ in 0..100 {
        if http
            .get(format!("http://{}/v2/", server.host))
            .send()
            .await
            .is_ok_and(|response| response.status().is_success())
        {
            return Ok(server);
        }
        if let Some(status) = server.zot.as_mut().unwrap().try_wait()? {
            anyhow::bail!(
                "zot exited with {status}: {}",
                fs::read_to_string(root.join("zot.log"))?
            );
        }
        tokio::time::sleep(Duration::from_millis(100)).await;
    }
    anyhow::bail!(
        "zot did not become ready: {}",
        fs::read_to_string(root.join("zot.log"))?
    )
}

#[derive(Clone)]
struct Oras {
    binary: PathBuf,
    root: PathBuf,
    auth: PathBuf,
}

impl Oras {
    async fn run(&self, args: &[&str]) -> Result<Vec<u8>> {
        let this = self.clone();
        let args: Vec<_> = args.iter().map(|s| (*s).to_owned()).collect();
        tokio::task::spawn_blocking(move || -> Result<_> {
            let mut command = Command::new(&this.binary);
            command.args(&args);
            if args[0] == "cp" {
                command
                    .arg("--from-registry-config")
                    .arg(&this.auth)
                    .arg("--to-registry-config")
                    .arg(&this.auth);
            } else {
                command.arg("--plain-http");
                command.arg("--registry-config").arg(&this.auth);
            }
            let result = command.current_dir(&this.root).output()?;
            ensure!(
                result.status.success(),
                "ORAS {} failed: {}\n{}",
                args.first().unwrap(),
                String::from_utf8_lossy(&result.stderr),
                String::from_utf8_lossy(&result.stdout)
            );
            Ok(result.stdout)
        })
        .await?
    }

    async fn login(&self, host: &str) -> Result<()> {
        use std::io::Write;
        let this = self.clone();
        let host = host.to_owned();
        tokio::task::spawn_blocking(move || -> Result<_> {
            let mut child = Command::new(&this.binary)
                .args([
                    "login",
                    &host,
                    "--username",
                    "writer",
                    "--password-stdin",
                    "--plain-http",
                ])
                .arg("--registry-config")
                .arg(&this.auth)
                .stdin(Stdio::piped())
                .stdout(Stdio::piped())
                .stderr(Stdio::piped())
                .spawn()?;
            child.stdin.take().unwrap().write_all(PASSWORD.as_bytes())?;
            let result = child.wait_with_output()?;
            ensure!(
                result.status.success(),
                "ORAS login: {}",
                String::from_utf8_lossy(&result.stderr)
            );
            Ok(())
        })
        .await?
    }

    async fn workflow(&self, server: &Server, output: &str) -> Result<(String, String)> {
        let repo = format!("{}/project/model", server.host);
        let source = format!("{repo}:model-1");
        self.run(&[
            "push",
            &source,
            "model.ifc:application/x-step",
            "--artifact-type",
            "application/vnd.example.ifc-release.v1",
            "--annotation",
            CREATED,
        ])
        .await?;
        let original = self.run(&["manifest", "fetch", &source]).await?;
        let model_digest = digest(&original);
        let pinned = format!("{repo}@{model_digest}");
        let report = serde_json::to_vec(&json!({
            "example": true, "model": model_digest, "findings": []
        }))?;
        fs::write(self.root.join("report.json"), &report)?;
        // Auto detection must work; forcing v1.1 alone would miss a broken OCI-Subject header.
        let attached = self
            .run(&[
                "attach",
                &pinned,
                "report.json:application/json",
                "--artifact-type",
                REPORT,
                "--annotation",
                CREATED,
                "--format",
                "json",
            ])
            .await?;
        let attached: Value = serde_json::from_slice(&attached)?;
        let report_digest = attached["digest"]
            .as_str()
            .context("ORAS attachment digest")?
            .to_owned();
        for mode in [false, true] {
            let mut args = vec![
                "discover",
                &pinned,
                "--format",
                "json",
                "--depth",
                "1",
                "--artifact-type",
                REPORT,
            ];
            if mode {
                args.extend(["--distribution-spec", "v1.1-referrers-api"]);
            }
            let discovered: Value = serde_json::from_slice(&self.run(&args).await?)?;
            let refs = discovered["referrers"]
                .as_array()
                .context("ORAS referrers list")?;
            assert_eq!(refs.len(), 1);
            assert_eq!(refs[0]["digest"], report_digest);
            assert_eq!(refs[0]["artifactType"], REPORT);
        }
        let attached_manifest = self
            .run(&["manifest", "fetch", &format!("{repo}@{report_digest}")])
            .await?;
        let attached_manifest: Value = serde_json::from_slice(&attached_manifest)?;
        assert_eq!(attached_manifest["subject"]["digest"], model_digest);
        assert_eq!(attached_manifest["subject"]["size"], original.len());
        self.run(&[
            "pull",
            &format!("{repo}@{report_digest}"),
            "--output",
            output,
        ])
        .await?;
        assert_eq!(
            fs::read(self.root.join(output).join("report.json"))?,
            report
        );
        assert_eq!(self.run(&["manifest", "fetch", &pinned]).await?, original);
        // Native attachment does not create or mutate a referrers fallback tag.
        assert_eq!(
            String::from_utf8(self.run(&["repo", "tags", &repo]).await?)?.trim(),
            "model-1"
        );
        Ok((model_digest, report_digest))
    }
}

#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
#[ignore = "requires SUBSOIL_TEST_ORAS and SUBSOIL_TEST_ZOT executable paths"]
async fn oras_attachment_discovery_and_copy_work_on_subsoil_and_zot() -> Result<()> {
    let root = tempfile::tempdir()?;
    let oras = Oras {
        binary: std::env::var_os("SUBSOIL_TEST_ORAS")
            .context("Set SUBSOIL_TEST_ORAS")?
            .into(),
        root: root.path().to_owned(),
        auth: root.path().join("oras-auth.json"),
    };
    let zot_binary =
        PathBuf::from(std::env::var_os("SUBSOIL_TEST_ZOT").context("Set SUBSOIL_TEST_ZOT")?);
    let subsoil = subsoil(&root.path().join("subsoil")).await?;
    let zot = zot(&zot_binary, &root.path().join("zot")).await?;
    fs::write(root.path().join("model.ifc"), "ISO-10303-21;\nHEADER;\nFILE_SCHEMA(('IFC4'));\nENDSEC;\nDATA;\nENDSEC;\nEND-ISO-10303-21;\n")?;
    oras.login(&subsoil.host).await?;
    let source = oras.workflow(&subsoil, "subsoil-report").await?;
    let independent = oras.workflow(&zot, "zot-report").await?;
    assert_eq!(
        source, independent,
        "Both registries preserve identical manifests"
    );
    let from = format!("{}/project/model@{}", subsoil.host, source.0);
    let to = format!("{}/copied/model:model-1", zot.host);
    oras.run(&[
        "cp",
        "--recursive",
        "--from-plain-http",
        "--to-plain-http",
        &from,
        &to,
    ])
    .await?;
    let copied: Value = serde_json::from_slice(
        &oras
            .run(&[
                "discover",
                &to,
                "--distribution-spec",
                "v1.1-referrers-api",
                "--format",
                "json",
                "--depth",
                "1",
            ])
            .await?,
    )?;
    assert_eq!(copied["referrers"][0]["digest"], source.1);
    let copied_report = format!("{}/copied/model@{}", zot.host, source.1);
    oras.run(&["pull", &copied_report, "--output", "copied-report"])
        .await?;
    assert_eq!(
        fs::read(root.path().join("copied-report/report.json"))?,
        fs::read(root.path().join("subsoil-report/report.json"))?
    );
    eprintln!("Verified ORAS attachment, discovery, byte-identical retrieval on Subsoil and zot, and recursive copy Subsoil -> zot");
    Ok(())
}
