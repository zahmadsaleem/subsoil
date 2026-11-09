# Application exchange and the proposed OCI handoff

Subsoil demonstrates an application workflow over independent storage and
identity services. Zot implements OCI Distribution. Keycloak handles identity
and permission decisions. Subsoil browses files and opens selected versions in
applications. IFC Lite reads and publishes directly against Zot's public address.

The `oci-handoff/1` format is a proposal implemented by this demonstration. It
is not an OCI, IETF, or buildingSMART standard. The transfer and authorization
protocols it uses are existing specifications.

## Build and check

```sh
cargo build --locked
cargo test --locked
cargo clippy --all-targets -- -D warnings
```

The live integration uses the Compose services:

```sh
docker compose --profile viewer up --build -d
docker compose --profile test run --rm smoke
```

IFC Lite's guide, `docs/guide/local-registry.md` in its checkout, describes its
connection configuration and corresponding application checks.

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

Application selection requires no profile registry or content matching rule.
Applications interpret OCI metadata and report unsupported formats themselves.
The IFC Lite demonstration accepts a manifest containing exactly one
`application/x-step` layer, checks its size and SHA-256, then loads it through
the viewer's ordinary model-loading path. Its publisher uses
`application/vnd.ifc-lite.ifc.v1` and the OCI empty config descriptor; these IFC
format choices are separate from the handoff protocol.

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

For opening, request `repository:project/model:pull`. For publishing, the
Keycloak demonstration requests these independently grantable scope values:

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
registered applications after checking that the selected manifest is readable.
`POST /handoff/launches` accepts `application_id`, `repository`, `reference`,
and `intent`, resolves the manifest through Zot, and returns a `launch_url`
containing the handoff fragment. These requests use the browser's registry token.
There is no launch credential, transaction store, or application-specific token
response extension.

## Optional conversion and OCI tools

The command line retains OCI client operations independently of Subsoil's web
workflow. Run `subsoil --help` for manifest inspection, blob retrieval, receiving,
copying and publication. HTTP development access requires `--plain-http`.

```sh
subsoil convert /path/to/model.ifc --output build/release --model-id design
subsoil verify build/release
```

Conversion creates Subsoil's experimental Parquet/GLB release format and
includes the original IFC by default. `--omit-source` excludes it. Applications
still decide which formats they can interpret; conversion is unnecessary for
opening and publishing the plain IFC artifact used by Compose.
