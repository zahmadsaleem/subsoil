# Open CDE exchange proposal

Subsoil proposes a shared way to exchange issued models, drawings, and their
review records across common data environments (CDEs). Each organization can
work in the tools it already uses and keep a usable copy of the records it is
entitled to retain. The first test is a model handoff between a delivery team
and an asset owner.

## Design objectives

The [research findings](CDE_AI_RESEARCH.md#conclusion) identify work at three
points. During delivery, teams copy files and reconstruct their revisions or
approvals. At handover, owners need records they can use after the delivery
system closes. Specialist tools and AI introduce another handoff when they
read inputs and return results for review.

The [Subsoil philosophy](PHILOSOPHY.md) sets the design direction. A recipient
should be able to open a version in a compatible tool they already use and see
what it was issued for. A site engineer looking for the drawing to build from
and a designer reviewing a newer revision can both be right. The exchange
therefore has to carry the decision about use alongside the exact version.

## Keep the decisions with the release

An engineering firm keeps working models and reviews in its usual CDE. At
issue, it publishes an exact release containing the selected model, any
companion drawings, and their roles in the package. An issue record binds
that release to the project's document identifier, revision, scope of work,
intended use, issuer, and time of issue. The owner's acceptance is a separate decision
about the same release, made by the person authorized under the project
agreement. Keeping decisions linked to the exact release lets a team add a
decision without rewriting the model or copying it between status folders.

Show the file, revision, and permitted use together wherever people choose
what to open. A newer version issued for review can appear alongside an earlier
version still authorized for construction. Give site teams a link to the
version they were sent, and record a withdrawal or replacement as a new
decision so the earlier history remains intelligible. An approval made in an
existing review system needs to be captured as part of the exchange with a
reference to its evidence.

## Choose how files and information travel

A single project CDE can simplify oversight when everyone can work there.
Direct vendor APIs can serve jobs that depend on live features, and
buildingSMART's [openCDE Documents API](https://github.com/buildingSMART/documents-API)
already provides document selection, download, and upload between compatible
tools. Subsoil proposes an exchange for **published releases** alongside those
workflows. A connector can read an issued version through an existing API or
accept a controlled export, check the required fields, and publish it in the
agreed form. Start with an export where the handoff is infrequent. Automate
capture when repeated transfers justify maintaining a connector.

Use the Open Container Initiative (OCI) standards for packaging and delivery.
An OCI manifest lists the files in a release and the content fingerprints
needed to check them. Its own fingerprint, called a digest, identifies the
complete release even if a version name moves. Compatible registries and
clients can retrieve and copy it using the same distribution protocol. A
copy must include the decision and result records the recipient needs. When
the related records share a repository, a supported registry can also list
them by their subject. Verify those records as well as the files during a
transfer or restore. [OCI Image Specification](https://github.com/opencontainers/image-spec/blob/v1.1.1/manifest.md)
[OCI Distribution Specification](https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md)

The parties still need a published content profile for the exchange. Agree
which files are required, which IFC export and drawing formats the recipient
can use, and which classifications, units, identifiers, and relationships
must survive. Shared definitions or an ontology can help when the receiving
tool has to interpret objects or follow relationships. Validate those fields
in the receiving tool. Keep native files where editable fidelity or a licensed
engine is needed, and use the source application's API for live queries and
authorized edits. [Technical architecture](ARCHITECTURE.md#what-implementations-agree-on)

## Keep custody and access under project rules

The producer retains working records and shares the releases it is authorized
to issue. The owner retains accepted releases and the evidence it needs in a
system it controls, whether it operates the service itself or uses a provider.
The project agreement assigns authority to issue, accept, correct, retain, and
reuse records. It also sets the rules for sensitive material and secondary
uses such as AI training.

Give each role the access it needs. A reader should be able to open an issued
version without gaining the ability to publish. Treat submitting, issuing,
accepting, and deleting as separate permissions. For the pilot, give authors
write access to a draft area and reserve issued writes for the authorized
publication service. Enforce the issue policy at the service that accepts
writes so the same rules apply through the CDE interface and the registry API.
Record who acts and when, and retain enough evidence for the owner to verify
the decision using the project's trust rules. A digest checks which bytes
arrived. Authority and approval come from the decision record and the rules
for verifying it.

Plan for a change of operator or service. Agree which copies the owner must
retain, how to recover them, and who takes over reviews when people leave.
The cost of running the exchange includes account administration, connector
maintenance, recovery, and any licences required for people who need to open
the files.

## Let applications work on exact versions

When someone selects **Open with…** or **Edit with…**, Subsoil's handoff tells
the app which version to open and what the person wants to do. The app requests
its own read access to the source folder and separately obtains permission to
publish to the chosen destination. The same release can be opened by a reader, a
checker, or another compatible app. A registry client such as ORAS can move
packages independently of the Subsoil interface. [Browser handoff](ARCHITECTURE.md#browser-handoff)
[ORAS artifact tools](https://oras.land/docs/commands/oras_cp/)

Start with a bounded operation such as validating an IFC against a defined
ruleset. Publish the report with links to the exact input and ruleset versions
and identify the tool that ran the check. The responsible person reviews the
report in the existing process. An AI agent can call the same operation through
a tool interface such as the Model Context Protocol (MCP). The operation and
its permissions still govern what can be read or changed. A report destined
for review needs permission to write to that destination and a separate decision
before it is accepted as part of the project record. [MCP tools specification](https://modelcontextprotocol.io/specification/2025-11-25/server/tools)

Other applications can use the same version references. A federation tool
can record which versions it combined and how it aligned them. A comparison
tool can name both versions and the kinds of changes it checks. Analysis
across projects also depends on comparable measures and permission to reuse
the records.

## What the prototype demonstrates

The Subsoil example uses Zot to store published files and Keycloak for sign-in
and scoped access. A user can open a version or an individual file from a work
package in a registered app and publish a new version from the starter editor.
That demonstrates the exact-version handoff and publication. The pilot adds
governed issue and acceptance, verification of decision records, and recovery
from a copy held by the owner. [Connect an app](DEVELOPMENT.md)

## Pilot and acceptance criteria

Use one model exchange that the two organizations already repeat. Before the
test, agree who issues and accepts the release, what the owner must retain,
and which fields and file formats the recipient needs. Record how long the
current workflow takes and where people manually copy files, re-enter metadata,
request access, or confirm approvals. Include the person who will actually use
the drawing or model on site or in operations in the acceptance test.

Issue a first model for construction and accept it on the owner's side. Then
publish a second revision for review. Ask the recipient to open the version
they can use for construction and identify the issue and acceptance evidence.
Run the documented check in a second app and verify that the report points to
the exact input and ruleset. Try to issue as a user with draft access, through
both the CDE and the registry API, and verify that both block the action.

Disable access to the source for the test. Using the owner's retained copy,
recover the first version and its decisions with another compatible tool.
Check the required fields and verify that the report still links to its input.
Compare time, manual steps, and access and support effort with the baseline.
The trial succeeds if the recipient can choose and verify the right version,
the authorization rules hold, and the records remain usable independently
of the original account. Use the observed effort to decide whether to
maintain the connector or extend the exchange to another project workflow.
