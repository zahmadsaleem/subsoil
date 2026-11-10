use anyhow::{ensure, Result};
use reqwest::Url;
use serde::{Deserialize, Serialize};
#[derive(Clone, Copy, Debug, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Intent {
    View,
    Edit,
}
#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Application {
    pub id: String,
    pub name: String,
    pub launch_url: String,
}

pub(crate) fn web_url(value: &str) -> Result<Url> {
    let url = Url::parse(value)?;
    let host = url.host_str().unwrap_or("").trim_matches(['[', ']']);
    let loopback = host == "localhost"
        || host
            .parse::<std::net::IpAddr>()
            .is_ok_and(|ip| ip.is_loopback());
    ensure!(
        !value.chars().any(|c| c.is_whitespace() || c.is_control())
            && !value.contains('\\')
            && (url.scheme() == "https" || (url.scheme() == "http" && loopback))
            && url.host_str().is_some() && !host.contains('*')
            && url.username().is_empty() && url.password().is_none()
            && url.query().is_none() && url.fragment().is_none(),
        "Application URLs must be exact HTTPS or loopback HTTP URLs without credentials, query or fragment"
    );
    Ok(url)
}
