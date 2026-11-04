# Project information and the changing AEC software landscape

Architecture, engineering, and construction (AEC) organizations need project information to remain useful as applications,
partners, and responsibilities change. Common data environments help organize
delivery, but information still has to cross organizational boundaries. AI and
the growing ability to build small applications make those boundaries more
consequential: a new tool needs understandable data, appropriate access, and a
way to return results that other people can trust.

The Open Container Initiative (OCI) offers a possible shared foundation for portable published versions and
linked results. Organizations can choose compatible applications, registries,
and hosting without adopting a single provider's product. ORAS supplies tools
for working with OCI artifacts; Subsoil is a usable demonstration of this
approach. The opportunity depends on maintained integrations, agreed content
formats, and sound information management as well as reliable transfer.

## Collaboration across several environments

The Infrastructure Client Group describes large projects using several
disconnected systems despite attempts to standardize on one CDE. Mandating a
client's system can leave delivery partners duplicating information or adapting
to a different platform for every customer. Its paper, informed by a November
2025 expert discussion, recommends open information structures, portability,
and governance across connected environments. It also recognizes the added
integration and governance complexity of an open ecosystem. [1]

Academic research supports the concern about information crossing boundaries.
A literature review of 46 publications combined with interviews with 15 BIM
experts identifies project complexity and simultaneous use of multiple CDEs as
sources of accountability, transparency, and reliability problems. The study is
qualitative evidence about recurring difficulties, rather than a measurement
of their prevalence across the industry. [2]

This is consistent with the UK BIM Framework's distinction between a CDE
workflow and the technical solutions supporting it. Its guidance addresses
metadata moving between solutions, revision control, and the status assigned
to information. A delivery mechanism therefore needs to fit an information
workflow; transferring a file does not establish its suitability for a task. [3]

For a project team, the practical questions are familiar. Which version is this?
What was it issued for? Which inputs produced this report? Who can use it?
Those questions offer a clearer introduction to a shared delivery architecture
than its storage terminology.

## Dependence on software suppliers

The Nordic architectural associations' September 2022 open letter to Autodesk
describes dissatisfaction with licensing costs, unwanted application bundles,
interoperability, performance, and responsiveness to user needs. The four
associations represented more than 14,000 architects. The letter is evidence
of organized historical dissatisfaction; its individual product complaints
should not be presented as a current assessment without fresh verification. [4]

Recent practitioner discussions illustrate the operational side of dependence.
A February 2026 discussion describes updating training materials, BIM execution
plans, and client communications around platform changes. Participants disagree
about whether those changes are justified. A separate March 2026 feature request
asks for better continuity of responsibilities, reviews, and issue history when
a user leaves a project. These are firsthand accounts and requests, not verified
product audits or representative surveys. [5][6]

The implication for a shared delivery foundation is practical freedom to choose
and change compatible tools. A credible demonstration should preserve exact
publications and the relationships it claims to carry when applications or hosting providers change.
It should also make the ongoing integration and operating work visible. An open
interface alone does not eliminate migration costs or make proprietary native
files universally editable.

## AI adoption and the information it depends on

RIBA's 2026 survey reports that 74% of architectural practices use AI in at
least some projects. Its accompanying discussion describes productivity
benefits alongside concerns about intellectual property, careers, and other
effects on professional practice. [7]

RICS's 2025 construction survey received more than 2,200 responses globally,
48% from the UK. When asked to select their top three adoption barriers,
respondents most often selected skills shortages (46%), integration with
existing systems (37%), and data quality and availability (30%). Implementation
cost (29%), unclear return on investment (28%), and privacy and security (22%)
were also concerns. These are reported perceptions. The survey covers a
different population and period from RIBA's and is not directly comparable. [8]

The resulting opportunity is to reduce the effort required to connect useful
applications to understandable information. Exact versions can anchor an
analysis to known inputs. Linked results can help people investigate how an
answer was obtained. Structured content can support queries when its fields,
units, and meaning are agreed. These are ingredients for an AI workflow;
correct answers still require appropriate methods, data quality, and review.

## Teams are beginning to build their own tools

Stantec's design-computing leader identifies AI-assisted creation of small,
specialized applications as a 2026 trend. His account describes rapid
prototyping while cautioning that improvements to individual tasks have not
yet produced an equivalent acceleration of whole projects. He also identifies
inconsistent information across design and business systems as a barrier to
deeper analysis and AI, and raises questions about data ownership. This is a
practitioner's account and outlook, rather than an industry-wide productivity
study. [9]

