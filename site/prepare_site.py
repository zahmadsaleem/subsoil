#!/usr/bin/env python3
"""Assemble and verify the static Subsoil website for GitHub Pages."""

import argparse
from collections import defaultdict
from html import escape
from html.parser import HTMLParser
from os.path import relpath
from pathlib import Path
import re
import shutil
import subprocess
import sys
from urllib.parse import unquote, urlsplit

SITE = Path(__file__).resolve().parent
ROOT = SITE.parent
GITHUB_DOCS = "https://github.com/zahmadsaleem/subsoil/blob/main/docs/"
GOVERNANCE_FILES = (
    "index.html", "deck.css", "deck.js", "speaker-notes.html", "previews/overview.png",
    "assets/project-record.png", "assets/handoff.png", "assets/own-records.png",
    "assets/release-in-hand.png",
)
PUBLISHED_PITCH_RESOURCES = (
    "assets/centralized-exchange.png", "assets/distributed-collaboration.png",
    "oci-subsoil-manifest.example.json", "subsoil-profile-config.example.json",
)
INLINE = re.compile(r"\[([^\]\n]+)\]\(([^)\n]+)\)|\[(\d+)\]|\*\*([^*\n]+)\*\*|\*([^*\n]+)\*")
BARRIERS = (
    ("Skills shortages", 46),
    ("Integration with existing systems", 37),
    ("Data quality and availability", 30),
    ("Implementation cost", 29),
    ("Unclear return on investment", 28),
    ("Lack of standards and guidance", 25),
)


def markdown_sections(markdown):
    lead, *sections = re.split(r"\n(?=## )", markdown.strip())
    result = []
    for section in sections:
        title, body = section.removeprefix("## ").split("\n", 1)
        anchor = re.sub(r"[^a-z0-9]+", "-", title.lower()).strip("-")
        result.append((anchor, title, body.strip()))
    return lead, result


def render_inline(text, cite=None):
    result = []
    offset = 0
    for match in INLINE.finditer(text):
        result.append(escape(text[offset:match.start()]))
        label, url, number, strong, emphasis = match.groups()
        if url:
            if url == "../README.md":
                url = "../index.html"
            elif url.split("#", 1)[0] in ("DEVELOPMENT.md", "ARCHITECTURE.md"):
                url = GITHUB_DOCS + url
            result.append(f'<a href="{escape(url)}">{escape(label)}</a>')
        elif number and cite:
            result.append(cite(int(number)))
        elif strong:
            result.append(f"<strong>{escape(strong)}</strong>")
        elif emphasis:
            result.append(f"<em>{escape(emphasis)}</em>")
        else:
            result.append(escape(match.group()))
        offset = match.end()
    result.append(escape(text[offset:]))
    return "".join(result)


def render_paragraphs(text, *, cite=None, css=""):
    tag = f'<p class="{css}">' if css else "<p>"
    return "\n".join(
        tag + render_inline(" ".join(paragraph.split()), cite) + "</p>"
        for paragraph in text.strip().split("\n\n")
    )


