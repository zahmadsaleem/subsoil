"""Prepare public addresses and first-start credentials for the hosted Compose stack."""

import argparse
import json
import pathlib
import re
import secrets
import tempfile


ROOT = pathlib.Path(__file__).resolve().parents[1]
OUTPUT = ROOT / "build" / "hosted"


def hostname(value):
    """An ASCII DNS name without a port or path."""
    label = r"[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?"
    value = value.lower()
    if len(value) > 253 or not re.fullmatch(fr"{label}(\.{label})+", value):
        raise argparse.ArgumentTypeError("use a DNS hostname such as files.example.org")
    if not value.rsplit(".", 1)[1].isalpha():
        raise argparse.ArgumentTypeError("use a DNS hostname")
    return value


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--files-host", required=True, type=hostname)
    parser.add_argument("--registry-host", required=True, type=hostname)
    parser.add_argument("--auth-host", required=True, type=hostname)
    args = parser.parse_args()
    if len({args.files_host, args.registry_host, args.auth_host}) != 3:
        parser.error("use distinct hostnames for files, registry and authorization")
    if OUTPUT.exists():
        parser.error("build/hosted already exists. Preserve it and its deployment credentials.")

    files = f"https://{args.files_host}"
    registry = f"https://{args.registry_host}"
    issuer = f"https://{args.auth_host}/realms/subsoil"
    admin_password, db_password, seed_secret, reader, writer = (
        secrets.token_hex(24) for _ in range(5)
    )

    demo = ROOT / "demo"
    subsoil = json.loads((demo / "subsoil.json").read_text())
    subsoil.update(registry=registry, authorization_server=issuer)
    realm = json.loads((demo / "keycloak" / "realm.json").read_text())
    realm["sslRequired"] = "all"
    realm["clients"] = [c for c in realm["clients"] if c["clientId"] != "wrong-audience"]
    for client in realm["clients"]:
        if client["clientId"] == "subsoil-browser":
            client["redirectUris"] = [files + "/explore"]
            client["webOrigins"] = [files]
        if client["clientId"] == "demo-seed":
            client["secret"] = seed_secret
        for mapper in client.get("protocolMappers", []):
            if mapper["protocolMapper"] == "subsoil-demo-registry-token":
                mapper["config"]["registry"] = registry
    for user in realm["users"]:
        if user["username"] in ("reader", "writer"):
            user["credentials"][0]["value"] = reader if user["username"] == "reader" else writer

    env = {
        "FILES_HOST": args.files_host,
        "REGISTRY_HOST": args.registry_host,
        "AUTH_HOST": args.auth_host,
        "KC_ADMIN_PASSWORD": admin_password,
        "KC_DB_PASSWORD": db_password,
        "SEED_SECRET": seed_secret,
    }
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(dir=OUTPUT.parent, prefix=".hosted-") as directory:
        staging = pathlib.Path(directory)
        generated = staging / "config"
        generated.mkdir(mode=0o700)
        subsoil_config = generated / "subsoil.json"
        subsoil_config.write_text(json.dumps(subsoil, indent=2) + "\n")
        imported = generated / "subsoil-realm.json"
        imported.write_text(json.dumps(realm, indent=2) + "\n")
        # Unprivileged containers read these files through their direct mounts.
        # The enclosing directory keeps the originals private on the host.
        subsoil_config.chmod(0o644)
        imported.chmod(0o644)
        env_file = generated / "compose.env"
        env_file.touch(mode=0o600)
        env_file.write_text("".join(f"{key}={value}\n" for key, value in env.items()))
        credentials = generated / "accounts.txt"
        credentials.touch(mode=0o600)
        credentials.write_text(f"reader: {reader}\nwriter: {writer}\nadmin: {admin_password}\n")
        generated.rename(OUTPUT)
    print("Created build/hosted. Keep accounts.txt and compose.env private.")


if __name__ == "__main__":
    main()
