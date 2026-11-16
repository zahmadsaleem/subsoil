<h1><picture><source media="(prefers-color-scheme: dark)" srcset="assets/brand/subsoil-logo-chalk.svg"><img src="assets/brand/subsoil-logo.svg" alt="Subsoil" width="280"></picture></h1>

Subsoil is an example common data environment (CDE) built on an OCI registry.
Browse project files and their versions, open a version in an independent app,
and publish revisions directly to Zot. Keycloak handles sign-in and issues
repository access tokens.

## Try it

Open the [hosted demo](https://files.lab.madsolame.com/explore), create an
account, and find the sample files in **My workspace**. Start a local copy
with Docker Compose.

```sh
docker compose up --build -d
```

Open [Subsoil Files](http://localhost:5080/explore) and confirm your signup
using the [local inbox](http://localhost:8025).

- [Connect an app](docs/DEVELOPMENT.md) and publish a revision with the starter editor.
- [Architecture](docs/ARCHITECTURE.md) explains how independent apps and CDEs exchange versions.

Licensed under the [Apache License 2.0](LICENSE).
