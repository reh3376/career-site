#!/usr/bin/env python3
"""Find the defects that were in reh-01-v01-2026 across every résumé.

That file carried a duplicated job entry, a truncated heading, an
editing note left in the executive summary, a second-person template
leftover, and a sentence that stopped mid-phrase. The .docx had all of
them too, so they were in the document rather than in any conversion,
and the worry was that anything sharing an ancestor shared them.

It did not. Across the other 59 résumés exactly one real defect turned
up, a mangled dash and a missing date dash in Resume-0326. The
duplicated entry and the template leftovers were unique to one file.

Getting to that number took one correction worth recording, because it
is the whole reason this script is shaped the way it is. The first
version matched patterns in the Markdown and reported 59 of 60 as
defective. Most of those patterns, pandoc's \\| and \\$ escapes and
its {.underline} spans, are correct output that never reaches the page.
Checking the rendered PDF text showed zero of them. A pattern that has
not been confirmed absent from a rendered PDF is a guess, and a
scanner full of guesses is worse than no scanner: it buries the two
files that matter under fifty-seven that do not.

Two classes, kept apart on purpose:

  MECHANICAL  deterministic to fix, and verified to reach the PDF.
  JUDGMENT    needs the author. Which of two job descriptions is the
              real one, what a truncated heading was meant to say,
              whether a dangling sentence is missing a clause.

The script only reports. Fixing is a separate, reviewable step, because
a script that rewrites sixty résumés unattended is how a subtle error
reaches forty employers at once.

Rendering is a separate step too, scripts/render_resume_pdf.py, which
compares each render's page count against the existing PDF. That check
only works while the old PDF is still there, so regenerate over them
rather than deleting first.

Usage:
    python3 scripts/audit_resume_defects.py
    python3 scripts/audit_resume_defects.py --json
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

RESUME_DIR = Path("docs/personal/resume")

# Deterministic to repair, and verified to actually reach the PDF.
#
# What is NOT here matters as much as what is. An earlier version of
# this script flagged pandoc's escapes, \\| and \\$, and its
# {.underline} spans, and reported 31 of 60 résumés as defective. They
# are correct Markdown and correct pandoc output: the rendered PDFs
# contain zero literal artifacts and zero stray backslashes. Flagging
# them turned a two-file problem into a fifty-nine-file alarm.
#
# The rule this encodes: a pattern earns a place here only after its
# absence has been confirmed in rendered PDF text, not in the source.
MECHANICAL: dict[str, str] = {
    "google redirect wrapper": r"google\.com/url\?q=",
    "mangled triple dash": r"\w---\s",
    "stray lone period": r"(?m)^>?\s*\.\s*$",
    "missing dash in date range": r"\b(19|20)\d{2}\s+Present\b",
}

# Needs the author's decision.
JUDGMENT: dict[str, str] = {
    # The real failure mode is a bold run that ends mid-word AND with no
    # space, as in "**Advanced Automation & Contro**Model predictive".
    # "**Engineering: **Process Engineering" is a bold label with its
    # colon inside and a trailing space; it renders correctly and must
    # not be flagged. Requiring no trailing space is what separates them.
    "truncated heading": r"\*\*[A-Z][^*\n]{4,40}[^.:*\s]\*\*[A-Z][a-z]",
    "editing note in summary": r"Will include standard summary",
    "second-person leftover": r"\byour business\b",
    "sentence ends mid-phrase": r"open-source contribution\s*$",
    # Not a defect, a disagreement: the site states thirty years. Listed
    # so the inconsistency is visible, not so it is automatically
    # "fixed".
    "conflicting years claim": r"\b25\+\s*years\b",
}

# Deliberately not checked: the phone number. It is a defect on a
# résumé published to an indexed page and entirely correct on one sent
# to an employer, so whether it belongs is a property of the
# destination, not of the file.

def duplicate_job_entries(text: str) -> list[str]:
    """Job headings that appear more than once verbatim.

    A résumé legitimately repeats a company across promotions, so the
    whole heading including dates has to match before it counts.
    """
    headings = re.findall(
        r"(?m)^#{0,4}\s*\**\s*([A-Z][^\n|*]{4,70}\|[^\n*]{4,70})\**\s*$", text
    )
    seen: dict[str, int] = {}
    for h in headings:
        key = re.sub(r"\s+", " ", h).strip().rstrip("*").strip()
        seen[key] = seen.get(key, 0) + 1
    return [h for h, n in seen.items() if n > 1]


def scan(path: Path) -> dict:
    text = path.read_text(encoding="utf-8", errors="replace")
    mech = [name for name, pat in MECHANICAL.items() if re.search(pat, text)]
    judg = [name for name, pat in JUDGMENT.items() if re.search(pat, text)]
    dupes = duplicate_job_entries(text)
    if dupes:
        judg.append("duplicate job entry")
    return {
        "file": path.name,
        "mechanical": mech,
        "judgment": judg,
        "duplicates": dupes,
        "clean": not mech and not judg,
    }


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--json", action="store_true")
    args = ap.parse_args()

    files = sorted(RESUME_DIR.glob("*.md"))
    if not files:
        print(f"no résumés found in {RESUME_DIR}", file=sys.stderr)
        return 1
    results = [scan(p) for p in files]

    if args.json:
        print(json.dumps(results, indent=2))
        return 0

    affected = [r for r in results if not r["clean"]]
    print(f"{len(files)} résumés scanned, {len(affected)} with defects\n")

    # Which defects, and how widespread. The spread is the useful
    # number: one file is an oversight, forty is a shared template and a
    # different problem.
    tally: dict[str, int] = {}
    for r in results:
        for d in r["mechanical"] + r["judgment"]:
            tally[d] = tally.get(d, 0) + 1
    if tally:
        print("defect                          files   class")
        for name, n in sorted(tally.items(), key=lambda kv: -kv[1]):
            kind = "mechanical" if name in MECHANICAL else "judgment"
            print(f"  {name:<30} {n:>4}   {kind}")
        print()

    for r in affected:
        bits = []
        if r["mechanical"]:
            bits.append("mech: " + ", ".join(r["mechanical"]))
        if r["judgment"]:
            bits.append("JUDGMENT: " + ", ".join(r["judgment"]))
        print(f"  {r['file']}")
        for b in bits:
            print(f"      {b}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
