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

Open [Subsoil Files](http://localhost:5080/explore). Sign in as `writer` with
`writer-demo-password`, open `project/model`, and choose the `issued` version.
Download `model.ifc` to get the original file. The `reader` account
(`reader-demo-password`) can browse and download from this folder.

Publish a revision with ORAS or another OCI client against
`localhost:5001/project/model`. Refresh Subsoil to see the new version.
The [deployment guide](docs/DEPLOYMENT.md#publish-a-version) walks through the
exchange and shows how to run the HTTPS stack on a single server.

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