Broader evidence shows substantial use of AI for application development.
Anthropic's April 2025 analysis of 500,000 coding interactions found frequent
work on interfaces and web applications. Its sample concerns its own products
and excludes Team, Enterprise, and API usage from the Claude.ai portion; it
cannot establish AEC adoption or production software quality. [10]

An implication for infrastructure is that more people may be able to prototype
a useful tool while still needing help with its connection to real work.
Access control, version selection, file interpretation, error handling, and
result publication remain necessary. A common contract can make some of that
work reusable. Supported examples and maintained integrations are needed to
turn the contract into something an in-house developer can use.

## Agents need information and capabilities

Whether an agent should operate on software or data is best understood as two
related responsibilities. Data supplies the context and evidence for a task.
Software supplies the operations that can be performed on that information.
The Model Context Protocol (MCP) reflects this distinction through resources that expose context and tools
that can query systems, invoke APIs, or perform computations. MCP is one
possible integration mechanism; it does not require OCI, and OCI does not
require MCP. [19][20]

For an AEC workflow, the useful contract is an authorized input version, a
defined operation, and a result connected to its inputs. An agent might request
quantities from a supported model, invoke an engineering calculation, and
prepare a report for review. Authoring changes may need the source application's
geometry engine, constraints, or transaction API. Structured access can reduce
dependence on a particular screen layout, while interface automation remains a
possible route where better integrations are unavailable.

OCI can deliver the input and output artifacts. It does not execute the
operation, make a model understandable to an agent, or provide the agent's
authorization and execution policy. Query services, adapters, domain engines,
and access controls complete the workflow. Keeping these responsibilities
explicit allows teams to change agents and applications while retaining
inspectable project records.

## Security and responsibility across tools

Construction information crosses organizational and contractual boundaries.
NCSC's construction joint-venture guidance addresses how participating
businesses should protect sensitive information through a coordinated security
approach. Its advice places responsibility on management as well as technical
specialists. [11]

Agentic tools introduce additional questions about what software can access and
do. NCSC's August 2026 interim advice recommends constrained environments,
limited credentials, observation of activity, and appropriate human oversight.
It explicitly advises against relying on prompts alone. [12]

Access should be explained through concrete actions: an application
can read a selected version, or publish to a permitted destination. Claims about
secure automation need additional evidence about the application, its execution
environment, and operating procedures. A content fingerprint detects changed
bytes; it does not establish that a model is correct or approved. Hosting under
an organization's control does not, by itself, establish legal ownership,
permitted AI training, or compliance. These remain separate responsibilities.

## An active and changing market

Incumbents are actively adapting to AI. Autodesk's April 2026 announcements
describe granular data APIs, MCP integrations including a Revit preview, and
an expanding marketplace for applications and agent tools. Capabilities in
preview and planned integrations should be distinguished from generally
available products. [13]

Other providers also address information access and reuse. Speckle describes
model-based AI queries, visibility into their inputs, and analysis across
projects. These are the provider's product claims, not independently assessed
performance findings. Meanwhile, buildingSMART's openCDE Documents API already
defines file selection, download, and upload between compatible clients and
CDEs. [14][15]

An OCI-based proposal therefore needs to explain its particular role and how
it can fit existing workflows. OCI provides content distribution and artifact
associations; domain profiles define what the delivered files mean. The
proposed value is portable publications and linked results across independently
operated stores using a shared interface. It must be demonstrated with
maintained applications and explicit content agreements. [16][17]

Subsoil is one usable demonstration, not a required product or exclusive
gateway. ORAS is an OCI artifact tool, rather than a separate domain standard.
Other compatible clients and registries can participate. Common transport does
not require every participant to use Subsoil's model conversion, data profile,
browser, or application-launch flow. Those are implementation choices and
additional agreements, distinct from the OCI specifications. [18]

## A useful way to explain the opportunity

Begin with continuity: project information should remain useful as teams and
tools change. Show the effort involved in moving it between systems, then
introduce the growing ability to build specialized software. The foundation
becomes relevant when people see why each new application needs dependable
inputs, appropriate access, and a place for results.

Federation, version comparison, specialist applications, and learning across
projects can then serve as practical examples. A federation application selects
exact contributions and defines their alignment. A comparison application
explains changes it can identify. A focused application publishes a result
against its inputs. Portfolio analysis requires consistent measures and rights
to use the information. The delivery layer supports these activities without
supplying their domain logic.

