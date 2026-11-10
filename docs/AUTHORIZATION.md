# Authorization across the exchange

Identity and authorization belong to the authorization service. Subsoil does
not maintain accounts, passwords, grant databases, or a local token issuer.
The Compose demonstration uses Keycloak. Applications authorize independently
and transfer registry content directly through Zot's public address.

## Login and permissions

Keycloak registers separate public clients for Subsoil's browser and IFC Lite.
Their exact callbacks and browser origins are in `demo/keycloak/realm.json`.
Browser login uses Authorization Code with PKCE S256. Tokens are obtained from
Keycloak's ordinary OIDC token endpoint and use the normal OAuth response
fields. Subsoil and IFC Lite do not require custom session or permission JSON.

The demonstration has two people:

| Account | Password | File access |
| --- | --- | --- |
| `reader` | `reader-demo-password` | Receive files in `project/model` |
| `writer` | `writer-demo-password` | Receive and publish in `project/model` and `reviews/model` |

Each repository action has an identically named realm role and optional client
scope, such as `repository:project/model:pull` or
`repository:project/model:push`. Keycloak's scope-role mappings remove client
scopes the account cannot use. The demo mapper checks roles again and encodes
the granted actions in the Distribution JWT `access` claim. Its Docker protocol
mapper applies the same repository role checks to conventional OCI clients.

Catalog browsing additionally requests `registry:catalog:read` and the concrete
repository pull scopes. Catalog permission alone grants no repository reads.
Zot uses the token's repository permissions to filter its catalog. Subsoil
paginates those results for its Files page.

The optional client scopes enumerate the demo's folders. Adding another folder
requires its realm roles, client scopes and scope-role mappings, and the desired
browser listing scope. This is an intentionally small demo policy adapter,
not a general policy language or an authorization standard.

## What the standards cover

[OAuth 2.0](https://www.rfc-editor.org/rfc/rfc6749.html) defines the code and token
exchange. [PKCE](https://www.rfc-editor.org/rfc/rfc7636.html) binds a code to its
initiating client transaction. [Bearer token usage](https://www.rfc-editor.org/rfc/rfc6750.html)
defines the Authorization header and challenge behavior. The
[Distribution token specification](https://distribution.github.io/distribution/spec/auth/token/)
and [JWT format](https://distribution.github.io/distribution/spec/auth/jwt/)
define repository scopes and the registry's `access` claims. Those claim names
are not a generic OAuth or JWT requirement.

A handoff supplies a content locator. It grants no access, and `view`/`edit`
intent is not authorization. Repository pull permission normally covers all
readable versions in that repository. Tag- or digest-specific delegation is
outside the portable contract demonstrated here.

## Validation and service boundaries

Keycloak signs tokens. Envoy, at the public Zot address, validates their
signature, issuer, audience and time bounds using the public JWKS prepared from Keycloak. Zot
validates its trusted signing certificate and enforces the repository actions.
The extra gateway check is necessary for this pinned demo: Zot v2.1.23's
traditional Bearer verifier does not itself enforce the configured issuer and
audience. Zot has no host port; access from applications goes through Envoy.

Subsoil forwards browser tokens only to its operator-configured private
registry upstream. It cannot turn a token into broader access and exposes no
OCI storage endpoint. The browser's access and refresh tokens remain in memory;
only pending OAuth state and the PKCE verifier temporarily use session storage.
Disconnecting the file browser clears its tokens. Keycloak manages the separate
SSO session, so the next authorization may reuse that login.

IFC Lite authorizes the source for pull and a chosen destination for pull and
push. The demo requests read access for publication because it checks whether a
version name already exists. Denied permissions require the owner to grant
access; changing a handoff or hiding a button cannot change server enforcement.

## Demonstration limits

Compose publishes local HTTP ports and includes known demo passwords. Keycloak
runs in development mode. The initialization container snapshots the realm's
current RSA signing certificate for Zot; the demo does not automate Zot's
certificate reload after Keycloak key rotation. The initialization service also prepares Envoy’s JWKS snapshot, including Keycloak’s Docker key-ID aliases. Both verifiers need a refresh when the signing key rotates.
These choices support a local demonstration and need deliberate deployment
configuration for shared hosting.

Issued tokens expire after five minutes. Removing a role changes newly issued
tokens; the gateway and Zot can continue accepting an already-issued token
until its expiry. An existing token is not an online permission lookup. See the
[deployment guide](DEPLOYMENT.md) for services and verification.