class Research:
    def __init__(self, markdown):
        body, references = markdown.strip().split("\n## References\n", 1)
        lead, self.sections = markdown_sections(body)
        _, self.intro = lead.split("\n", 1)
        self.references = {}
        for line in references.strip().splitlines():
            number, entry = line.split(". ", 1)
            self.references[int(number)] = entry
        self.citations = defaultdict(list)

    def cite(self, number):
        if number not in self.references:
            raise ValueError(f"Research citation {number} has no reference.")
        uses = self.citations[number]
        anchor = f"cite-{number}-{len(uses) + 1}"
        uses.append(anchor)
        return (
            f'<sup class="cite"><a id="{anchor}" href="#ref-{number}" '
            f'role="doc-noteref" aria-label="Source {number}">[{number}]</a></sup>'
        )

    def barriers_chart(self):
        bars = []
        for label, value in BARRIERS:
            bars.append(
                f'<li><span>{escape(label)}</span><span class="chart-track" aria-hidden="true">'
                f'<span class="chart-bar" style="--share: {value * 2}%"></span></span>'
                f'<strong>{value}%</strong></li>'
            )
        return (
            '<figure class="data-figure" aria-labelledby="barriers-title">'
            '<h3 id="barriers-title">Six most-cited barriers to AI adoption</h3>'
            '<ol class="chart">' + "\n".join(bars) + '</ol>'
            '<figcaption class="figure-note">RICS, Q1 2025. The survey received more than 2,200 responses '
            'globally. Respondents could select up to three barriers. Bar lengths use a 0–50% scale.'
            + self.cite(8) + '</figcaption></figure>'
        )

    def exchange_diagram(self):
        return (
            '<figure class="data-figure" aria-labelledby="exchange-title">'
            '<h3 id="exchange-title">One release across two CDEs</h3>'
            '<ol class="exchange-steps">'
            '<li><strong>Engineering firm</strong>Issues a version from its own CDE.</li>'
            '<li><strong>Shared interface</strong>Carries the exact version and its approval record.</li>'
            '<li><strong>Owner</strong>Keeps the release and checks what it was approved for.</li>'
            '</ol><figcaption class="figure-note">Proposed workflow. The project defines who can issue, '
            'accept, and retain each release.' + self.cite(1) + '</figcaption></figure>'
        )

    def footnotes(self):
        entries = []
        for number, entry in self.references.items():
            links = []
            for position, anchor in enumerate(self.citations[number], 1):
                links.append(
                    f'<a href="#{anchor}" role="doc-backlink" '
                    f'aria-label="Back to citation {position} for source {number}">↩{position}</a>'
                )
            entries.append(
                f'<li id="ref-{number}" tabindex="-1">{render_inline(entry, self.cite)}'
                f'<span class="backlinks">{" ".join(links)}</span></li>'
            )
        return (
            '<section class="footnotes" id="sources" aria-labelledby="sources-title">'
            '<h2 id="sources-title">Sources</h2><ol>\n' + "\n".join(entries) + '\n</ol></section>'
        )

    def render(self, template):
        template = re.sub(r"\{\{cite:(\d+)\}\}", lambda match: self.cite(int(match[1])), template)
        article = [render_paragraphs(self.intro, cite=self.cite, css="intro")]
        for anchor, title, text in self.sections:
            paragraphs = render_paragraphs(text, cite=self.cite)
            if anchor == "data-ownership-across-organizational-boundaries":
                paragraphs += "\n" + self.exchange_diagram()
            if anchor == "ai-adoption-and-the-information-it-depends-on":
                paragraphs += "\n" + self.barriers_chart()
            article.append(f'<section id="{anchor}"><h2>{escape(title)}</h2>\n{paragraphs}\n</section>')
        article.append(self.footnotes())
        contents = [
            f'<li><a href="#{anchor}">{escape(title)}</a></li>'
            for anchor, title, _ in self.sections
        ]
        contents.append('<li><a href="#sources">Sources</a></li>')
        return template.replace("<!-- contents -->", "\n".join(contents)).replace("<!-- article -->", "\n".join(article))


def render_philosophy(template, markdown):
    _, sections = markdown_sections(markdown)
    opening, *middle, conclusion = sections
    _, opening_title, opening_text = opening
    lead, story = opening_text.split("\n\n", 1)
    conclusion_anchor, conclusion_title, conclusion_text = conclusion
    contents = [
        f'<li><a href="#{anchor}">{escape(title)}</a></li>'
        for anchor, title, _ in middle + [conclusion]
    ]
    sections_html = []
    for anchor, title, body in middle:
        content = render_paragraphs(body)
        if anchor == "how-do-i-know-what-to-work-from":
            content += """
<figure class="version-comparison">
  <figcaption>Two versions in the same folder</figcaption>
  <dl>
    <div>
      <dt><span>Version 7</span> Issued for coordination</dt>
      <dd>Check a proposed change.</dd>
    </div>
    <div>
      <dt><span>Version 6</span> Issued for construction</dt>
      <dd>Use it to build on site.</dd>
    </div>
  </dl>
</figure>"""
        sections_html.append(f'<section id="{anchor}"><h2>{escape(title)}</h2>\n{content}\n</section>')
    blocks = {
        "{{opening-title}}": escape(opening_title),
        "<!-- opening-lead -->": render_paragraphs(lead),
        "<!-- opening-story -->": render_paragraphs(story),
        "<!-- contents -->": "\n".join(contents),
        "<!-- sections -->": "\n".join(sections_html),
        "{{conclusion-anchor}}": conclusion_anchor,
        "{{conclusion-title}}": escape(conclusion_title),
        "<!-- conclusion -->": render_paragraphs(conclusion_text),
    }
    for marker, markup in blocks.items():
        template = template.replace(marker, markup)
    return template


class LocalReferences(HTMLParser):
    def __init__(self, page):
        super().__init__()
        self.ids = set()
        self.links = []
        self.feed(page.read_text(encoding="utf-8"))

    def handle_starttag(self, tag, attributes):
        for attr, value in attributes:
            if attr == "id":
                if value in self.ids:
                    raise ValueError(f"Duplicate HTML id: {value}")
                self.ids.add(value)
            if attr in ("href", "src") and value:
                url = urlsplit(value)
                if not (url.scheme or url.netloc):
                    self.links.append(url)


