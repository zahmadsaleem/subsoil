# Connect an app

Use the starter editor to open a file from Subsoil and publish a new
version directly to Zot. The [architecture](ARCHITECTURE.md) describes
the handoff and standards for independent implementations.

## Try the starter editor

The demo allows the [starter editor](../examples/editor/) on port 5173.
Run it from the repository root.

```sh
python3 -m http.server 5173 --bind 127.0.0.1 --directory examples/editor
```

1. Sign in to [the hosted demo](https://files.lab.madsolame.com/explore) or
   [your local demo](http://localhost:5080/explore). Under **Workspace apps**,
   enter an app name, keep the editor defaults, and select **Register app**.
2. Copy **App configuration**. In **My workspace**, select `notes-v1` and
   choose **Edit with…**. Paste the configuration into the editor and sign in.
3. Edit the text, publish `notes-v2`, and refresh Subsoil to see the new version.

## Connect your own app

Register the exact callback URLs under `http://localhost:5173`, one per line. An app
with a launch URL appears under **Open with…** when it handles the selected
manifest's `artifactType` and requested action. These declarations let an
app edit notes and view reviews.

```json
{
  "handles": [
    {"artifact_type": "application/vnd.subsoil.demo.notes.v1", "intents": ["view", "edit"]},
    {"artifact_type": "application/vnd.subsoil.demo.review.v1+json", "intents": ["view"]}
  ]
}
```

Use the issuer, registry origin, public client ID, and repository from **App
configuration**. Request `repository:<repository>:pull` to open files and
add `repository:<repository>:push` to publish. Send each action as a separate
OAuth scope and set `resource` to the registry origin. The starter editor
uses the equivalent `openid workspace:read` and `workspace:write` scopes.
Keycloak grants access only to the registered workspace.

## Prompt for a coding agent

```text
Build a browser app on http://localhost:5173 that opens and publishes files
in a Subsoil workspace. Use the contract and starter editor at
https://github.com/zahmadsaleem/subsoil/blob/main/docs/ARCHITECTURE.md
and https://github.com/zahmadsaleem/subsoil/tree/main/examples/editor.
Use my workspace app configuration from the guide at
https://github.com/zahmadsaleem/subsoil/blob/main/docs/DEVELOPMENT.md.
Match OCI artifactType, open by digest, and publish a new tag directly to Zot.
Show the revision in Subsoil.
```
