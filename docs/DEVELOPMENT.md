# Application exchange and the proposed OCI handoff

Subsoil demonstrates an application workflow over independent storage and
identity services. Zot implements OCI Distribution. Keycloak handles identity
and permission decisions. Subsoil browses files and opens selected versions in
applications. Clients read and publish directly against Zot's public address.

The `oci-handoff/1` format is a proposal implemented by this demonstration. It
is not an OCI, IETF, or buildingSMART standard. The transfer and authorization
protocols it uses are existing specifications.

## Build and check

```sh
go build -o build/subsoil .
go test ./...
go vet ./...
```

The live integration uses the Compose services:

```sh
docker compose up --build -d
docker compose --profile test run --rm smoke
```

## Register an application

A workspace owner can use **Workspace apps** in Subsoil or send
`POST /workspaces/<id>/apps` with their Subsoil browser token. Get the
workspace ID and repository from `POST /workspace`, which seeds sample
versions on the first visit. Register for the exact local developer origin
`http://localhost:5173`:

```json
{
  "name": "Project editor",
  "redirect_uris": ["http://localhost:5173/"],
  "launch_url": "http://localhost:5173/",
  "handles": [
    {"artifact_type": "application/vnd.subsoil.demo.notes.v1", "intents": ["view", "edit"]},
    {"artifact_type": "application/vnd.subsoil.demo.checklist.v1+json", "intents": ["view", "edit"]},
    {"artifact_type": "application/vnd.subsoil.demo.review.v1+json", "intents": ["view"]}
  ]
}
```

On success the registrar returns the app ID, public OAuth client ID, issuer,
registry URL, and workspace repository alongside the registration. Copy the
app configuration into the app. The registrar sets the exact Keycloak web
origin for its token request. Compose allows the developer origin at the
registry gateway for OCI reads and uploads.

List existing entries with `GET /workspaces/<id>/apps` and remove one with
`DELETE /workspaces/<id>/apps/<app>`. Removing an app stops new sign-ins
for that client. A registry token already issued expires on its usual schedule.
An optional `launch_url` makes the app selectable in **Open with…**.

Operator-configured apps can also be placed in the `applications` list for
repositories outside the personal workspaces. Set their `id`, `name`,
`launch_url`, and the same `handles` entries.

## Try the starter editor

Run this from the repository root:

```sh
python3 -m http.server 5173 --bind 127.0.0.1 --directory examples/editor
```

In Subsoil, open **Workspace apps**, enter a name, and register the default
editor. Open `notes-v1` from **My workspace** using **Edit with…**.
Paste the registration's configuration into **Connect to Subsoil** in the
editor, sign in, edit the text, and push a new version. Refresh Subsoil to
see it in the workspace. The same starter serves as a preview app at
`http://localhost:5173/index.html`. Register that URL as both the callback
and launch URL with just the `view` intent to see two choices in **Open with…**.

To start a new app with a coding agent, copy this prompt:

```text
Build a browser app that opens and publishes versions in a Subsoil workspace.
Read the handoff and artifact contract in
https://github.com/zahmadsaleem/subsoil/blob/main/docs/DEVELOPMENT.md
and the permissions in
https://github.com/zahmadsaleem/subsoil/blob/main/docs/AUTHORIZATION.md.
Use the workspace app registration settings and run on http://localhost:5173.
Accept oci-handoff/1, authorize with PKCE S256, and fetch the file by digest.
Offer Push to… and upload OCI blobs and a new manifest tag directly to Zot.
Show the version in Subsoil and confirm that another workspace is denied.
```

## Open with

A handoff identifies content, independently of permission to retrieve it:

```json
{
  "protocol": "oci-handoff/1",
  "source": {
    "registry": "http://localhost:5001",
    "repository": "project/model",
    "digest": "sha256:32982f1d25a7c6d12f627c1fbc5d3d09df56514c3a24de7647bcc6808d6be214"
  },
  "intent": "edit"
}
```

`registry` is the public OCI registry origin. `repository` is its folder name
in the API. `digest` pins the manifest bytes using SHA-256; a tag is resolved
before handing content to an application. `intent` is `view` or `edit`, a user
interface hint that grants no authority. Opening a digest does not restrict a
repository-level access token to that digest.

An optional `destination` can suggest a `registry` and `repository`. It neither
authorizes publication nor fixes the version name. An application can publish
without any preceding handoff and can choose another configured destination.

