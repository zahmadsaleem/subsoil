#!/usr/bin/env python3
"""Refresh the presentation text and speaker notes from docs/GOVERNANCE_PITCH.md."""

from html import escape
from pathlib import Path
import re

DECK_DIR = Path(__file__).resolve().parent
PITCH = DECK_DIR.parents[1] / "docs" / "GOVERNANCE_PITCH.md"
SLIDE_PATTERN = re.compile(
    r"^## (\d{2}) ([^\n]+)\n\n### ([^\n]+)\n\n\*\*Speaker notes\*\*\n\n"
    r"(.*?)(?=^## |\Z)",
    re.M | re.S,
)
LINK_PATTERN = re.compile(r"\[([^\]]+)\]\((https?://[^)]+)\)")


def render_notes(markdown):
    paragraphs = []
    for paragraph in markdown.strip().split("\n\n"):
        pieces = []
        offset = 0
        for match in LINK_PATTERN.finditer(paragraph):
            label, url = match.groups()
            pieces.append(escape(paragraph[offset:match.start()]))
            pieces.append(f'<a href="{escape(url)}">{escape(label)}</a>')
            offset = match.end()
        pieces.append(escape(paragraph[offset:]))
        paragraphs.append("<p>" + "".join(pieces) + "</p>")
    return "\n".join(paragraphs)


def update_section(markup, number, title, subtitle, notes):
    start, section = markup.split(f'id="slide-{number}"', 1)
    section, remainder = section.split("</section>", 1)
    section = re.sub(r'data-title="[^"]*"', lambda _: f'data-title="{title}"', section, count=1)
    fields = {
        f'<h1 id="title-{number}">': title,
        '<p class="lede">': subtitle,
        '<aside class="speaker-note" hidden>': "\n" + notes + "\n  ",
    }
    for opening, value in fields.items():
        tag = opening[1:].split()[0]
        pattern = re.escape(opening) + rf".*?(?=</{tag}>)"
        section, count = re.subn(pattern, lambda _: opening + value, section, flags=re.S)
        if count != 1:
            raise ValueError(f"Expected one {tag} field in slide {number}.")
    return start + f'id="slide-{number}"' + section + "</section>" + remainder


def main():
    index_path = DECK_DIR / "index.html"
    notes_path = DECK_DIR / "speaker-notes.html"
    index = index_path.read_text(encoding="utf-8")
    all_notes = notes_path.read_text(encoding="utf-8")
    slides = SLIDE_PATTERN.findall(PITCH.read_text(encoding="utf-8"))
    layout_numbers = re.findall(r'<section class="slide\b[^>]+id="slide-(\d+)"', index)
    if [str(int(slide[0])) for slide in slides] != layout_numbers or not slides:
        raise ValueError("The Markdown slide order must match the presentation layout.")

    articles = []
    for number, title, subtitle, notes in slides:
        ordinal = int(number)
        title = escape(title)
        notes = render_notes(notes)
        index = update_section(index, ordinal, title, escape(subtitle), notes)
        articles.append(
            f'<article id="note-{ordinal}">\n'
            f'  <p class="slide-index" aria-label="Slide {ordinal}">{number}</p>\n'
            f'  <h2><a href="index.html#slide-{ordinal}">{title}</a></h2>\n'
            f'{notes}\n'
            '</article>'
        )

    prefix, generated = all_notes.split("<!-- Notes start -->\n", 1)
    _, suffix = generated.split("<!-- Notes end -->", 1)
    index_path.write_text(index, encoding="utf-8")
    notes_path.write_text(
        prefix + "<!-- Notes start -->\n" + "\n".join(articles) + "\n<!-- Notes end -->" + suffix,
        encoding="utf-8",
    )


if __name__ == "__main__":
    main()
