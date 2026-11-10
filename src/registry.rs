//! OCI reference validation and local client credential-file writes. Registry storage is provided by Zot.
use anyhow::{ensure, Context, Result};
use std::{fs, io::Write, path::Path};

pub const MANIFEST_TYPE: &str = "application/vnd.oci.image.manifest.v1+json";

pub fn atomic_write(path: &Path, bytes: &[u8]) -> Result<()> {
    let parent = path.parent().context("Missing storage parent")?;
    fs::create_dir_all(parent)?;
    let mut file = tempfile::NamedTempFile::new_in(parent)?;
    file.write_all(bytes)?;
    file.as_file().sync_all()?;
    file.persist(path).map_err(|error| error.error)?;
    Ok(())
}
pub(crate) fn valid_repo(repo: &str) -> Result<()> {
    ensure!(
        !repo.is_empty()
            && repo.len() <= 255
            && repo.split('/').all(|part| !part.is_empty()
                && (part.as_bytes()[0].is_ascii_lowercase()
                    || part.as_bytes()[0].is_ascii_digit())),
        "Invalid repository name"
    );
    ensure!(
        repo.split('/').all(|part| part
            .bytes()
            .all(|c| c.is_ascii_lowercase() || c.is_ascii_digit() || b"._-".contains(&c))),
        "Invalid repository name"
    );
    Ok(())
}

pub(crate) fn valid_tag(tag: &str) -> Result<()> {
    ensure!(
        !tag.is_empty()
            && tag.len() <= 128
            && tag
                .bytes()
                .all(|c| c.is_ascii_alphanumeric() || b"_.-".contains(&c))
            && !tag.starts_with(['.', '-']),
        "Invalid artifact tag"
    );
    Ok(())
}

pub async fn bind(address: std::net::SocketAddr) -> Result<tokio::net::TcpListener> {
    Ok(tokio::net::TcpListener::bind(address).await?)
}
