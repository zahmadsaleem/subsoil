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
| `http://localhost:8025` | Demo verification emails |

The `zot-init` service fetches Keycloak's public signing certificate and
writes the registry and gateway configuration. The seed service publishes
`demo/model.ifc` as `project/model:issued` if the version is absent.

Select **Sign in** and register an account, then use the email in Mailpit
to verify it. **My workspace** contains three sample versions, each with
a distinct OCI artifact type. The
[development guide](DEVELOPMENT.md#try-the-starter-editor) shows how to
register and run the editor.

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

## Host on a VPS

Prepare one VPS with SSH key access. Allow inbound TCP 80 and 443 and
restrict SSH to your IP. Point three DNS names at the server for the file
browser, registry, and sign-in issuer. Only publish IPv6 DNS records for an
address configured and reachable on the VPS.

On Debian 13, the bootstrap playbook installs Docker Engine, Compose,
Git, Python, and sudo. It creates a named `deploy` account with your SSH
public key and grants it passwordless sudo for automation. Run the initial
bootstrap from your local Subsoil checkout using the VPS root account.

```sh
nix develop -c ansible-playbook -i 'vps.example.org,' -u root \
  --private-key ~/.ssh/id_ed25519 deploy/bootstrap.yml
```

The trailing comma makes the address an inline inventory. If Ansible is
already installed locally, run the command starting at `ansible-playbook`.
The default operator key is `~/.ssh/id_ed25519.pub` on the machine running
Ansible. Pass `-e subsoil_deploy_public_key_file=/absolute/path/to/key.pub`
for a different public key. Open a separate SSH session as `deploy` and
verify access with `sudo -n true`.

For a manual setup, follow the [Docker Engine installation guide for
Debian](https://docs.docker.com/engine/install/debian/).

The **Publish Deployment Images** workflow builds Subsoil and the Keycloak
image with the registry token mapper for each commit on `main`. It publishes
Linux amd64 and arm64 images to GitHub Container Registry (GHCR), tagged
with the full commit SHA. Wait for both images to publish before deploying.
After the first publish, set both packages to Public in
[GitHub Packages](https://github.com/zahmadsaleem/subsoil/packages) so the VPS
can pull them without a registry credential.

From the same local checkout, connect as the operator and pass the three
public hostnames to the hosted playbook.

```sh
nix develop -c ansible-playbook -i 'vps.example.org,' -u deploy \
  --private-key ~/.ssh/id_ed25519 deploy/hosted.yml \
  -e subsoil_files_host=files.example.org \
  -e subsoil_registry_host=registry.example.org \
  -e subsoil_auth_host=auth.example.org
```

The operator owns the checkout at `~/subsoil` and its generated credentials.
Ansible uses sudo for Docker pulls and Compose startup. It waits for the
sample file and checks all three public endpoints over HTTPS. To deploy a
specific tag or commit, pass `-e subsoil_revision=<ref>` for a revision with
published images. Rerun the playbook with the same hostnames to apply an
update with the existing credentials and volumes.

### Reset a legacy root deployment

The reset playbook removes the `subsoil-hosted` containers, networks, and
volumes associated with the old `/root/subsoil` checkout. This deletes
stored files and versions, Keycloak accounts and settings, TLS state, and
the old checkout and its generated credentials. Keep a backup of any
data you intend to restore.

Once the new operator login works, use check mode to preview the reset
from your local checkout.

```sh
nix develop -c ansible-playbook -i 'vps.example.org,' -u deploy \
  --private-key ~/.ssh/id_ed25519 deploy/destroy.yml \
  -e subsoil_confirm_destroy=subsoil-hosted --check
```

Remove `--check` to perform the reset. Run `deploy/hosted.yml` as the
operator after it completes.

### Enable hosted signup

After the first deployment, save Keycloak's SMTP settings as
`~/subsoil/build/hosted/smtp.json` while signed in as the operator on the
VPS. Set its permissions to `0600` and rerun the deployment. This example
uses implicit TLS on port 465.

```json
{
  "host": "smtp.example.org",
  "port": "465",
  "from": "subsoil@example.org",
  "ssl": "true",
  "auth": "true",
  "user": "subsoil@example.org",
  "password": "<smtp password>"
}
```

The identity setup enables signup and email verification in the existing
Keycloak realm when this file is present. The registry limits each request
body to 4 MiB and limits published content to 64 MiB per repository, with
a ceiling of 128 repositories. The per-repository quota counts published
content. Uncommitted uploads can use disk until the registry reclaims them.

### Manual deployment

As the operator, clone Subsoil on the VM and check out a commit with
published images. From its root, generate the hosted configuration with
Python 3. Substitute your DNS names in this command:

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

Use the checked-out commit to select the images, then pull and start them:

```sh
hosted=(sudo env "SUBSOIL_IMAGE_TAG=$(git rev-parse HEAD)" docker compose --env-file build/hosted/compose.env -f compose.hosted.yaml)
"${hosted[@]}" config --quiet
"${hosted[@]}" pull --policy always
"${hosted[@]}" up --no-build --pull never -d
"${hosted[@]}" ps -a
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
