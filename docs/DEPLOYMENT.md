# Run the CDE demonstration

Subsoil browses folders and versions in Zot. Keycloak signs repository tokens
and Envoy checks them at the registry address. Subsoil is stateless.

## Run locally

Clone Subsoil and install Docker with Compose:

```sh
docker compose up --build -d
```

| Address | Service |
| --- | --- |
| `http://localhost:5080/explore` | Subsoil Files |
| `http://localhost:5001/v2/` | Zot through its Envoy gateway |
| `http://localhost:8080` | Keycloak |

The `zot-init` service fetches Keycloak's public signing certificate and
writes the registry and gateway configuration. The seed service publishes
`demo/model.ifc` as `project/model:issued` if the version is absent.

Sign in to [Subsoil Files](http://localhost:5080/explore) as `writer` with
`writer-demo-password`. Open `project/model`, choose `issued`, and download
`model.ifc`. The `reader` account (`reader-demo-password`) can receive the
same version and cannot publish. The local Keycloak administrator is
`admin` / `admin-demo-password`.

### Publish a version

Install [ORAS](https://oras.land/docs/installation) on the machine running
the client. From the repository root, publish a new sample version:

```sh
oras login --plain-http --username writer localhost:5001
oras push --plain-http --artifact-type application/vnd.subsoil.file.v1 \
  localhost:5001/project/model:revision-2 demo/model.ifc:application/x-step
oras logout localhost:5001
```

Enter the writer password when prompted. Refresh Subsoil and open `revision-2`
to see the published file. To check role enforcement, log in as `reader` and
try publishing to a fresh tag. Zot should deny the upload. For an HTTPS
deployment, use its registry hostname and omit `--plain-http`.

### Check the exchange

```sh
docker compose --profile test run --rm smoke
docker compose ps -a
docker compose logs subsoil keycloak registry zot seed
```

The smoke test uses real OAuth PKCE and native OCI token exchanges. It checks
that readers can browse and download and are denied uploads, then publishes
an edited sample directly to Zot and checks that Subsoil lists the revision.

`docker compose down` preserves volumes. For a disposable local demo,
`docker compose down --volumes` also removes identities, signing keys, files,
and versions. The next startup reimports the local Keycloak realm.

## Host on Hetzner Cloud

Prepare one Ubuntu VM with an SSH key and the Docker Engine, Buildx, and
Compose plugins. Allow TCP 80 and 443 in the Hetzner firewall and restrict
SSH to your IP. Point three DNS names at the server for the file browser,
registry, and sign-in issuer. Only publish IPv6 DNS records for an address
configured and reachable on the VM.

Clone Subsoil on the VM. From its root, generate the hosted configuration
with Python 3. Substitute your DNS names in this command:

```sh
python3 deploy/configure.py \
  --files-host files.example.org \
  --registry-host registry.example.org \
  --auth-host auth.example.org
```

The command creates `build/hosted/` with distinct generated passwords and
the Compose environment. Keep that directory private and back it up.
`accounts.txt` contains the generated reader, writer, and administrator
passwords. The Keycloak realm is imported on initial startup, so regenerating
local files alone cannot rotate credentials in an existing database.

Start Compose using the generated environment:

```sh
docker compose --env-file build/hosted/compose.env -f compose.hosted.yaml config --quiet
docker compose --env-file build/hosted/compose.env -f compose.hosted.yaml up --build -d
docker compose --env-file build/hosted/compose.env -f compose.hosted.yaml ps -a
```

Caddy obtains TLS certificates once DNS and the firewall are ready. Open
the files hostname at `/explore` and repeat the publish and read-only checks
using the generated accounts. Caddy exposes the Keycloak demo realm for
sign-in. The admin console, database, and raw Zot listener stay inside the
Docker network.

Keep recoverable backups of the PostgreSQL database and Zot's `zot-data`
volume, plus the generated configuration and deployment credentials. Use
`pg_dump` for the database and take a consistent copy of the registry volume
with writes paused. Preserve both when replacing the VM.

## Connect external applications

Subsoil loads its public registry origin, a fixed private registry gateway,
authorization issuer, client ID, scopes, and application registrations from
`subsoil serve --config <configuration.json> --listen <address>`. The
[development guide](DEVELOPMENT.md#register-an-application) shows how to
register another application. The gateway behind Subsoil must enforce
repository access on every request, including catalog listing.