Serialize the object as UTF-8 JSON and encode it using base64url without
padding. Append `#handoff=<encoded-object>` to the application's launch URL.
The fragment contains no passwords, authorization codes, access tokens, or
refresh tokens. The application validates the object, checks the registry
against its configured connections, removes the fragment from the address,
and obtains its own authorization.

Subsoil matches every app's declared artifact types against the top-level
`artifactType` of the selected manifest. It offers the matching `view` and
`edit` intents and rechecks the chosen app at launch. For example,
`oras push --artifact-type application/vnd.subsoil.demo.notes.v1 …`
sets the type used by the notes editor. The file's own media type is recorded
in its layer descriptor, such as `text/plain` for `notes.txt`. The demo
file package has one file layer and an OCI empty config descriptor.

## Authorization and push to

The application configures a registry origin, authorization issuer, and public
OAuth client ID. A handoff cannot supply a new issuer or token endpoint. Discover
endpoints using [RFC 8414 authorization server metadata](https://www.rfc-editor.org/rfc/rfc8414.html)
or [OpenID Connect Discovery](https://openid.net/specs/openid-connect-discovery-1_0.html),
verify the exact issuer, and require PKCE S256.

Use Authorization Code with [PKCE](https://www.rfc-editor.org/rfc/rfc7636.html),
exact callback registration, a fresh verifier and state, and issuer-bound
callback validation using [RFC 9207](https://www.rfc-editor.org/rfc/rfc9207.html).
Follow the [OAuth security BCP](https://www.rfc-editor.org/rfc/rfc9700.html).
Passwords belong on the authorization server's page. Applications keep access
tokens in tab memory and send them only to the registry they authorize.

Workspace browser apps request `openid workspace:read` for opening. Add
`workspace:write` to publish a revision. Keycloak converts the workspace
permissions into exact OCI repository actions for the registered client
and signed-in owner. The shared IFC fixture uses concrete OAuth scopes:

```text
repository:project/model:pull repository:project/model:push
```

A space separates OAuth scope values. Distribution also permits comma-separated
actions within a repository scope. The browser demo separates actions so
Keycloak can filter each client scope through its role mappings. Clients
understand the permissions returned in the ordinary OAuth `scope` field and
treat omitted `scope` as the originally requested scope, as specified by
[RFC 6749](https://www.rfc-editor.org/rfc/rfc6749.html).

Include the registry URL as the [RFC 8707 resource indicator](https://www.rfc-editor.org/rfc/rfc8707.html).
The demo mapper checks the token request's resource against the registry
configured for that client and sets its registry audience. A different
registry connection requires authorization for that destination. Source tokens
are never reused merely because another registry accepts Bearer authentication.

Transfer uses [OCI Distribution 1.1](https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md):
check the new version name, upload config and file blobs, then commit the
manifest. The successful manifest PUT completes publication. Return its digest
for reopening; optional metadata or a later listing failure cannot undo that
success. The viewer rejects an existing tag before uploading and checks again
before committing. This client check is not atomic tag immutability; the
registry deployment owns any immutability policy. Digest references remain
content-addressed regardless of tag changes.

## Subsoil's workflow API

`subsoil serve --config demo/subsoil.json --listen 127.0.0.1:5080` starts the
workflow application. Its configuration selects a public registry URL, a fixed
private upstream for the same deployment, the browser's authorization issuer,
client ID and scopes, and registered application IDs, names and launch URLs.
`demo/subsoil.json` uses Docker service names for its private upstream and is
mounted into the Compose container.

The browser uses the read-only `/files/` API. Subsoil forwards its Bearer token
to the configured registry gateway and fetches manifests, lists and files
through OCI APIs. The gateway and Zot enforce that access. Subsoil never issues
tokens, accepts uploads, or stores registry content. Listing follows bounded
OCI pagination and returns only the registry's authorized results.

`GET /handoff/applications?repository=<name>&reference=<tag-or-digest>` lists
matching applications after checking that the selected manifest is readable.
`POST /handoff/launches` accepts `application_id`, `repository`, `reference`,
and `intent`, resolves the manifest through Zot, and returns a `launch_url`
containing the handoff fragment. These requests use the browser's registry token.
There is no launch credential, transaction store, or application-specific token
response extension.
