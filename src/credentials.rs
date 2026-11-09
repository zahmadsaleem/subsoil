use anyhow::Result;
use serde::{Deserialize, Serialize};
use std::{fs, path::Path};
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Access {
    #[serde(rename = "type")]
    pub kind: String,
    pub name: String,
    pub actions: Vec<String>,
}
#[cfg(unix)]
pub fn private_file(path: &Path) -> Result<()> {
    use std::os::unix::fs::PermissionsExt;
    fs::set_permissions(path, fs::Permissions::from_mode(0o600))?;
    Ok(())
}
#[cfg(not(unix))]
pub fn private_file(_path: &Path) -> Result<()> {
    Ok(())
}
