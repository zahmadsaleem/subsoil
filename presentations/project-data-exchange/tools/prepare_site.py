#!/usr/bin/env python3
"""Validate the complete pitch and export only its public static resources."""
import argparse
import hashlib
import html
from html.parser import HTMLParser
import json
from pathlib import Path
import re
import shutil
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parents[1]
SLIDE_COUNT = 6
ARTWORK = (
    "assets/pitch-tool-and-models.png",
    "assets/pitch-separated-files.png",
    "assets/pitch-coordination-loop.png",
    "assets/pitch-ifc-model.png",
)
PUBLIC_FILES = (
    "index.html", "speaker-notes.html", "preview.html", "sources.json",
    "previews/overview.png", *ARTWORK,
    *(f"previews/slide-{number:02}.png" for number in range(1, SLIDE_COUNT + 1)),
)
TOOL_FILES = (
    "tools/prepare_site.py", "tools/sync.py", "tools/serve.py",
    "tools/verification.json",
)
RETIRED_FILES = (
    "oci-ifcx-slide.json", "navigation-check.js", "preview.png",
    "project-verification.json", "sync-verification.json", "verification.json",
    "sync.py", "serve.py",
)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def retired_files(root):
    return [p for p in root.glob("new-slide-*") if p.is_file()] + [
        root / name for name in RETIRED_FILES if (root / name).exists()
    ] + [
        path for path in (root / "previews").glob("slide-*.png")
        if f"previews/{path.name}" not in PUBLIC_FILES
    ]


class References(HTMLParser):
    def __init__(self):
        super().__init__()
        self.links = []
        self.ids = []

    def handle_starttag(self, tag, attributes):
        for key, value in attributes:
            if key == "id":
                self.ids.append(value)
            if key in ("href", "src") and value:
                self.links.append(value)


def text_content(markup):
    return " ".join(html.unescape(re.sub(r"<[^>]+>", "", markup)).split())


def read_pitch(path):
    sections = re.findall(
        r"^## (\d{2}) ([^\n]+)\n\n### ([^\n]+)\n\n\*\*Speaker notes\*\*\n\n"
        r"(.*?)(?=^## |\Z)",
        path.read_text(), re.M | re.S,
    )
    if [number for number, *_ in sections] != [f"{n:02}" for n in range(1, SLIDE_COUNT + 1)]:
        raise ValueError(f"{path.name} must contain {SLIDE_COUNT} numbered slides with speaker notes.")
    return [(title, subtitle, notes.strip()) for _, title, subtitle, notes in sections]


def notes_html(markdown):
    def link(match):
        label, target = match.groups()
        if target.startswith("ARCHITECTURE.md"):
            target = "https://github.com/zahmadsaleem/subsoil/blob/main/docs/" + target
        return f'<a href="{html.escape(target)}">{html.escape(label)}</a>'

    parts = re.split(r"^```text\n(.*?)\n^```$", markdown, flags=re.M | re.S)
    rendered = []
    for index, part in enumerate(parts):
        if index % 2:
            rendered.append(f"<pre><code>{html.escape(part)}</code></pre>")
            continue
        for para in part.strip().split("\n\n"):
            if not para:
                continue
            links = re.compile(r"\[([^\]]+)\]\(([^)]+)\)")
            pieces = []
            last = 0
            for match in links.finditer(para):
                pieces.extend((html.escape(para[last:match.start()]), link(match)))
                last = match.end()
            pieces.append(html.escape(para[last:]))
            rendered.append("<p>" + "".join(pieces) + "</p>")
    return "\n".join(rendered)


def update_between(path, label, body):
    start = f"<!-- {label} start -->\n"
    end = f"<!-- {label} end -->"
    before, generated = path.read_text().split(start, 1)
    _, after = generated.split(end, 1)
    path.write_text(before + start + body + "\n" + end + after)


def refresh_copy(root):
    slides = read_pitch(root.parents[1] / "docs/PITCH.md")
    index_path = root / "index.html"
    index = index_path.read_text()
    articles, thumbnails = [], []
    for n, (title, subtitle, notes) in enumerate(slides, 1):
        title, subtitle = html.escape(title), html.escape(subtitle)
        notes_markup = notes_html(notes)

        def replace_slide(match):
            section = match[0]
            section = re.sub(r'data-title="[^"]*"', lambda _: f'data-title="{title}"', section)
            section = re.sub(r'<h1 id="title-\d+">.*?</h1>', lambda _: f'<h1 id="title-{n}">{title}</h1>', section, flags=re.S)
            section = re.sub(r'<p class="lede">.*?</p>', lambda _: f'<p class="lede">{subtitle}</p>', section, flags=re.S)
            section = re.sub(r'<aside class="speaker-note" hidden>.*?</aside>',
                             lambda _: f'<aside class="speaker-note" hidden>\n{notes_markup}\n  </aside>', section, flags=re.S)
            if 'id="copy-prompt"' in section:
                first_line = re.search(r"^```text\n([^\n]+)", notes, re.M)[1]
                excerpt = re.sub(r"\[[^\]]+\]", lambda match: f"<mark>{match[0]}</mark>", html.escape(first_line))
                section = re.sub(r'<p class="prompt-excerpt">.*?</p>',
                                 lambda _: f'<p class="prompt-excerpt">{excerpt}</p>', section, flags=re.S)
            return section

        index, replaced = re.subn(rf'<section class="slide\b[^>]+id="slide-{n}".*?</section>', replace_slide, index, flags=re.S)
        if replaced != 1:
            raise ValueError(f"Missing or duplicate layout for slide {n}.")
        articles.append(f'<article id="note-{n}">\n<p class="slide-index">{n:02}</p>\n<h2>{title}</h2>\n{notes_markup}\n</article>')
        thumbnails.append(f'<a href="index.html#slide-{n}"><img src="previews/slide-{n:02}.png" '
                          f'alt="Slide {n} · {title}" width="1600" height="900"></a>')
    index_path.write_text(index)
    update_between(root / "speaker-notes.html", "Notes", "\n".join(articles))
    update_between(root / "preview.html", "Previews", "\n".join(thumbnails))


