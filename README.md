<h1><picture><source media="(prefers-color-scheme: dark)" srcset="assets/brand/subsoil-logo-chalk.svg"><img src="assets/brand/subsoil-logo.svg" alt="Subsoil" width="280"></picture></h1>

Subsoil is an example common data environment (CDE) built on an OCI registry.
Use it to browse folders, inspect versions, download files, and open a selected
version with a registered application. External apps publish new versions
directly to the registry.

| Component | Responsibility |
| --- | --- |
| Subsoil | File and version browsing, downloads, and application handoffs |
| Zot | OCI manifests, file storage, and repository permissions |
| Keycloak | Sign-in and repository access tokens |
| Envoy | Token validation at the public registry address |

## Run the demonstration

Clone this repository and start the local Docker Compose stack:

```sh
docker compose up --build -d
```

Open [Subsoil Files](http://localhost:5080/explore), select **Sign in**, and
create an account. Open the verification message in
[the local inbox](http://localhost:8025). Subsoil seeds example versions
in **My workspace** the first time you sign in.

Run the [starter editor](docs/DEVELOPMENT.md#try-the-starter-editor) on
`http://localhost:5173` and register it under **Workspace apps**. Open
`notes-v1` with the editor, change a line, and publish `notes-v2`. Refresh
Subsoil to see the new version. The
[deployment guide](docs/DEPLOYMENT.md#publish-a-version) has an ORAS example
and explains how to run the HTTPS stack.

```sh
docker compose --profile test run --rm smoke
```

The smoke test checks sign-in, reader permissions, direct registry transfers,
and whether Subsoil can browse a version published by another client. The Go
service uses only the standard library and serves the bundled browser UI.

The [development guide](docs/DEVELOPMENT.md) describes registering external
applications and the proposed OCI handoff format. [Authorization](docs/AUTHORIZATION.md)
explains how permissions cross services.

Licensed under the [Apache License 2.0](LICENSE).
