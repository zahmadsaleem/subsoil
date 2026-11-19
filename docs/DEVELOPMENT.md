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

1. Sign in to the [local CDE](http://localhost:5080/) with a verified account.
   Open **My workspace**, set it up if prompted, and select **Workspace apps**.
2. Use `http://localhost:5173/` as the callback and launch URL. Enter
   `application/vnd.subsoil.demo.notes.v1` under **Edit package types**,
   register the app, and copy its configuration. In **My workspace**, open
   `notes-v1` and choose **Edit with** your app. Paste the configuration into
   the editor and sign in.
3. Edit the text, publish `notes-v2`, and refresh Subsoil to see the new version.

## Connect your own app

Register the exact callback URLs under `http://localhost:5173`, one per line. An app
with a launch URL can declare package types, file types, or both. This
registration lets an app open a work package and handle its PDFs individually.

```json
{
  "handles": [
    {"artifact_type": "application/vnd.subsoil.demo.work-package.v1", "intents": ["view"]}
  ],
  "file_handles": [
    {"media_type": "application/pdf", "intents": ["view"]}
  ]
}
```

Use the issuer, registry origin, public client ID, and repository from **App
configuration**. Request `repository:<repository>:pull` to open files and
add `repository:<repository>:push` to publish. Send each action as a separate
OAuth scope and set `resource` to the registry origin. The starter editor
uses the equivalent `openid workspace:read` and `workspace:write` scopes.
Keycloak grants access only to the registered workspace.

### Open a work package

In **My workspace**, select **Add sample work package**. Register the starter
app with the package and file types above in **View package types** and
**View file types**. Open **Level 02 coordination** and launch it with
**Open package with…** to browse the release, or choose **Open with…** next to
the drawing or checklist to start with that PDF. Paste the app configuration
and sign in to let it fetch the pinned manifest and files directly from Zot.

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
