"""Set up workspace clients in a new or existing Keycloak realm."""

import json
import os
from pathlib import Path
import time
import urllib.error
import urllib.parse
import urllib.request


KEYCLOAK = "http://keycloak:8080"
REALM = "/admin/realms/subsoil"
REGISTRAR = "subsoil-registrar"
BRANDING = {
    "displayName": "Subsoil", "displayNameHtml": "Subsoil",
    "loginTheme": "subsoil", "accountTheme": "subsoil",
}


def request(path, method="GET", data=None, token=None, form=False):
    headers = {"X-Forwarded-Proto": "https"}
    if token:
        headers["Authorization"] = "Bearer " + token
    body = None
    if data is not None:
        headers["Content-Type"] = (
            "application/x-www-form-urlencoded" if form else "application/json"
        )
        body = (urllib.parse.urlencode(data) if form else json.dumps(data)).encode()
    req = urllib.request.Request(KEYCLOAK + path, body, headers, method=method)
    with urllib.request.urlopen(req, timeout=10) as response:
        body = response.read()
        return json.loads(body) if body else None


def admin_login():
    for attempt in range(120):
        try:
            return request(
                "/realms/master/protocol/openid-connect/token", "POST",
                {
                    "grant_type": "password",
                    "client_id": "admin-cli",
                    "username": "admin",
                    "password": os.environ["KC_ADMIN_PASSWORD"],
                }, form=True,
            )["access_token"]
        except (OSError, urllib.error.HTTPError):
            if attempt == 119:
                raise
            time.sleep(1)


def configure():
    token = admin_login()

    def admin(path="", method="GET", data=None):
        return request(REALM + path, method, data, token)

    def client(name):
        found = admin("/clients?" + urllib.parse.urlencode({"clientId": name}))
        return next((item for item in found if item["clientId"] == name), None)

    def ensure_subject(path, mappers):
        if not any(item["protocolMapper"] == "oidc-sub-mapper" for item in mappers):
            admin(path, "POST", {
                "name": "Account subject", "protocol": "openid-connect",
                "protocolMapper": "oidc-sub-mapper",
                "config": {"access.token.claim": "true", "introspection.token.claim": "true"},
            })

    local_demo = os.getenv("LOCAL_DEMO") == "true"
    smtp = None
    if local_demo:
        smtp = {"host": "mailpit", "port": "1025", "from": "demo@subsoil.test"}
    else:
        file = Path("/settings/smtp.json")
        if file.is_file():
            smtp = json.loads(file.read_text())
            if not isinstance(smtp, dict) or not smtp.get("host") or not smtp.get("from"):
                raise ValueError("smtp.json must set host and from")

    realm = BRANDING.copy()
    if smtp:
        # Existing fixture accounts are managed by the demo and keep their login.
        for name in ("reader", "writer"):
            users = admin("/users?" + urllib.parse.urlencode({"username": name, "exact": "true"}))
            if users and users[0].get("email") in (None, name + "@example.test"):
                admin("/users/" + users[0]["id"], "PUT", {
                    "email": name + "@fixtures.subsoil.invalid",
                    "emailVerified": True,
                })
        realm.update(registrationAllowed=True, verifyEmail=True, smtpServer=smtp)
    admin("", "PUT", realm)

    registrar = client(REGISTRAR)
    if not registrar:
        admin("/clients", "POST", {
            "clientId": REGISTRAR,
            "protocol": "openid-connect",
            "enabled": True,
            "publicClient": False,
            "standardFlowEnabled": False,
            "directAccessGrantsEnabled": False,
            "serviceAccountsEnabled": True,
            "fullScopeAllowed": False,
            "defaultClientScopes": [],
            "optionalClientScopes": [],
        })
        registrar = client(REGISTRAR)
    registrar_path = "/clients/" + registrar["id"]
    management_path = "/clients/" + client("realm-management")["id"]
    account = admin(registrar_path + "/service-account-user")
    roles = "/users/" + account["id"] + "/role-mappings" + management_path
    admin(roles, "POST", [admin(management_path + "/roles/manage-clients")])
    # Limit the service-account token to the realm-management client role it needs.
    admin(registrar_path + "/scope-mappings" + management_path, "POST", [
        admin(management_path + "/roles/manage-clients"),
    ])
    mapper_path = registrar_path + "/protocol-mappers/models"
    mappers = admin(mapper_path)
    ensure_subject(mapper_path, mappers)
    if not any(mapper["name"] == "registrar client roles" for mapper in mappers):
        admin(mapper_path, "POST", {
            "name": "registrar client roles", "protocol": "openid-connect",
            "protocolMapper": "oidc-usermodel-client-role-mapper",
            "config": {
                "usermodel.clientRoleMapping.clientId": "realm-management",
                "claim.name": "resource_access.realm-management.roles",
                "multivalued": "true", "access.token.claim": "true", "jsonType.label": "String",
            },
        })

    browser_path = "/clients/" + client("subsoil-browser")["id"]
    browser = admin(browser_path)
    callbacks = browser["redirectUris"]
    if local_demo:
        callback = "http://localhost:5080/sign-in/callback"
        if callback not in callbacks:
            callbacks = [*callbacks, callback]
    else:
        settings = json.loads(Path("/settings/subsoil.json").read_text())
        callbacks = [settings["cde"]["origin"] + "/sign-in/callback"]
    if browser["redirectUris"] != callbacks:
        browser["redirectUris"] = callbacks
        admin(browser_path, "PUT", browser)

    scopes = {scope["name"]: scope for scope in admin("/client-scopes")}
    for name, label in (
        ("workspace:read", "Read files in your demo workspace"),
        ("workspace:write", "Publish versions in your demo workspace"),
    ):
        if name not in scopes:
            admin("/client-scopes", "POST", {
                "name": name, "protocol": "openid-connect",
                "attributes": {
                    "include.in.token.scope": "true",
                    "display.on.consent.screen": "true",
                    "consent.screen.text": label,
                },
            })
            scopes = {scope["name"]: scope for scope in admin("/client-scopes")}
        admin(browser_path + "/optional-client-scopes/" + scopes[name]["id"], "PUT")

    mapper_path = browser_path + "/protocol-mappers/models"
    mappers = admin(mapper_path)
    mapper = next(item for item in mappers if item["protocolMapper"] == "subsoil-demo-registry-token")
    mapper["config"]["workspaceBrowser"] = "true"
    admin(mapper_path + "/" + mapper["id"], "PUT", mapper)
    ensure_subject(mapper_path, mappers)

    secret = admin(registrar_path + "/client-secret")["value"]
    temporary = Path("/identity/registrar.tmp")
    temporary.write_text(json.dumps({
        "client_id": REGISTRAR, "client_secret": secret,
        "issuer": KEYCLOAK + "/realms/subsoil",
    }))
    temporary.chmod(0o400)
    os.chown(temporary, 65532, 65532)
    temporary.replace("/identity/registrar.json")
    print("Configured workspace identity and app registration.", flush=True)


if __name__ == "__main__":
    configure()
