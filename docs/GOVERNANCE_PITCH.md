# Open CDE governance pitch

## 01 Who gets to keep the project record?

### Teams need their records long after a project closes.

**Speaker notes**

An engineer can work on three jobs and spend the week in three clients’ common data environments, or CDEs. When a job closes, the firm needs a record of what it issued. Years later, the owner needs to open the model and see what it was approved for. [Infrastructure Client Group on project CDEs](https://www.ice.org.uk/media/3k0h5ofz/nc-ddtg-future-of-cdes-paper.pdf)

## 02 The handoff is where context gets lost

### The receiving team needs to know which version it can use.

**Speaker notes**

Say the firm sends an IFC to the owner. If the two CDEs handle revisions and status differently, someone may have to enter that information again and match the file to its approval. That work comes back when the owner moves the records to another system. [UK BIM Framework guidance on transferring metadata](https://ukbimframework.org/wp-content/uploads/2021/02/Guidance-Part-C_Facilitating-the-common-data-environment-workflow-and-technical-solutions_Edition-2.pdf#page=16)

## 03 Each organization keeps control of its data

### The project agrees who can issue, accept, and keep each release.

**Speaker notes**

The engineering firm keeps its working models in a system it controls and shares the versions it’s responsible for issuing. The owner keeps the approved model and its release record in its own system. Both teams can consult the records they’re entitled to keep after the project account closes.

## 04 Connect the systems teams already use

### Use OCI as the common way to exchange issued versions.

**Speaker notes**

The firm can keep review in its existing CDE. A connector could send an issued model and its approval record over OCI. The owner can load quantities from that model into its lake or warehouse and issue a table back through the same exchange, with a link to the model it came from. [OCI Distribution Specification](https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md)

## 05 Prove it on one model

### Show that the project can control what gets issued and recover an earlier release.

**Speaker notes**

In the pilot, an author can submit a draft. Both the CDE and the registry API must block an unauthorized release. Have the designated people authorize and accept the model, then issue a second revision. The owner should still be able to verify the first approval.

## 06 Bring one real handoff

### Two organizations, one model, and the project’s approval rules.

**Speaker notes**

Start with the two systems the teams already use. Once the release has crossed, disconnect the source. Ask the owner to open the exact model and show who approved it and for what use. Then try the same exchange with a compatible replacement on one side.
