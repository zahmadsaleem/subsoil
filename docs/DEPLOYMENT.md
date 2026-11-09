# Run the exchange demonstration

The root `compose.yaml` runs Subsoil over an independently implemented registry
and identity provider. Subsoil is stateless: file content lives in Zot and
accounts and signing keys live in Keycloak.

## Start the services

Install Docker with Compose. Place the IFC Lite checkout at
`../../lt-plus/ifc-lite`, or set `IFC_LITE_CHECKOUT` to its absolute path. The
checkout must contain the OCI handoff application changes and
`apps/viewer/Dockerfile.registry`.

```sh
docker compose --profile viewer up --build -d
```

| Address | Service |
| --- | --- |
| `http://localhost:5080/explore` | Subsoil Files |
| `http://localhost:3001` | IFC Lite, when the viewer profile is enabled |
| `http://localhost:5001/v2/` | Zot's public OCI address through Envoy |
| `http://localhost:8080` | Keycloak |

Port 3001 lets the container viewer run alongside a development server on port 3000.
Port 5080 avoids the macOS service commonly occupying port 5000. Changing a
public port also requires changing its registered callbacks, browser origins,
and configured registry/issuer URLs. Internal Docker addresses remain separate
from the public URLs that appear in tokens and handoffs.

The `zot-init` service waits for Keycloak, writes its public certificate and Zot
configuration to `zot-config`, and exits. Zot then starts. The `seed` service
uses a dedicated Keycloak service account to upload `demo/model.ifc` through
Zot's public gateway. Rerunning the seed publishes the same `issued` content.

For an existing local IFC Lite development server, omit `--profile viewer`.
Set the application's `launch_url` in `demo/subsoil.json` to
`http://localhost:3000/open-artifact` and restart Subsoil.
Use its connection configuration from the [development guide](DEVELOPMENT.md)
and register the exact localhost callbacks already supplied by the demo realm.

## Use the workflow

Sign in to Subsoil as `writer` / `writer-demo-password`. Open `project/model`,
choose `issued`, and select **Open with…** or **Edit with… → IFC Lite**.
IFC Lite independently authorizes its source read with Keycloak and retrieves
the selected model from Zot by digest.

Edit a property, then choose **File → Cloud Sources → Push to…**. Select the
file store and folder, enter a fresh version name, and approve destination
access. **Open saved version** opens a new viewer tab for the exact saved
manifest. Refresh the versions in Subsoil to see the publication.

The `reader` / `reader-demo-password` account can receive `project/model` and
cannot publish. Keycloak's demonstration administrator is `admin` /
`admin-demo-password`; its realm import is `demo/keycloak/realm.json`.

## Verify and inspect

```sh
docker compose --profile test run --rm smoke
docker compose ps
docker compose logs subsoil keycloak registry zot seed
```

The smoke service runs real PKCE exchanges for both accounts, validates file
bytes and handoff contents, checks reader denial, publishes an edited IFC,
reopens it by digest, confirms Subsoil lists the new version, and checks that
Envoy rejects a genuine Keycloak token without the registry audience.

The container check uses internal network addresses for connections while
preserving the public issuer, callback and registry identifiers. These network
adaptations are test scaffolding; applications use public addresses directly.

## Persistence and reset

Zot content is in `zot-data`. Keycloak's development database, identities and
signing keys are in `keycloak-data`. Its imported realm is loaded only when it
does not already exist. Editing the import file does not overwrite a running
realm. Use the Keycloak administration interface for existing accounts.

```sh
docker compose down
```

This stops the demo while retaining its named volumes. To deliberately discard
all demo accounts, keys and model versions and import the configuration again:

```sh
docker compose down --volumes
```

Restart the complete stack after resetting. Keycloak's keys and Zot's trusted
certificate must agree; the initialization service handles this on a fresh
start. A running Zot process does not automatically reload that certificate.

## Other deployments

`subsoil serve --config <configuration.json> --listen <address>` needs a fixed
OCI upstream, its browser-facing registry URL, the authorization issuer and
browser client, requested scopes, and application launch registrations. It
requires no registry storage directory and offers no local account commands.
The upstream must enforce repository access for every request, including
catalog listing. An operator must not substitute an unrestricted registry
endpoint behind the workflow API.

The Compose configuration is a local demo using HTTP and known credentials.
Shared hosting needs HTTPS, persistent identity-provider storage appropriate to
that environment, application callback/origin registration, and a signing-key
rotation strategy. Configure object storage, retention, garbage collection,
and tag immutability in Zot. Subsoil's read-only browsing API and application
handoffs remain independent of those storage choices.
