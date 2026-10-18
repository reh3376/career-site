#!/usr/bin/env python3
"""Render an interview-prep Markdown document to a linked, searchable PDF.

Markdown is the source, PDF is the artifact, same split as
scripts/render_resume_pdf.py. pandoc converts to Typst through
scripts/templates/interview-pandoc.typ and the `typst` Python package
compiles it. Neither is a new dependency.

The point of this script, beyond the conversion, is the wiring. The
notes are dense with cross-references: "§6.3, Case D", "Story H",
"§8 (honest bridge)". On paper those are instructions to go and find
something. In a thirty-minute call that is time the reader does not
have, so every one of them is turned into a live internal link and the
headings are given stable anchors to land on.

Three things make that harder than it sounds, and each is handled here
rather than left to chance:

**Pandoc's automatic anchors throw away the section numbers.** A
heading "## 6.3 Case framework" gets the id `case-framework`, because
pandoc drops everything before the first letter. So a link written to
`#63-case-framework` silently points at nothing: no error, no warning,
a dead click in the room. This script therefore assigns its own
anchors, derived from the section number, and injects them as explicit
header attributes.

**It injects them into a copy.** The Markdown is the document its
author reads and edits; filling his headings with `{#sec-6-3}` to serve
the renderer would be the tool making the source worse to serve
itself.

**A reference only becomes a link if its target exists.** Every one is
resolved against the headings actually found, and anything unresolved
is reported and left as plain text, because a link that goes nowhere is
worse than no link at all. The counts are printed so a drop is visible.

Usage:
    services/sidecar/.venv/bin/python scripts/render_interview_pdf.py \
        docs/personal/interview-prep/reh-interview-BlueOriginPM.md \
        --out docs/personal/interview-prep/reh-interview-BlueOriginPMv2.pdf
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
import tempfile
from pathlib import Path

TEMPLATE = Path("scripts/templates/interview-pandoc.typ")

# "## 3. Openers", "### 6.3 Case framework (...)", "### Story D — ..."
H = re.compile(r"^(?P<hashes>#{1,6})\s+(?P<text>.+?)\s*$")
SECTION_NUM = re.compile(r"^(?P<num>\d+(?:[.]\d+)?|\d+[a-z])\s*[.]?\s+")
STORY = re.compile(r"^Story\s+(?P<letter>[A-Z])\b")


def anchor_for(text: str) -> str | None:
    """A stable anchor from a heading, keyed on what prose refers to.

    Sections are referred to by number ("§6.3") and stories by letter
    ("Story D"), so those are the anchors. A heading that is neither
    gets none: nothing points at it, and inventing an id for it would
    only add noise to the document.
    """
    m = SECTION_NUM.match(text)
    if m:
        return "sec-" + m.group("num").replace(".", "-")
    m = STORY.match(text)
    if m:
        return "story-" + m.group("letter").lower()
    return None


def collect_anchors(lines: list[str]) -> dict[str, str]:
    found: dict[str, str] = {}
    for line in lines:
        m = H.match(line)
        if not m:
            continue
        a = anchor_for(m.group("text"))
        if a and a not in found:
            found[a] = m.group("text")
    return found


def add_anchors(lines: list[str]) -> list[str]:
    out = []
    for line in lines:
        m = H.match(line)
        if m:
            a = anchor_for(m.group("text"))
            if a and "{#" not in line:
                line = f"{m.group('hashes')} {m.group('text')} {{#{a}}}"
        out.append(line)
    return out


# A section reference: §0, §6.3, §13. Allowed to carry a letter suffix
# (§2b) because the specification numbering uses them.
SEC_REF = re.compile(r"§(?P<num>\d+(?:[.]\d+)?|\d+[a-z])")
# "Story D", and the plural forms the notes use just as often:
# "Stories A, B", "Stories A, B, F, and I". Each letter is linked
# separately, so "Stories C, D" is two destinations rather than one.
STORY_REF = re.compile(r"\bStory (?P<letter>[A-Z])\b")
STORIES_REF = re.compile(r"\bStories ((?:[A-Z](?:,\s*(?:and\s+)?|\s+and\s+)?)+)")
# "Case A" through "Case D" in section 6.3. They are bold lines rather
# than headings, so add_case_anchors gives them something to land on.
CASE_LINE = re.compile(r"^\*\*Case (?P<letter>[A-Z]):")
CASE_REF = re.compile(r"\bCase (?P<letter>[A-D])\b")
# Spans that must not be touched: existing links, code, and the ASCII
# architecture diagram.
SKIP = re.compile(r"\[[^\]]*\]\([^)]*\)|`[^`]*`")


def add_case_anchors(lines: list[str]) -> tuple[list[str], set[str]]:
    """Give the four cases in section 6.3 something to link to.

    They are written as bold paragraph openers rather than headings,
    and promoting them would change the author's structure to suit the
    renderer. An empty span carrying an id does the same job and leaves
    the outline alone.
    """
    out: list[str] = []
    found: set[str] = set()
    for line in lines:
        m = CASE_LINE.match(line)
        if m:
            a = "case-" + m.group("letter").lower()
            found.add(a)
            out.append(f"[]{{#{a}}}")
            out.append("")
        out.append(line)
    return out, found


def acronyms_from_table(lines: list[str]) -> list[str]:
    """The acronyms the document defines, read from its own table.

    The notes open with a 95-row glossary, because the author wrote
    them knowing he would be reading them under pressure. On paper that
    table is a page you flip back to. Here every acronym in the prose
    links to it, so an unfamiliar "RTY" or "TEEP" is one click from its
    expansion instead of a scroll back to section zero.

    Read from the table rather than from a hand-kept list: the glossary
    is the thing the author maintains, so a row added there starts
    linking without anyone remembering to update this.
    """
    out: list[str] = []
    for line in lines:
        m = re.match(r"^\|\s*([A-Za-z0-9][A-Za-z0-9./ -]*?)\s*\|\s*[^|]+\|\s*$", line)
        if not m:
            continue
        cell = m.group(1).strip()
        if cell in {"Acronym", "Figure", "Term", "Metric", "When", "Where"}:
            continue
        # A cell may define several spellings at once: "MPS / MRP",
        # "ISA-18.2 / ISA-88 / ISA-95", "A.S. / B.S.".
        for part in cell.split("/"):
            part = part.strip()
            # Only genuine acronyms: short, and not ordinary words. A
            # one-letter token would match everywhere.
            if 2 <= len(part) <= 12 and re.match(r"^[A-Z0-9][A-Za-z0-9.-]*$", part):
                out.append(part)
    # Longest first, so "ISA-18.2" is matched before "ISA" would be.
    return sorted(set(out), key=len, reverse=True)


def link_acronyms(
    lines: list[str], acronyms: list[str], table_line: int
) -> tuple[list[str], int]:
    """Link the first use of each acronym in each section.

    Per section rather than once per document, because the reader does
    not read this front to back; they jump to section 6.1 and need the
    link to be there. And only the first use in that section, because a
    page where every third token is blue is a page nobody can read.

    The glossary itself is skipped, as are headings, code and anything
    already inside a link.
    """
    pattern = re.compile(r"(?<![A-Za-z0-9-])(" + "|".join(re.escape(a) for a in acronyms) + r")(?![A-Za-z0-9-])")
    out: list[str] = []
    seen: set[str] = set()
    count = 0
    in_fence = False
    in_glossary = False

    for i, line in enumerate(lines):
        if line.startswith("```"):
            in_fence = not in_fence
            out.append(line)
            continue
        if H.match(line):
            # A new section resets the "already linked here" set, and a
            # new heading tells us whether we are inside the glossary.
            if H.match(line).group("hashes") == "#" * len(H.match(line).group("hashes")):
                seen = set()
            in_glossary = "Acronyms used in these notes" in line
            out.append(line)
            continue
        if in_fence or in_glossary or not line.strip() or line.lstrip().startswith("|"):
            out.append(line)
            continue

        held: list[str] = []

        def hold(m: re.Match) -> str:
            held.append(m.group(0))
            return f"\x00{len(held) - 1}\x00"

        work = SKIP.sub(hold, line)

        def one(m: re.Match) -> str:
            nonlocal count
            a = m.group(1)
            if a in seen:
                return a
            seen.add(a)
            count += 1
            return f"[{a}](#acronyms)"

        work = pattern.sub(one, work)
        for j, original in enumerate(held):
            work = work.replace(f"\x00{j}\x00", original)
        out.append(work)

    return out, count


def link_refs(lines: list[str], anchors: dict[str, str]) -> tuple[list[str], dict]:
    """Turn §N and Story X in the prose into Markdown links.

    Skips headings (a section does not need to link to itself), fenced
    code, and any span already inside a link or backticks.
    """
    out: list[str] = []
    stats = {"sec": 0, "story": 0, "case": 0, "unresolved": {}}
    in_fence = False

    for line in lines:
        if line.startswith("```"):
            in_fence = not in_fence
            out.append(line)
            continue
        if in_fence or H.match(line) or not line.strip():
            out.append(line)
            continue

        # Protect spans that are already links or code.
        held: list[str] = []

        def hold(m: re.Match) -> str:
            held.append(m.group(0))
            return f"\x00{len(held) - 1}\x00"

        work = SKIP.sub(hold, line)

        def sec(m: re.Match) -> str:
            a = "sec-" + m.group("num").replace(".", "-")
            if a not in anchors:
                stats["unresolved"].setdefault(m.group(0), 0)
                stats["unresolved"][m.group(0)] += 1
                return m.group(0)
            stats["sec"] += 1
            return f"[{m.group(0)}](#{a})"

        def story(m: re.Match) -> str:
            a = "story-" + m.group("letter").lower()
            if a not in anchors:
                stats["unresolved"].setdefault(m.group(0), 0)
                stats["unresolved"][m.group(0)] += 1
                return m.group(0)
            stats["story"] += 1
            return f"[{m.group(0)}](#{a})"

        def stories(m: re.Match) -> str:
            # Rebuild the run, linking each letter and keeping the
            # separators ("Stories A, B, F, and I") exactly as written.
            def letter(lm: re.Match) -> str:
                a = "story-" + lm.group(0).lower()
                if a not in anchors:
                    return lm.group(0)
                stats["story"] += 1
                return f"[{lm.group(0)}](#{a})"

            return "Stories " + re.sub(r"[A-Z]", letter, m.group(1))

        def case(m: re.Match) -> str:
            a = "case-" + m.group("letter").lower()
            if a not in anchors:
                stats["unresolved"].setdefault(m.group(0), 0)
                stats["unresolved"][m.group(0)] += 1
                return m.group(0)
            stats["case"] += 1
            return f"[{m.group(0)}](#{a})"

        work = SEC_REF.sub(sec, work)
        work = STORIES_REF.sub(stories, work)
        work = STORY_REF.sub(story, work)
        # Not on the case's own label line: a heading that links to
        # itself is a click that does nothing, and it colours the label
        # as though it led somewhere.
        if not CASE_LINE.match(line):
            work = CASE_REF.sub(case, work)

        for i, original in enumerate(held):
            work = work.replace(f"\x00{i}\x00", original)
        out.append(work)

    return out, stats


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("src", type=Path)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--title", default=None)
    ap.add_argument("--subtitle", default=None)
    ap.add_argument("--header", default=None, help="running header text; defaults to the title")
    ap.add_argument(
        "--no-toc",
        action="store_true",
        help="short document: no contents page, no page break per section",
    )
    ap.add_argument("--keep-typst", action="store_true", help="keep the .typ for debugging")
    args = ap.parse_args()

    if not args.src.exists():
        print(f"{args.src} does not exist", file=sys.stderr)
        return 1
    if not TEMPLATE.exists():
        print(f"{TEMPLATE} does not exist; run from the repository root", file=sys.stderr)
        return 1

    lines = args.src.read_text(encoding="utf-8").split("\n")
    anchors = collect_anchors(lines)
    # The guard exists because a silently broken heading scan would drop
    # every cross-reference without failing. It only means something for
    # a document that *has* cross-references: a short piece of prose with
    # no numbered sections legitimately has no anchors, and refusing to
    # render it would be the check inventing a problem.
    body_text = "\n".join(lines)
    wants_links = bool(SEC_REF.search(body_text) or STORY_REF.search(body_text))
    if wants_links and len(anchors) < 5:
        print(
            f"{args.src} makes section or story references but only "
            f"{len(anchors)} anchors were found; the heading scan is broken "
            "and every cross-reference would silently drop",
            file=sys.stderr,
        )
        return 1

    body = add_anchors(lines)
    # The glossary needs a landing anchor of its own; it is a heading
    # with neither a section number nor a story letter, so anchor_for
    # gives it none.
    body = [
        (f"{m.group('hashes')} {m.group('text')} {{#acronyms}}" if (m := H.match(ln)) and "Acronyms used in these notes" in ln else ln)
        for ln in body
    ]
    body, case_anchors = add_case_anchors(body)
    anchors.update({a: "case" for a in case_anchors})
    body, stats = link_refs(body, anchors)

    # Same reasoning as the anchor guard: it is worth failing when a
    # glossary is present and the scan of it came back empty, and it is
    # not worth failing on a document that simply has no glossary.
    has_glossary = "Acronyms used in these notes" in body_text
    acronyms = acronyms_from_table(lines) if has_glossary else []
    if has_glossary and len(acronyms) < 20:
        print(
            f"{args.src} has a glossary but only {len(acronyms)} acronyms "
            "parsed from it; the table scan is broken and the expansions "
            "would not be linked",
            file=sys.stderr,
        )
        return 1
    n_acr = 0
    if acronyms:
        body, n_acr = link_acronyms(body, acronyms, 0)
    stats["acronyms"] = n_acr
    stats["acronym_terms"] = len(acronyms)

    # The document's own H1 is always removed, not only when no --title
    # was given. With --shift-heading-level-by=-1 it becomes a level 0
    # heading, which pandoc promotes into the title metadata, silently
    # overriding -M title and carrying the raw "---" and the line break
    # with it. So the H1 comes out of the body either way and the title
    # is set from one place.
    title = args.title
    if body and body[0].startswith("# "):
        if title is None:
            title = body[0][2:].strip()
        body = body[1:]

    with tempfile.TemporaryDirectory() as td:
        tmp_md = Path(td) / "doc.md"
        tmp_md.write_text("\n".join(body), encoding="utf-8")
        typ = (args.out.parent / (args.out.stem + ".typ")) if args.keep_typst else Path(td) / "doc.typ"

        cmd = [
            "pandoc",
            str(tmp_md),
            "-f",
            # tex_math_dollars off: the notes quote redacted dollar figures
            # ("$XB contracts", "$X0MM+ in cost savings") and pandoc
            # reads the pair as inline math, mangling both. There is no
            # mathematics in the document.
            "markdown+pipe_tables+strikeout+footnotes-tex_math_dollars",
            "-t",
            "typst",
            "--template",
            str(TEMPLATE),
            *([] if args.no_toc else ["--toc", "--toc-depth=2"]),
            # The document's sections are "## 1." through "## 13.", so
            # without this they arrive as level 2 and the template's
            # section styling, which is the page-break and the rule,
            # lands on nothing.
            "--shift-heading-level-by=-1",
            "-o",
            str(typ),
        ]
        if title:
            cmd += ["-M", f"title={title}"]
        if args.subtitle:
            cmd += ["-M", f"subtitle={args.subtitle}"]
        if args.header:
            cmd += ["-M", f"header={args.header}"]

        r = subprocess.run(cmd, capture_output=True, text=True)
        if r.returncode != 0:
            print(r.stderr, file=sys.stderr)
            return 1
        if r.stderr.strip():
            print(r.stderr.strip(), file=sys.stderr)

        import typst

        args.out.parent.mkdir(parents=True, exist_ok=True)
        try:
            # The root has to contain the input, and the generated .typ
            # normally lives in a temporary directory outside the repo.
            # Nothing is imported at compile time, so the file's own
            # directory is a sufficient root.
            typst.compile(str(typ), output=str(args.out), root=str(typ.parent))
        except Exception as exc:  # noqa: BLE001 - the message is the point
            print(f"typst failed:\n{exc}", file=sys.stderr)
            return 1

    size = args.out.stat().st_size
    print(f"{args.out}  {size / 1024:.0f} KB")
    print(
        f"  {len(anchors)} anchors, {stats['sec']} section links, "
        f"{stats['story']} story links, {stats['case']} case links, "
        f"{stats['acronyms']} acronym links "
        f"from {stats['acronym_terms']} glossary terms"
    )
    if stats["unresolved"]:
        print("  unresolved, left as plain text:")
        for ref, n in sorted(stats["unresolved"].items()):
            print(f"    {ref} x{n}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
