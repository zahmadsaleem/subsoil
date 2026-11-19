# Philosophy

## “I already shared it.”

You send someone a link to a project file. They get a sign-in screen. You
request access for them, then download a copy and send it while they wait. A
week later, someone asks which copy the team reviewed. You open the folder,
then the email thread, then the meeting notes. You remember settling this
already.

On another project, the client needs the same report uploaded to their
system. Their form asks for information you entered when you issued it. You
fill it in again. Later, someone new joins the project and asks where to find
the versions they can work from. You explain which folder to use, which links
still work, and who to ask when the two systems disagree.

By the time someone says they’re sick of the CDE, there’s usually a whole
afternoon behind that sentence. The client needs a record of what was issued.
The contractor needs to know what they can build from. The designer needs
space to work on something before sharing it. Each of those needs makes
sense. So why do the people doing the work spend so much time making sure
that what everyone knows about a file travels with the file itself?

On one metro extension, teams used four platforms and email side by side.
A reply could arrive in one system and have to be copied into another to
keep the record complete. In a construction survey in Japan, the most
frequently reported difficulty with sharing information was that the tools
and methods varied from one project to the next. [Metro case study](https://www.mdpi.com/2412-3811/10/10/266) · [Construction survey](https://prtimes.jp/main/html/rd/p/000000042.000105632.html)

## Open it in the tool you already use

“Listen wherever you get your podcasts.” You hear that and open the app you
already use. Someone else can find the same episode in a different app. The
producer can publish the episode and its description through a feed that
both apps understand. [How podcast feeds reach listeners](https://podcasters.apple.com/support/5108-how-apple-podcasts-distributes-your-shows-to-listeners)

Sharing project work should feel that straightforward. When someone gives
you access to a file, you should be able to open it in a compatible tool
you already use.

## How do I know what to work from?

A coordination team might need the newest drawing to check a proposed
change. The site team might need an earlier drawing that’s still the one
issued for construction. Both teams can open the same folder and make
different, correct choices.

The version tells you which file you have. What it was issued for, which
part of the work it covers, and whether it still applies help you decide
whether to use it. Sharing should give the next person enough context to
decide what they can rely on for the work in front of them.

## The meaning should travel with the work

When a team brings an issued drawing into its own system, the record should
carry both the version and what the issuer said it could be used for. Someone
who opens it years later should still be able to see what it was issued for
and understand the decisions the team made at the time.

## A shared way to publish a version

For a version to travel between tools, those tools need a common way to
handle what was published. The Open Container Initiative (OCI) defines how
to package files and how to publish and retrieve an exact version.
[OCI packaging specification](https://github.com/opencontainers/image-spec/blob/v1.1.1/manifest.md) · [OCI distribution specification](https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md)

Helm, a software package manager, already uses OCI to distribute versioned
packages. [Helm documentation](https://docs.helm.sh/docs/v3/topics/registries/)

A design reviewer needs the model version issued for review. Someone
ordering materials needs a table of quantities and a link to the model
version it came from. A facilities team investigating an overheating room
might need raw sensor readings and a record of where and when they were
taken. In each case, the team can publish the files or data with the details
the next person needs, so a compatible tool can retrieve them together.

## Use this tool on this version

Ask an AI assistant what’s on your calendar on Thursday. If it’s connected
to your calendar through the Model Context Protocol (MCP), the assistant’s
app has a standard way to tell the calendar tool which day to look up.
[How MCP calls a tool](https://modelcontextprotocol.io/docs/learn/architecture#understanding-the-tool-execution-request)

Subsoil’s handoff applies the same idea when a person chooses a tool. You
pick a published model version and choose **Open with…**. The project
browser tells the app where to find the exact version you selected. The
app uses its own access to retrieve it.
[How the handoff works](ARCHITECTURE.md#open-with-and-publish)

## Why Subsoil

The tools people work in are the topsoil. That’s where someone draws a
detail, checks a model, or pulls up a drawing on site. Deeper down are
business systems such as ERP, which handle purchasing, budgets, and payments.

Subsoil names the layer between them. Teams can publish project files in
systems they control, give others access, and use different tools to work
from the same published versions. The results can become part of the
project record too. A review report can go back with a link to the exact
model version the reviewer checked.

Projects change hands over time. The people who take over a building
should be able to pick up the records they receive and put them to work
in the tools they use then.
