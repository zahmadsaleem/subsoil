# Subsoil pitch

## 01 Building tools is only half the job

### A small tool still needs the project data people already work with.

**Speaker notes**

Say I want to check whether the openings in a structural model line up with the services model. With AI coding tools, it’s easier to put together a small app for that check. Then I have to get both models into it. When either one changes, the app needs a way to get the new version and the team needs to know which versions it checked.

## 02 Two apps need the same data

### Each new tool can mean another export, another copy, and another place to check what changed.

**Speaker notes**

The structural model is in one system and the services model is in another. Each system has its own way to get data out. I can export both and run the check today. Next week I have to find the right revisions again, and the result may end up in a third place. If the platform charges separately for API access, the team pays again to work with data it already paid to create and store.

What I want is for a second app to pick up the data the first app used, do its part, and make the result available to whoever needs it next.

## 03 Use OCI to exchange project data

### An existing standard for sharing published versions across tools and organizations.

**Speaker notes**

My proposal is to use OCI, the Open Container Initiative standards used to package and distribute container images. The same standards can carry project files. An app can open a specific published version and publish an output using the same kind of registry API, even when the teams involved run different systems. [OCI packaging specification](https://github.com/opencontainers/image-spec/blob/v1.1.1/manifest.md) · [OCI distribution specification](https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md)

The apps still need to agree on what’s in the package and how to interpret it. Subsoil’s proposed handoff lets a CDE tell an app which version to open. The app gets its own access and reads that version from the registry. A connection to the source system’s API can handle live queries and approved edits to the source. [Subsoil architecture guide](ARCHITECTURE.md#what-implementations-agree-on)

## 04 One team’s output is the next team’s input

### Teams can iterate on shared data from the tools and systems they already use.

**Speaker notes**

The services team opens the structural model in its own tool and sends back a proposed change to an opening, with the geometry and the clearance it needs. The structural engineer reviews it in their own tool and publishes an updated structural model. That version is the starting point for the services team’s next check.

Each organization can keep its own system and decide what it shares. Each tool can keep its own way of working with the data. If an agent helps run the check, it can work from those same versions and return its findings for someone to review.

## 05 IFClite and Subsoil

### IFC models can travel through a registry. Subsoil shows how an app can open an exact version.

**Speaker notes**

IFClite gives us an example with real model data. Its experimental registry support lets an IFC tool open a model from an OCI registry. The proposed handoff adds a way for a project browser to pass the exact selected version to an app. [IFClite model registry documentation](https://ifclite.dev/docs/guide/local-registry/)

You can try the handoff in Subsoil with a sample file. Open it in the separate starter editor, make a change, and publish a new version. Both versions are then visible back in Subsoil.

## 06 Build an app for your own workflow

### Test it in Subsoil, then try a registry you control.

**Speaker notes**

Pick a task you actually want an app to do. Try it with a sample in Subsoil so you can see it open a specific version and, if it writes a result, publish that result where you choose. If your team wants to operate its own registry, try the same app with that connection and your own access rules.

Copy this prompt to your coding agent and replace the bracketed text with your use case.

```text
Using the OCI handoff protocol proposed by Subsoil, build or adapt an app that [describe what you want the app to do].

Use these guides for the handoff and the local test.
https://github.com/zahmadsaleem/subsoil/blob/main/docs/ARCHITECTURE.md#what-implementations-agree-on
https://github.com/zahmadsaleem/subsoil/blob/main/docs/DEVELOPMENT.md

Help me register the app in a Subsoil demo workspace and test it locally at http://localhost:5173. Use a sample the app understands and open the exact version selected in Subsoil. If the app produces a result, let me publish it as a new version to a repository I select and can write to.

Keep the registry and sign-in settings configurable. Show me what I would need to connect the same app to an OCI registry my team hosts or controls.
```

[Open the Subsoil demo](https://cde.subsoil.dev/)
