<h1><picture><source media="(prefers-color-scheme: dark)" srcset="assets/brand/subsoil-logo-chalk.svg"><img src="assets/brand/subsoil-logo.svg" alt="Subsoil" width="280"></picture></h1>

Subsoil is an example common data environment (CDE) built on an OCI registry.
Browse project files and their versions, open a version in an independent app,
and publish revisions directly to Zot. Keycloak handles sign-in and issues
repository access tokens.

OCI defines the packaging and delivery specifications. Tools such as ORAS and
other compatible clients and registries can participate independently of Subsoil
and its model profile.

- [Subsoil website](https://subsoil.dev/)
- [Open CDE governance slides](https://subsoil.dev/open-cde-governance/)
- [Project data exchange slides](https://subsoil.dev/project-data-exchange/) and [speaker notes in Markdown](docs/PITCH.md)
- [OCI handbook](https://subsoil.dev/handbook/)
- [Research on CDEs, information flow, and AI](https://subsoil.dev/research/)

## Try it

Open the [hosted CDE](https://cde.subsoil.dev/), create an account, and set up
**My workspace**. Add the sample work package to try it with an app. Start a
local copy with Docker Compose.

```sh
docker compose up --build -d
```

Open the [local CDE](http://localhost:5080/) and sign in as `reader` with
`reader-demo-password` to browse the Riverside work-package releases. To
register an app and try both kinds of handoff, create an account, confirm
your signup in the [local inbox](http://localhost:8025), and open **My workspace**.

- [Connect an app](docs/DEVELOPMENT.md) and publish a revision with the starter editor.
- [Architecture](docs/ARCHITECTURE.md) explains how independent apps and CDEs exchange versions.

Licensed under the [Apache License 2.0](LICENSE).
