#!/usr/bin/env python3
"""Convert .docx résumés to canonical Markdown, correcting known date drift.

Markdown is the canonical form: editable, diffable, ingestible by the
corpus (which only takes .md and .txt), and renderable to PDF. The
.docx and .pdf copies are outputs and archives respectively.

Two transformations beyond a plain pandoc run:

Headings. The sources mark sections with bold all-caps paragraphs
rather than Word heading styles, so pandoc emits `**SUMMARY**`. The
corpus chunker segments on headings, so a résumé converted verbatim
becomes one undifferentiated block. Short all-caps bold paragraphs are
promoted to level-two headings.

Dates. Roger adjudicated the drift the audit found (see
scripts/audit_resume_dates.py): the master chronology is correct and
the outliers are wrong. Corrections are applied here rather than by
hand across dozens of files, and every substitution is reported so a
silent edit to a factual claim is impossible.

Usage:
    services/sidecar/.venv/bin/python scripts/resume_to_md.py --dry-run
    services/sidecar/.venv/bin/python scripts/resume_to_md.py
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

# Corrections carry a reason so the report explains itself, and are
# anchored tightly enough that they cannot fire on unrelated text.
# Pandoc writes an en dash as the two-character escape `--`, so a
# single-character dash class silently matches nothing in its output.
DASH = r"(?:-{1,3}|[–—])"
CORRECTIONS: list[tuple[str, str, str]] = [
    (
        rf"\b2022\s*{DASH}\s*(?:Present|present|PRESENT|Current|current)\b",
        "2022 – Jul 2026",
        "Whiskey House ended Jul 2026; 'Present' is no longer true",
    ),
    (
        rf"\b2018\s*{DASH}\s*2022\b",
        "2017 – 2022",
        "Bardstown Bourbon started 2017, not 2018",
    ),
    (
        rf"\b2015\s*{DASH}\s*2018\b",
        "2015 – 2017",
        "Sazerac ended 2017, not 2018",
    ),
    (
        rf"\b2007\s*{DASH}\s*2015\b",
        "2006 – 2015",
        "Joy Global started 2006, not 2007",
    ),
    (
        r"\b2007 to 2015\b",
        "2006 to 2015",
        "Joy Global started 2006, not 2007",
    ),
]

# A section label: entirely bold, short, and upper case. Title-cased so
# the heading reads as a heading rather than as shouting.
BOLD_LABEL = re.compile(r"^\*\*([A-Z][A-Z0-9 &/,'’.\-]{2,60})\*\*$")


def merge_bold_runs(text: str) -> tuple[str, int]:
    """Collapse `****`, which is a bold run closing and reopening.

    Word stores a heading like "Engineering & Technology" as three runs
    when the ampersand was typed or pasted separately, and pandoc is
    faithful to that: `**Engineering ****&**** Technology**`. Markdown
    reads the empty `****` as a literal, so the rendered line shows
    stray asterisks. Deleting the sequence rejoins the runs into the
    single bold span the author intended.
    """
    count = text.count("****")
    return text.replace("****", ""), count


def preserve_row_blocks(text: str) -> tuple[str, int]:
    """Keep a block laid out as rows from collapsing into a paragraph.

    Markdown joins consecutive lines into one paragraph, which is right
    for prose and wrong for a competencies block written as rows of
    pipe-delimited terms. Roger's Heaven Hill résumé lays ten rows out
    that way; rendered without this, they run together and "Alternatives
    and Best Value Options Analysis" is followed on the same line by
    "Total Installed Cost Estimating".

    The obvious fix, pandoc's hard_line_breaks extension, turns every
    newline into a break and so destroys every wrapped prose paragraph
    in the archive. This is narrower: only a run of two or more adjacent
    lines that each carry a pipe separator, which prose does not.

    Applied at render time rather than to the source, because the source
    is the author's document and this is a presentation concern.
    """
    lines = text.split("\n")

    def is_row(line: str) -> bool:
        s = line.strip()
        return (
            " | " in s
            and not s.startswith(("#", "-", "*", ">", "|"))
            and not s.endswith("|")
        )

    out, marked, i = [], 0, 0
    while i < len(lines):
        run_end = i
        while run_end < len(lines) and is_row(lines[run_end]):
            run_end += 1
        if run_end - i >= 2:
            # Every line but the last gets a hard break; the last ends
            # the block normally so the following blank line still
            # closes the paragraph.
            for j in range(i, run_end - 1):
                out.append(lines[j].rstrip() + "  ")
                marked += 1
            out.append(lines[run_end - 1])
            i = run_end
        else:
            out.append(lines[i])
            i += 1
    return "\n".join(out), marked


def promote_headings(text: str) -> tuple[str, int]:
    out, promoted = [], 0
    for line in text.split("\n"):
        m = BOLD_LABEL.match(line.strip())
        if m:
            label = m.group(1).strip().rstrip(":")
            # The name line is bold and upper case in some résumés but
            # is the document title, not a section.
            if label.replace(".", "").replace(" ", "").isalpha() and len(label.split()) <= 5 and "HENLEY" in label:
                out.append(f"# {label.title()}")
            else:
                out.append(f"## {label.title()}")
            promoted += 1
        else:
            out.append(line)
    return "\n".join(out), promoted


def apply_corrections(text: str, name: str) -> tuple[str, list[str]]:
    notes = []
    for pattern, replacement, reason in CORRECTIONS:
        hits = re.findall(pattern, text)
        if not hits:
            continue
        text = re.sub(pattern, replacement, text)
        notes.append(f"{len(hits)}x '{hits[0] if isinstance(hits[0], str) else ''}' -> {replacement}  ({reason})")
    return text, notes


def convert(src: Path, dest: Path, dry_run: bool) -> tuple[bool, list[str]]:
    try:
        raw = subprocess.run(
            ["pandoc", str(src), "-t", "markdown-simple_tables-multiline_tables", "--wrap=none"],
            capture_output=True,
            text=True,
            check=True,
        ).stdout
    except subprocess.CalledProcessError as exc:
        return False, [f"pandoc failed: {exc.stderr.strip()[:200]}"]

    text, merged = merge_bold_runs(raw)
    text, promoted = promote_headings(text)
    text, notes = apply_corrections(text, src.name)
    if promoted:
        notes.insert(0, f"{promoted} section labels promoted to headings")
    if merged:
        notes.insert(0, f"{merged} split bold runs rejoined")
    if not dry_run:
        dest.write_text(text, encoding="utf-8")
    return True, notes


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--src", type=Path, default=Path("docs/personal/resume"))
    ap.add_argument("--dry-run", action="store_true")
    # Overwriting is per file and never wholesale. A blanket --force
    # destroyed three hand-edited .md files that happened to sit beside
    # an older .docx: the Markdown was the newer document and the .docx
    # was the stale one, which is the opposite of what a bulk rewrite
    # assumes. Naming each file makes that mistake impossible to make
    # by accident.
    ap.add_argument(
        "--overwrite",
        action="append",
        default=[],
        metavar="STEM",
        help="rewrite this one .md even though it exists, e.g. --overwrite reh-resume-AWS (repeatable)",
    )
    args = ap.parse_args()

    docx = sorted(args.src.glob("*.docx"))
    if not docx:
        print(f"no .docx under {args.src}", file=sys.stderr)
        return 1

    ok = failed = corrected = 0
    for src in docx:
        dest = src.with_suffix(".md")
        if dest.exists() and src.stem not in args.overwrite:
            print(f"skip  {src.name}  ({dest.name} already exists)")
            continue
        good, notes = convert(src, dest, args.dry_run)
        if not good:
            print(f"FAIL  {src.name}: {notes[0]}", file=sys.stderr)
            failed += 1
            continue
        ok += 1
        date_notes = [n for n in notes if "->" in n]
        if date_notes:
            corrected += 1
            print(f"ok    {dest.name}")
            for n in date_notes:
                print(f"        CORRECTED  {n}")
        else:
            print(f"ok    {dest.name}")

    verb = "would convert" if args.dry_run else "converted"
    print(f"\n{verb} {ok}, {corrected} had dates corrected, {failed} failed")
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
