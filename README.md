<h1><picture><source media="(prefers-color-scheme: dark)" srcset="assets/brand/subsoil-logo-chalk.svg"><img src="assets/brand/subsoil-logo.svg" alt="Subsoil" width="280"></picture></h1>

Subsoil demonstrates information exchange between independent applications:
find a file version, **Open with…** an application, edit it, and **Push to…** a
chosen file store. Its proposed OCI handoff contract connects the experience
to standard OAuth authorization and OCI transfer.

| Component | Responsibility |
| --- | --- |
| Zot | Manifests, files, versions, uploads, downloads, and storage management |
| Subsoil | File/version browsing, application selection, and handoffs |
| Keycloak | Login, account permissions, and registry access tokens |
| IFC Lite | Opening and editing IFC models, selecting a destination, and publishing |

IFC Lite transfers content directly through Zot's public address. Subsoil reads
Zot for its file browser and never stores manifests or file content. A small
Envoy gateway validates token issuer and audience; Zot enforces repository
permissions. The demo's Keycloak mapper expresses authorized OAuth scopes as
Distribution token claims.

## Run the demonstration

Clone this repository and install Docker with Compose. The viewer build fetches
a pinned IFC Lite production revision and applies the included OCI handoff
patch. No separate IFC Lite checkout is required. The first build needs network
access to GitHub and the package registries.

```sh
docker compose --profile viewer up --build -d
```

Open [Subsoil Files](http://localhost:5080/explore). Sign in as `writer` with
`writer-demo-password`, choose `project/model` and `issued`, then choose
**Open with…** or **Edit with… → IFC Lite**. Authorize the application on
Keycloak. IFC Lite receives the selected version directly from Zot.

Edit the model, open **File → Cloud Sources → Push to…**, choose a folder and a
fresh version name, and publish. **Open saved version** opens the exact saved
content in another viewer tab. Refresh Subsoil's versions to see the result.
The `reader` account, password `reader-demo-password`, can open `project/model`
but cannot publish. `writer` can also publish in `reviews/model`.

The Compose deployment uses local HTTP and known demonstration credentials.
The [deployment guide](docs/DEPLOYMENT.md) explains its services, persistence,
ports, and reset behavior. An existing local IFC Lite development server can
be used by starting Compose without `--profile viewer`.
Set its application launch URL in `demo/subsoil.json` to that server's
`/open-artifact` address and restart Subsoil. The container viewer uses port 3001.

## Verify the exchange

```sh
docker compose --profile test run --rm smoke
```

This exercises real Keycloak PKCE login, permission downscoping, Zot transfers,
reader write denial, immutable handoff references, publication and reopening,
Subsoil's updated version listing, and audience rejection at the gateway.

The [development guide](docs/DEVELOPMENT.md) defines the proposed handoff and
application integration. [Authorization](docs/AUTHORIZATION.md) explains how
permissions cross the components.

The command-line tools can inspect, receive, copy, and publish OCI artifacts.
IFC conversion to Parquet and GLB remains an optional tool capability; the
open/edit/publish demonstration exchanges original IFC STEP files directly.
Conversion is experimental and is not buildingSMART approved.

Licensed under the [Apache License 2.0](LICENSE).
