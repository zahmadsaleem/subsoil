# The cost of fragmented information

Teams in architecture, engineering, and construction can send a drawing and
still have to answer which revision arrived, who approved it, and what the
receiving team can use it for. Those questions explain some of the work that
continues after a project introduces a common data environment (CDE).

## Scope and purpose

This research asks where project information becomes harder to find, interpret,
or rely on as it moves between systems and organizations. It follows drawings,
models, and the records of decisions about them through day-to-day delivery,
handover to operations, and reuse by specialist applications, including AI tools.
The aim is to identify the failures that an exchange needs to address and the
work that still belongs to project teams.

Case studies and interviews describe how teams work. Surveys provide figures
for the people and firms who responded. Practitioner posts illustrate specific
failures and workarounds, guidance explains how responsibilities and status
are meant to work, and prototypes show what was tested on particular tasks.
The site and handover examples come mainly from delivery and information
management roles. The survey findings apply to their respective samples.

## Sharing files and recording decisions

On a metro extension, document controllers moved submissions and replies by
hand between CDEs and other tools. Some subcontractors replied by email, and
those replies were hard to trace without a consistent archive. The 2025 case
study documents delays and identifies a risk that teams could work from an
outdated version. [1]

The problems recur across projects. In a 2025 survey in Japan, 47.2% of 999
respondents to an information-sharing question cited differences in methods
and tools across projects. Another 21.9% cited difficulty tracing changes
and identifying the latest version. Roughly seven in ten respondents asked
about CDE use reported having one in place, and about a quarter reported
using one effectively. Among participants asked why adoption or effective use
was difficult, learning effort (44.2%), complexity (31.8%), and setup and
operating costs (30.8%) were the most cited reasons. The survey publisher,
Applied Technology, helps firms set up and use CDEs. [2]

The learning curve can change where the official record ends up. In a LinkedIn
discussion started by BIM manager Ivaylo Ignatov, a design manager recalled
teams emailing drawings because introducing the CDE on site took too much
effort. Another participant described a project that kept formal replies in
the CDE with client backing and regular sessions to help users. [24]

Keeping a file in the CDE leaves a second question about the decision attached
to it. In a write-up of 20 interviews with BIM and information managers and
digital leads, BIM Corner describes projects using work-in-progress, shared,
and published folders without clear rules for moving a file between them.
Someone looking for an approved drawing then has to establish whether its
location or modification date reflects an actual decision and where that
decision was recorded. [6][25]

