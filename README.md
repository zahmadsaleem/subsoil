<h1><picture><source media="(prefers-color-scheme: dark)" srcset="assets/brand/subsoil-logo-chalk.svg"><img src="assets/brand/subsoil-logo.svg" alt="Subsoil" width="280"></picture></h1>

Subsoil is an example common data environment (CDE) built on an OCI registry.
Browse project files and their versions, open a version in an independent app,
and publish revisions directly to Zot. Keycloak handles sign-in and issues
repository access tokens.

OCI defines the packaging and delivery specifications. Tools such as ORAS and
other compatible clients and registries can participate independently of Subsoil
and its model profile.

[Read the project pitch](https://madsolame.com/subsoil/).
[Read the pitch and speaker notes in Markdown](docs/PITCH.md).
[Open the practical handbook](docs/handbook.html).
[Read the research on CDEs, information flow, and AI](docs/CDE_AI_RESEARCH.md).

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
