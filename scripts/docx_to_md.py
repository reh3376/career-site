#!/usr/bin/env python3
"""Convert a prose or technical .docx into corpus-ready Markdown.

`resume_to_md.py` already does .docx to .md, but everything specific to
it is specific to résumés: employment-date corrections, promoting a bold
upper-case line to a section heading, holding a competencies grid apart
so it does not collapse into a paragraph. Running it over a white paper
or a study guide would rewrite dates that are not employment dates and
turn emphasis into headings.

What a long technical document needs instead is the three things pandoc
cannot express in GFM and therefore emits as raw HTML:

  callout boxes   Word shades a single-cell table to make a sidebar.
                  Pandoc has nowhere to put it, so 28 of them arrived as
                  <table><colgroup>...</table> in the U.S. spirits study
                  guide. Raw HTML in the corpus is worse than useless:
                  the chunker keeps the tags, the embedder embeds them,
                  and a retrieved chunk hands the judge angle brackets.

  figures         <figure><img src="media/image1.png"><figcaption>. The
                  media is not extracted, so the img is a reference to a
                  file that does not exist. The caption, however, is real
                  prose that names what the figure shows, and is worth
                  keeping.

  escapes         Pandoc escapes brackets it cannot prove are safe, so a
                  citation reads \\[10\\] and a checklist box reads \\[ \\].

Everything else pandoc gets right, including tables: the study guide's
196 lines of GFM table came through intact. So this is deliberately not
a converter. It is pandoc plus the repairs, and it fails rather than
writing a file that still contains HTML, because the failure this whole
area keeps producing is a conversion that reports success.

Usage:
  python3 scripts/docx_to_md.py docs/personal/corpus/some-doc.docx
  python3 scripts/docx_to_md.py --dry-run docs/personal/corpus/*.docx
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from resume_to_md import merge_bold_runs  # noqa: E402

# A single-cell table: one <col>, one row, one cell. Word's shaded
# sidebar. More than one column is a real table and pandoc would have
# written GFM for it, so anything reaching here with two columns is
# something this script has not seen and should not guess at.
# Both openers carry attributes when the author set a width, which the
# study guide did not and the UCTS fixture does. A pattern anchored on
# the bare `<table>` matched every callout in the one real document
# tested and none in the fixture built to stand in for it.
TABLE_RE = re.compile(r"<table\b[^>]*>.*?</table>", re.S)
FIGURE_RE = re.compile(r"<figure\b[^>]*>.*?</figure>", re.S)
# A tag, and not every less-than sign. The structural-decline paper
# writes "<2%" in prose, and a pattern of `<[^>]+>` reads that plus the
# next `>` on the line as a tag and fails a conversion that is correct.
# A tag opens with a letter or a slash and a letter.
TAG_RE = re.compile(r"</?[A-Za-z][A-Za-z0-9-]*(?:\s[^<>]*?)?/?>")
PARA_RE = re.compile(r"<p>(.*?)</p>", re.S)


def strip_tags(html: str) -> str:
    text = TAG_RE.sub("", html)
    text = (
        text.replace("&amp;", "&")
        .replace("&lt;", "<")
        .replace("&gt;", ">")
        .replace("&quot;", '"')
        .replace("&#39;", "'")
        .replace("&nbsp;", " ")
    )
    return " ".join(text.split())


def callouts_to_blockquotes(text: str) -> tuple[str, int, list[str]]:
    """Turn Word's shaded single-cell tables into Markdown blockquotes.

    The cell holds one or more <p>. The first is nearly always the label
    the author bolded, so it leads the quote and the rest follow as
    paragraphs, which is the shape a reader of the .docx saw.
    """
    problems: list[str] = []
    converted = 0

    def one(m: re.Match[str]) -> str:
        nonlocal converted
        block = m.group(0)
        cols = len(re.findall(r"<col ", block))
        cells = re.findall(r"<t[hd]\b[^>]*>(.*?)</t[hd]>", block, re.S)
        cells = [c for c in cells if strip_tags(c)]
        if cols > 1 or len(cells) != 1:
            problems.append(
                f"table with {cols} columns and {len(cells)} non-empty cells "
                f"is not a callout; left as HTML"
            )
            return block
        paras = [strip_tags(p) for p in PARA_RE.findall(cells[0])]
        paras = [p for p in paras if p]
        if not paras:
            paras = [strip_tags(cells[0])]
        lines = [f"> **{paras[0]}**"] if len(paras) > 1 else [f"> {paras[0]}"]
        for p in paras[1:]:
            lines.append(">")
            lines.append(f"> {p}")
        converted += 1
        return "\n".join(lines)

    return TABLE_RE.sub(one, text), converted, problems


# An image reference, in any of the three shapes a pandoc has produced
# for the same .docx: wrapped in a <figure> with a <figcaption>, bare as
# an HTML <img>, or as Markdown's own image syntax. All three point into
# a media/ directory that is never extracted.
IMG_TAG_RE = re.compile(r"<img\b[^>]*/?>")
MD_IMG_RE = re.compile(r"!\[[^\]]*\]\([^)]*\)")


def figures_to_captions(text: str) -> tuple[str, int, int]:
    """Keep the caption, drop the image reference.

    The media directory is not extracted, so every image points at a
    file that is not there. The caption names what the figure showed,
    which is the part a retrieval system can use.

    Whether there is a caption to keep depends on the pandoc. 3.9 reads
    a Word figure into a <figure> with a <figcaption>; 3.1.3, which is
    what `apt-get install pandoc` gives CI on Ubuntu, writes a bare
    <img> and leaves the caption as an ordinary paragraph. The first
    CI run of the UCTS spec failed on exactly this, having passed
    locally, which is the difference a fixture exists to find. Both
    shapes are handled, and the caption text survives either way.
    """
    converted = dropped = 0

    def one(m: re.Match[str]) -> str:
        nonlocal converted, dropped
        block = m.group(0)
        dropped += len(IMG_TAG_RE.findall(block))
        cap = re.search(r"<figcaption>(.*?)</figcaption>", block, re.S)
        if not cap:
            return ""
        caption = strip_tags(cap.group(1))
        if not caption:
            return ""
        converted += 1
        return f"*{caption}*"

    text = FIGURE_RE.sub(one, text)

    # Whatever the <figure> pass did not account for. An image with no
    # caption carries nothing a corpus can use, so it goes; leaving it
    # would put a tag or a dead link in front of the embedder.
    text, n_tags = IMG_TAG_RE.subn("", text)
    text, n_md = MD_IMG_RE.subn("", text)
    dropped += n_tags + n_md

    # An image alone on its line leaves the line blank, which tidy()
    # collapses. Nothing else is normalised here: a blanket squeeze of
    # repeated spaces would also eat Markdown's two-space hard break
    # and the padding inside a table row.
    return text, converted, dropped


def unescape_brackets(text: str) -> tuple[str, int, int]:
    """Undo pandoc's bracket escaping, and rebuild real checklists.

    `\\[ \\]` at the start of a line is a checkbox the author drew. GFM
    has a task-list item for exactly that, so it becomes one; anything
    else is a citation marker and only needs the backslashes removed.
    """
    boxes = 0

    def box(m: re.Match[str]) -> str:
        nonlocal boxes
        boxes += 1
        return "- [ ] "

    text, _ = re.subn(r"(?m)^\\\[\s*\\\]\s*", box, text)
    escapes = text.count("\\[") + text.count("\\]") + text.count("\\_")
    text = text.replace("\\[", "[").replace("\\]", "]").replace("\\_", "_")
    return text, boxes, escapes


def title_to_h1(text: str) -> tuple[str, str]:
    """Give the document the single H1 Word did not.

    A Word title is a paragraph style, not a structural level, so pandoc
    writes it as bold body text. The sections underneath it were written
    with Heading 1, which pandoc correctly makes `#`. The result is a
    document with no title and ten peer H1s, and a chunker splitting on
    headings attaches the title to whatever follows it as prose.

    Promoting the title in place would make it a peer of the sections it
    covers, so the headings move down a level and the title takes the H1
    that is now free. A document already opening with an H1 was authored
    properly and is left alone, and so is one deep enough that demoting
    would run past H6.
    """
    lines = text.split("\n")
    first = next((i for i, ln in enumerate(lines) if ln.strip()), None)
    if first is None:
        return text, "empty"
    m = re.fullmatch(r"\*\*(.+?)\*\*", lines[first].strip())
    if not m:
        return text, "no bold title line" if not lines[first].startswith("# ") else "already titled"
    title = m.group(1).strip()

    if not re.search(r"(?m)^# ", text):
        lines[first] = f"# {title}"
        return "\n".join(lines), f"title promoted: {title}"

    if re.search(r"(?m)^#{6} ", text):
        return text, f"title left as bold: demoting would pass H6"

    lines[first] = f"# {title}"
    for i, ln in enumerate(lines):
        if i != first and re.match(r"^#{1,5} ", ln):
            lines[i] = "#" + ln
    return "\n".join(lines), f"title promoted and {len(re.findall(r'(?m)^#{1,5} ', text))} headings demoted"


def tidy(text: str) -> str:
    text = re.sub(r"\n{3,}", "\n\n", text)
    return text.strip() + "\n"


def convert(src: Path, dest: Path, dry_run: bool) -> tuple[bool, list[str]]:
    notes: list[str] = []
    try:
        raw = subprocess.run(
            ["pandoc", "-f", "docx", "-t", "gfm", "--wrap=none", str(src)],
            check=True,
            capture_output=True,
            text=True,
        ).stdout
    except FileNotFoundError:
        return False, ["pandoc is not installed"]
    except subprocess.CalledProcessError as exc:
        return False, [f"pandoc failed: {exc.stderr.strip()[:200]}"]

    text, callouts, problems = callouts_to_blockquotes(raw)
    notes += problems
    text, figures, imgs = figures_to_captions(text)
    text, bolds = merge_bold_runs(text)
    text, boxes, escapes = unescape_brackets(text)
    text, titling = title_to_h1(text)
    text = tidy(text)

    notes.append(
        f"{callouts} callouts, {figures} figure captions ({imgs} images dropped), "
        f"{bolds} split bold runs, {boxes} checkboxes, {escapes} escapes"
    )
    notes.append(titling)

    # The whole point: do not write a file that still has HTML in it.
    leftover = TAG_RE.findall(text)
    if leftover:
        kinds = sorted({t.split()[0].strip("<>/") for t in leftover})
        return False, notes + [f"{len(leftover)} HTML tags remain: {', '.join(kinds[:8])}"]

    words = len(text.split())
    notes.append(f"{words} words, {len(text.splitlines())} lines")
    if not dry_run:
        dest.write_text(text, encoding="utf-8")
    return True, notes


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("sources", nargs="+", type=Path)
    ap.add_argument("--dry-run", action="store_true", help="report without writing")
    ap.add_argument(
        "--overwrite",
        action="store_true",
        help="replace an existing .md; refused otherwise, because a hand-edited "
        "file is the more valuable one",
    )
    args = ap.parse_args()

    failures = 0
    for src in args.sources:
        if not src.is_file():
            print(f"FAIL  {src}: not a file")
            failures += 1
            continue
        dest = src.with_suffix(".md")
        if dest.exists() and not args.overwrite and not args.dry_run:
            print(f"SKIP  {src.name}: {dest.name} exists, pass --overwrite to replace")
            continue
        ok, notes = convert(src, dest, args.dry_run)
        tag = "ok  " if ok else "FAIL"
        print(f"{tag}  {src.name} -> {dest.name}")
        for n in notes:
            print(f"        {n}")
        if not ok:
            failures += 1
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