def check_local_links(output):
    pages = {page: LocalReferences(page) for page in output.rglob("*.html")}
    for page, references in pages.items():
        for url in references.links:
            target = (page.parent / unquote(url.path)).resolve() if url.path else page
            if target.is_dir():
                target = target / "index.html"
            if not target.is_relative_to(output) or not target.is_file():
                raise ValueError(f"{page.relative_to(output)} has a missing local link: {url.geturl()}")
            if url.fragment and target in pages and unquote(url.fragment) not in pages[target].ids:
                raise ValueError(f"{page.relative_to(output)} has a missing anchor: {url.geturl()}")


def copy(source, destination):
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(source, destination)


def add_site_favicon(output):
    icon = output / "assets/subsoil-symbol.svg"
    copy(ROOT / "assets/brand/subsoil-symbol.svg", icon)
    for page in output.rglob("*.html"):
        href = Path(relpath(icon, page.parent)).as_posix()
        markup = page.read_text(encoding="utf-8")
        head, closing_tag, body = markup.partition("</head>")
        if not closing_tag:
            raise ValueError(f"Missing closing head tag in {page.relative_to(output)}")
        link = f'<link rel="icon" type="image/svg+xml" href="{href}">'
        page.write_text(f"{head.rstrip()}\n  {link}\n{closing_tag}{body}", encoding="utf-8")


def preserve_published_pitch_links(app_deck, output):
    for name in PUBLISHED_PITCH_RESOURCES:
        copy(app_deck / name, output / name)

    for name, title in (("preview.html", "Project data exchange slides"), ("speaker-notes.html", "Speaker notes")):
        target = f"{app_deck.name}/{name}"
        (output / name).write_text(
            '<!doctype html>\n<html lang="en"><head><meta charset="utf-8">\n'
            f'<title>{title} · Subsoil</title><link rel="canonical" href="{target}">\n'
            f'<script>location.replace(\'{target}\' + location.hash);</script>\n'
            f'</head><body><a href="{target}">{title}</a></body></html>\n',
            encoding="utf-8",
        )


def export_site(output):
    output = output.resolve()
    if output == ROOT or ROOT.is_relative_to(output) or output.exists() and any(output.iterdir()):
        raise ValueError("Choose a separate, empty output directory.")
    output.mkdir(parents=True, exist_ok=True)

    copy(SITE / "index.html", output / "index.html")
    copy(SITE / "assets/site.css", output / "assets/site.css")
    copy(SITE / "assets/open-cde.png", output / "assets/open-cde.png")
    copy(ROOT / "assets/brand/subsoil-logo.svg", output / "assets/subsoil-logo.svg")
    copy(ROOT / "assets/brand/subsoil-wordmark.svg", output / "assets/subsoil-wordmark.svg")

    research = Research((ROOT / "docs/CDE_AI_RESEARCH.md").read_text(encoding="utf-8"))
    research_dir = output / "research"
    research_dir.mkdir()
    template = (SITE / "research.html").read_text(encoding="utf-8")
    (research_dir / "index.html").write_text(research.render(template), encoding="utf-8")

    for name in ("philosophy.css", "philosophy-drawing-handoff.webp", "philosophy-soil-layers.png"):
        copy(SITE / "assets" / name, output / "assets" / name)
    philosophy_dir = output / "philosophy"
    philosophy_dir.mkdir()
    template = (SITE / "philosophy.html").read_text(encoding="utf-8")
    markdown = (ROOT / "docs/PHILOSOPHY.md").read_text(encoding="utf-8")
    (philosophy_dir / "index.html").write_text(render_philosophy(template, markdown), encoding="utf-8")

    app_deck = ROOT / "presentations/project-data-exchange"
    subprocess.run(
        [sys.executable, str(app_deck / "tools/prepare_site.py"), "--output", str(output / app_deck.name)],
        check=True,
    )
    preserve_published_pitch_links(app_deck, output)
    governance_deck = ROOT / "presentations/open-cde-governance"
    for name in GOVERNANCE_FILES:
        copy(governance_deck / name, output / governance_deck.name / name)

    handbook = (ROOT / "docs/handbook.html").read_text(encoding="utf-8")
    handbook_links = {
        "../presentations/project-data-exchange/index.html": "../project-data-exchange/index.html",
        "../README.md": "../index.html",
        "CDE_AI_RESEARCH.md": "../research/index.html",
        "DEVELOPMENT.md": GITHUB_DOCS + "DEVELOPMENT.md",
        "ARCHITECTURE.md": GITHUB_DOCS + "ARCHITECTURE.md",
    }
    for old, new in handbook_links.items():
        handbook = handbook.replace(f'href="{old}"', f'href="{new}"')
    handbook_dir = output / "handbook"
    handbook_dir.mkdir()
    (handbook_dir / "index.html").write_text(handbook, encoding="utf-8")

    add_site_favicon(output)
    (output / ".nojekyll").write_text("", encoding="utf-8")
    check_local_links(output)
    print(f"Exported the homepage, philosophy, research, handbook, and both presentations to {output}.")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True, help="Empty directory for the public site.")
    export_site(parser.parse_args().output)