The work can continue after the information arrives. In a 2026 survey by the
[ACEC Research Institute](https://www.acec.org/research-institute/) and
Bentley Systems, 19% of respondents said they often or very often had to
substantially rework data received from clients, partners, or contractors.
The survey reached 251 ACEC member firms, about 10.5% of those invited. [3]

## Collaboration across several environments

Teams sometimes use several systems because each does a different job.
Jaskula and colleagues heard this in interviews with 15 experts in building
information modelling (BIM). Their study, which also reviewed 46 publications,
found that using several CDEs can make it harder to trace who is responsible
for a record and decide whether it can be relied on. [4]

The [Infrastructure Client Group](https://en.wikipedia.org/wiki/Infrastructure_Client_Group),
a forum for UK infrastructure clients, discussed what happens when the client
requires everyone to use one platform. It can make the project easier for the
client to oversee while leaving delivery partners to duplicate records from
their own systems. The group also found that using several platforms takes
more work to connect the tools and agree who is responsible for transfers. [5]

The [UK BIM Framework](https://wearenima.im/new-imi-framework/) describes how
the parties can follow one project workflow across different tools. Its
example follows a drawing between two CDEs. One system stores a
classification in a field, and the other uses folders. The drawing can
arrive while someone still has to work out where to put it. The guidance
also describes how to carry a revision and status with a drawing so the next
team knows whether it can use it for review or for construction. [6]

One way to **integrate** those two CDEs is to build a connector that maps the
field to the right folder. If both tools implement a common specification for
exchanging drawings and classifications, a third tool can take part by
implementing the same specification. The UK's
[BIM Interoperability Expert Group](https://www.cdbb.cam.ac.uk/AboutCDBB/WorkingGroups/bim-interoperability-expert-group/support-bim-interoperability-report)
calls the ability to exchange and use information **interoperability**. [7]

For a model exchange, an open format such as
[Industry Foundation Classes (IFC)](https://technical.buildingsmart.org/standards/ifc/)
gives tools a common structure for the model. An open exchange protocol such
as buildingSMART's [openCDE Documents API](https://github.com/buildingSMART/documents-API)
specifies how compatible tools can select and transfer files. [8][9][10]

The receiving tool also needs to interpret the information as the sender
intended. The
[European Interoperability Framework](https://interoperable-europe.ec.europa.eu/collection/iopeu-monitoring/3-interoperability-layers)
calls this semantic interoperability. A shared data dictionary such as the
[buildingSMART Data Dictionary](https://technical.buildingsmart.org/services/bsdd/data-structure/)
gives both sides definitions for classifications and properties. Where the
exchange needs to preserve relationships, an ontology can describe, for
example, which component an inspection result refers to. [5][8][11]

Researchers have tested these exchanges on specific project tasks. Petri and
colleagues used a common interface to share IFC models for coordination in a
UK project trial. Werbrouck and colleagues tested a prototype that gathered
model-related issues held in different systems so that tools could read them
through the
[BIM Collaboration Format (BCF) API](https://technical.buildingsmart.org/standards/bcf/).
[12][13]

## Keeping records usable after handover

At handover, the next system may belong to the team that operates the asset.
Jaskula's interviewees described moving records by hand from project CDEs
into facilities management systems. Contributors to the BIM Interoperability
Expert Group raised a longer-term concern about relying on proprietary
formats to keep records usable over the life of an asset. [4][7]

Usable copies matter when access and software change. In an account from
practice, asset-management specialist Lucas Cusack of Glider Technology
describes operations teams losing access to delivery CDEs, then having to
recover basic asset information from incomplete or incompatible files. A
2026 Autodesk forum post illustrates a different failure: a practitioner
reported manually repairing IFC and CAD links after software and path
changes. [26][27]

For an owner receiving records from several project systems, the
Infrastructure Client Group recommends handing over structured data along with
any mappings that explain how the records fit together. The group also
recommends agreeing who acts if a transfer fails and keeping consistent
permissions and audit records across systems. [5]

Access can also be hard to delegate correctly. In one Reddit discussion, a
practitioner reported wanting to let colleagues edit files without also
letting them delete them. An Autodesk Community feature request describes a
different access transition when someone leaves a project and a replacement
needs the prior reviews, issues, and responsibilities. [28][29]

For construction joint ventures, the UK's
[National Cyber Security Centre](https://www.ncsc.gov.uk/) (NCSC) and industry
partners recommend a shared security plan with responsibility assigned at board
level. [19]

## Reported AI use and barriers

Connecting new tools to existing records also came up in the
[Royal Institution of Chartered Surveyors](https://www.rics.org/) (RICS)
survey on AI in construction. Respondents most often cited skills shortages
as a barrier, followed by integration with existing systems and data quality
and availability. The Q1 2025 survey received more than 2,200 responses, 48%
from the UK. About 45% of respondents reported no AI implementation in their
organization and 34% reported early pilots. [15]

The [Royal Institute of British Architects](https://www.riba.org/) (RIBA)
surveyed architectural practices about how they use AI in their work. In its
2026 survey, 74% reported using AI in at least some projects, for activities
ranging from early design visualisation to project management. [14]

## Using AI to work with building models

On LinkedIn, Jaime Alonso Candau demonstrated a report in Claude that works
directly with a Revit model. Clicking a bar in the report selects the
corresponding walls, and editing a cell changes a model property. In a Reddit
discussion, a practitioner described using AI to build a tool for a repeated
task. They used Gemini to turn an existing script into a button that splits
ductwork into sections for ordering. [22][21]

Creating a new model component brings a different set of checks. On the Fetch
blog, Kyle Bruxvoort described using Claude and Revit to build an adjustable
cabinet from a product sheet. One early version faced the wrong way and
another had overlapping panels. He revised the instructions and tools available
to Claude and stepped in when Revit displayed a dialog during the final test.
[23]

An evaluation of 31 BIM scripting tasks measured some of that correction work.
The comparison with manual scripting found that AI-assisted work took 46% less
time and required 32% more rounds of correction. Results were less reliable
for whole systems than for individual components. Other researchers used a
rule checker to flag problems in generated models, and a study of IFC queries
found that some questions were hard to answer from the model data. [16][17][18]

For AI agents that can take actions, the NCSC's interim advice recommends
matching safeguards to the tool's level of autonomy, limiting its credentials
and access, and making its actions visible to the people responsible for
oversight. [20]

## Conclusion

The cost shows up before and after transfer. People copy submissions between
tools, re-enter classifications, or ask which revision they can use. The metro
case and the practitioner accounts show how approval evidence can end up in a
different place from the drawing. The surveys describe the burden of learning
and operating the tools as well as reworking the records they carry.
[1][2][3][6][24][25]

At handover, an owner needs the version, permitted use, and acceptance history
in a form the operations team can use. A link can lose its value when an account
closes or a path changes. Retaining files helps preserve the record, and
the information that explains those files can require separate mapping and
transfer. [4][5][6][7][26][27][28]

The same problem affects an application that reads or changes a model. The
querying study and cabinet experiment show how results depend on the data and
operations available to the tool. The scripting study also shows that a quicker
operation can take more rounds of correction. Reviewing the result therefore
depends on knowing which input and operation produced it. [16][17][20][23]

The [open CDE exchange proposal](PROPOSAL.md) sets out how Subsoil would address
these failures and how two organizations could test the approach on one handoff.

## References

1. Da Silva, S., and Boton, C. [Digital Integration in Construction: A Case Study on Common Data Environment Implementation for a Metro Line Project](https://www.mdpi.com/2412-3811/10/10/266). *Infrastructures*, 10(10), 266, 2025.
2. Applied Technology. [Survey on information sharing and data management on construction sites](https://prtimes.jp/main/html/rd/p/000000042.000105632.html). Online survey in Japan, November 2025.
3. ACEC Research Institute and Bentley Systems. [From Project Data to Infrastructure Intelligence](https://www.acec.org/resource/from-project-data-to-infrastructure-intelligence-connected-datas-role-in-engineering-the-built-environment/). November 2026. [Institute release with survey methods](https://www.hpac.com/industry-perspectives/article/55420732/engineering-firms-still-not-using-all-the-data-they-collect).
4. Jaskula, K., Kifokeris, D., Papadonikolaki, E., and Rovas, D. [Common Data Environments in construction: State-of-the-art and challenges for practical implementation](https://discovery.ucl.ac.uk/id/eprint/10186260/1/240129_SLR_con_in_published.pdf). UCL author manuscript, 2024.
5. Institution of Civil Engineers, Infrastructure Client Group. [The Future of Common Data Environments](https://www.ice.org.uk/media/3k0h5ofz/nc-ddtg-future-of-cdes-paper.pdf). Expert discussion held November 2025.
6. UK BIM Framework. [Guidance Part C: Facilitating the common data environment workflow and technical solutions](https://ukbimframework.org/wp-content/uploads/2021/02/Guidance-Part-C_Facilitating-the-common-data-environment-workflow-and-technical-solutions_Edition-1.pdf). Edition 1, September 2020.
7. BIM Interoperability Expert Group. [BIM Interoperability Expert Group Report](https://www.cdbb.cam.ac.uk/files/cih_bim_interoperability_expert_group_report_april_2020_final_wm_removed.pdf). Centre for Digital Built Britain, March 2020.
8. European Commission. [European Interoperability Framework: Interoperability layers](https://interoperable-europe.ec.europa.eu/collection/iopeu-monitoring/3-interoperability-layers).
9. buildingSMART. [Industry Foundation Classes (IFC)](https://technical.buildingsmart.org/standards/ifc/).
10. buildingSMART. [openCDE Documents API](https://github.com/buildingSMART/documents-API). Release 1.0 specification.
11. buildingSMART. [Data structure of the buildingSMART Data Dictionary](https://technical.buildingsmart.org/services/bsdd/data-structure/).
12. Petri, I., Beach, T., Rana, O. F., and Rezgui, Y. [Coordinating multi-site construction projects using federated clouds](https://orca.cardiff.ac.uk/id/eprint/103921/). *Automation in Construction*, 83, 273–284, 2017.
13. Werbrouck, J., Schulz, O., Oraskari, J., Mannens, E., Pauwels, P., and Beetz, J. [A generic framework for federated CDEs applied to Issue Management](https://research.tue.nl/en/publications/a-generic-framework-for-federated-cdes-applied-to-issue-managemen/). *Advanced Engineering Informatics*, 58, 102136, 2023.
14. Royal Institute of British Architects. [RIBA AI Report 2026](https://www.riba.org/work/insights-and-resources/ai-report/).
15. Royal Institution of Chartered Surveyors. [Artificial intelligence in construction report 2025](https://www.rics.org/news-insights/artificial-intelligence-in-construction-report). Q1 2025 Global Construction Monitor responses.
16. Alwashah, Z., Xiao, B., Liu, H., Mueller, S. T., and Shao, X. [Reliable LLM-driven BIM automation through capability-based multi-dimensional evaluation](https://doi.org/10.1016/j.autcon.2026.106910). *Automation in Construction*, 187, 106910, 2026.
17. Ibba, A., Alonso, R., and Reforgiato Recupero, D. [ASK-BIM: A knowledge graph-powered AI system for natural language querying of BIM models](https://doi.org/10.1016/j.datak.2026.102581). *Data & Knowledge Engineering*, 164, 102581, 2026.
18. Du, C., Esser, S., Nousias, S., and Borrmann, A. [Text2BIM: Generating Building Models Using a Large Language Model-Based Multiagent Framework](https://doi.org/10.1061/JCCEE5.CPENG-6386). *Journal of Computing in Civil Engineering*, 2026. [Author preprint](https://arxiv.org/abs/2408.08054).
19. National Cyber Security Centre and industry partners. [Joint Ventures in the Construction Sector](https://www.ncsc.gov.uk/files/Joint-ventures-in-the-Construction-Sector-guidance.pdf). 2022.
20. National Cyber Security Centre. [Managing the cyber risk of agentic AI](https://www.ncsc.gov.uk/blogs/managing-the-cyber-risk-of-agentic-ai). August 2026, interim advice.
21. r/bim. [Practitioner discussion of AI tools in everyday BIM work](https://www.reddit.com/r/bim/comments/1v79aml/have_the_attempted_ai_tools_changed_the_daytoday/). July 2026.
22. Alonso Candau, J. [Interactive Revit report in Claude](https://www.linkedin.com/posts/jaimealonsocandau_revit-claude-ai-activity-7493230485989548032-YIG3). LinkedIn demonstration, 2026.
23. Bruxvoort, K. [I onboarded Claude as a Revit family creator](https://blog.fetchbim.com/i-onboarded-claude-as-a-revit-family-creator). Fetch blog, May 2026.
24. Ignatov, I. [Practitioner discussion of CDE use on site](https://www.linkedin.com/posts/ivaylo-ignatov_stop-designing-cdes-for-bim-managers-if-activity-7346436631236636672-r5Qr). LinkedIn post and comments, 2025.
25. BIM Corner. [You Bought a CDE. Your Team Is Using It Like a Network Drive.](https://bimcorner.com/you-bought-a-cde-your-team-is-using-it-like-a-network-drive/). Practitioner interview write-up from a BIM training provider, September 2026.
26. Cusack, L. [The dirty truth about BIM handovers: why compliance doesn’t equal usability](https://digitalconstructionplus.com/the-dirty-truth-about-bim-handovers-why-compliance-doesnt-equal-usability/). Glider Technology practitioner account, November 2025.
27. Autodesk Community. [Report of broken IFC and CAD links after connector changes](https://forums.autodesk.com/t5/forma-for-construction-forum/rebranding-fatigue-broken-ifc-links-and-path-incompatibility-in/td-p/14093200). User report, April 2026.
28. Autodesk Community. [Autodesk Construction Cloud user level handover](https://forums.autodesk.com/t5/forma-for-construction-ideas/autodesk-construction-cloud-user-level-handover/idi-p/14064305). User-submitted feature request, March 2026.
29. r/bim. [Practitioner discussion of permissions in Autodesk Construction Cloud and SharePoint](https://www.reddit.com/r/bim/comments/1qt33p7/acc_autodesk_vs_sharepoint_microsoft/). February 2026.