def validate_bundle(root):
    root = root.resolve()
    stale = retired_files(root)
    if stale:
        raise ValueError(f"Retired drafting files remain: {[p.name for p in stale]}")
    for name in PUBLIC_FILES:
        path = root / name
        if not path.is_file() or path.is_symlink():
            raise ValueError(f"Missing public file or unsupported link: {name}")
    index = (root / "index.html").read_text()
    notes = (root / "speaker-notes.html").read_text()
    sections = re.findall(r'<section class="slide\b.*?</section>', index, re.S)
    articles = re.findall(r'<article\b.*?</article>', notes, re.S)
    titles = [html.unescape(re.search(r'data-title="([^"]+)"', s)[1]) for s in sections]
    if len(sections) != SLIDE_COUNT or len(articles) != SLIDE_COUNT:
        raise ValueError(f"The complete {SLIDE_COUNT}-slide order and notes are required.")
    pitch_path = root.parents[1] / "docs/PITCH.md"
    slides = read_pitch(pitch_path) if pitch_path.is_file() else None
    for number, (section, article) in enumerate(zip(sections, articles), 1):
        assert f'id="slide-{number}"' in section
        assert f'id="note-{number}"' in article
        assert f'<p class="slide-index" aria-label="Slide {number}">{number:02}</p>' in section
        embedded = re.search(r'<aside class="speaker-note" hidden>(.*?)</aside>', section, re.S)[1]
        standalone = article.split("</h2>", 1)[1].removesuffix("</article>")
        assert embedded.strip() == standalone.strip()
        assert len(text_content(embedded)) > 100
        heading = text_content(re.search(r'<h1 id="title-\d+">(.*?)</h1>', section, re.S)[1])
        assert heading == text_content(re.search(r"<h2>(.*?)</h2>", article, re.S)[1]) == titles[number - 1]
        if slides:
            title, subtitle, copy = slides[number - 1]
            assert heading == title
            assert text_content(re.search(r'<p class="lede">(.*?)</p>', section, re.S)[1]) == subtitle
            assert embedded.strip() == notes_html(copy)
    assert "<title>Project data exchange</title>" in index
    assert "<title>Speaker notes · Project data exchange</title>" in notes
    assert 'href="speaker-notes.html"' in index and 'href="index.html"' in notes
    assert 'id="copy-prompt"' in sections[-1] and "<pre><code>" in sections[-1]
    links = set()
    for name in ("index.html", "speaker-notes.html", "preview.html"):
        markup = (root / name).read_text()
        assert not re.search(r"/Users/|file://|library-file:", markup)
        references = References()
        references.feed(markup)
        assert len(references.ids) == len(set(references.ids)), f"Duplicate IDs in {name}"
        for target in references.links:
            parsed = urlsplit(target)
            if parsed.scheme or parsed.netloc or not parsed.path:
                continue
            local = unquote(parsed.path)
            assert not local.startswith("/") and ".." not in Path(local).parts
            assert local in PUBLIC_FILES, f"Unpublished relative resource: {target}"
            links.add(local)
    sources = json.loads((root / "sources.json").read_text())
    assert sources["presented_slides"] == SLIDE_COUNT
    assert sources["audience"] == "AEC teams, owners, tool builders and data practitioners"
    assert {item["file"] for item in sources["illustrations"]} == set(ARTWORK)
    for name in PUBLIC_FILES:
        if name.endswith(".png"):
            with (root / name).open("rb") as image:
                header = image.read(26)
            assert header.startswith(b"\x89PNG\r\n\x1a\n")
            if name in ARTWORK:
                assert header[25] in (4, 6), f"Expected PNG transparency in {name}."
    return {
        "slides": SLIDE_COUNT, "notes": SLIDE_COUNT, "slideTitles": titles,
        "presenter": "Ahmad Saleem Z", "notesMatch": True,
        "relativeLinksResolve": sorted(links), "legacyDraftFiles": 0,
        "publicFiles": {name: digest(root / name) for name in PUBLIC_FILES},
    }


def export_site(root, output):
    report = validate_bundle(root)
    output = output.resolve()
    if output == root.resolve() or root.resolve().is_relative_to(output):
        raise ValueError("Choose a separate export directory.")
    if output.exists() and any(output.iterdir()):
        raise ValueError("The export directory must be empty.")
    output.mkdir(parents=True, exist_ok=True)
    for name in PUBLIC_FILES:
        target = output / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(root / name, target)
    (output / ".nojekyll").write_text("")
    actual = {str(p.relative_to(output)) for p in output.rglob("*") if p.is_file()}
    assert actual == set(PUBLIC_FILES) | {".nojekyll"}
    assert all(digest(output / name) == value for name, value in report["publicFiles"].items())
    return report


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--refresh-copy", action="store_true", help="Refresh slide copy and notes from docs/PITCH.md.")
    parser.add_argument("--output", type=Path, help="Export the public Pages artifact to an empty directory.")
    parser.add_argument("--report", type=Path, help="Save portable validation metadata.")
    args = parser.parse_args()
    if args.refresh_copy:
        refresh_copy(ROOT)
    result = export_site(ROOT, args.output) if args.output else validate_bundle(ROOT)
    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({key: value for key, value in result.items() if key != "publicFiles"}, ensure_ascii=False))