The smallest persuasive demonstration connects two maintained applications to
an exact publication, produces a useful result linked to that input, and
transfers the supported package between independently operated stores. Assess
the time needed to connect the applications, the information preserved, and
whether another person can reproduce the handoff. The current implementation
and integration contracts are described in the [project overview](../README.md)
and [development guide](DEVELOPMENT.md).

## References

1. Institution of Civil Engineers, Infrastructure Client Group. [The Future of Common Data Environments](https://www.ice.org.uk/media/3k0h5ofz/nc-ddtg-future-of-cdes-paper.pdf). Expert discussion held November 2025.
2. Jaskula, K., Kifokeris, D., Papadonikolaki, E., and Rovas, D. [Common Data Environments in construction: State-of-the-art and challenges for practical implementation](https://discovery.ucl.ac.uk/id/eprint/10186260/1/240129_SLR_con_in_published.pdf). UCL author manuscript, 2024.
3. UK BIM Framework. [Guidance Part C: Facilitating the common data environment workflow and technical solutions](https://ukbimframework.org/wp-content/uploads/2021/02/Guidance-Part-C_Facilitating-the-common-data-environment-workflow-and-technical-solutions_Edition-1.pdf). Edition 1, September 2020.
4. AiN, ATL, Danske Arkitektvirksomheder, and SAMARK. [Open letter to Autodesk](https://aecmag.s3.amazonaws.com/Nordic-Letter-aecmag.com.pdf). September 12, 2022; reproduced by AEC Magazine.
5. r/bim. [Practitioner discussion of Autodesk cloud platform changes](https://www.reddit.com/r/bim/comments/1r8w7cm/why_are_they_doing_this_to_us/). February 2026.
6. Autodesk Community. [Autodesk Construction Cloud user level handover](https://forums.autodesk.com/t5/forma-for-construction-ideas/autodesk-construction-cloud-user-level-handover/idi-p/14064305). User-submitted feature request, March 23, 2026.
7. Royal Institute of British Architects. [RIBA AI Report 2026](https://www.riba.org/work/insights-and-resources/ai-report/).
8. Royal Institution of Chartered Surveyors. [Artificial intelligence in construction report 2025](https://www.rics.org/news-insights/artificial-intelligence-in-construction-report). Q1 2025 Global Construction Monitor responses.
9. Mullins, B., Stantec. [2026 AEC trends: AI in architecture, vibe coding, real-time analysis, and data](https://www.stantec.com/en/ideas/content/blog/2026/2026-aec-trends-part-2-ai-in-architecture-vibe-coding-real-time-analysis-data). February 19, 2026.
10. Anthropic. [Economic Index: AI's impact on software development](https://www.anthropic.com/research/impact-software-development). April 28, 2025.
11. National Cyber Security Centre and industry partners. [Joint Ventures in the Construction Sector](https://www.ncsc.gov.uk/files/Joint-ventures-in-the-Construction-Sector-guidance.pdf). 2022.
12. National Cyber Security Centre. [Managing the cyber risk of agentic AI](https://www.ncsc.gov.uk/blogs/managing-the-cyber-risk-of-agentic-ai). August 20, 2026; interim advice.
13. Autodesk Platform Services. [Building for Agentic AI: What's New in Autodesk Platform Services](https://aps.autodesk.com/blog/building-agentic-ai-whats-new-autodesk-platform-services). April 15, 2026.
14. Speckle. [Design data and intelligence platform](https://speckle.systems/). Product descriptions accessed November 4, 2026.
15. buildingSMART. [openCDE Documents API](https://github.com/buildingSMART/documents-API). Release 1.0 specification.
16. Open Container Initiative. [Distribution Specification 1.1.1](https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md).
17. Open Container Initiative. [Image Manifest Specification 1.1.1](https://github.com/opencontainers/image-spec/blob/v1.1.1/manifest.md), including artifact usage and subject associations.
18. ORAS. [Pushing and pulling OCI artifacts](https://oras.land/docs/how_to_guides/pushing_and_pulling/) and [copying artifacts between registries](https://oras.land/docs/commands/oras_cp/).
19. Model Context Protocol. [Resources](https://modelcontextprotocol.io/specification/2025-11-25/server/resources). Specification dated November 25, 2025.
20. Model Context Protocol. [Tools](https://modelcontextprotocol.io/specification/2025-11-25/server/tools). Specification dated November 25, 2025.
