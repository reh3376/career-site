#!/usr/bin/env python3
"""Apply the corrections Roger adjudicated across the résumé archive.

Roger writes a fresh résumé per posting, so a fact that drifted once is
now spread across dozens of files. Three drifts were put to him with
the evidence for each, and these are his rulings:

1. Employment chronology. Nine résumés said Joy Global 2007-2015,
   Sazerac 2015-2018, Bardstown Bourbon 2018-2022, six of them in
   month precision. Forty-nine, and the master, said 2006, 2017, 2017.
   Ruling: the master is correct, so the boundaries move back a year
   and keep their month where one is written.

2. Employer designation. Ruling: "Whiskey House of Kentucky,
   Elizabethtown, KY". This changes the master too, which said
   "Whiskey House, Bardstown, KY". Prose mentions of the company are
   left alone; only the employer line in an experience entry is
   normalised, because rewriting "while at Whiskey House" into the
   full legal name mid-sentence reads like a contract, not a résumé.

3. Associate degree. Thirty-four files said "Electronics Engineering
   Technology", thirteen said "Electrical". Ruling: Electrical, which
   is what the diploma says.

MAINresumeV and MAINresumeV 3 are excluded. They carry an entirely
different job history (11/2007-07/2009, 4/1999-11/2003) and are a much
older résumé rather than a variant of the current one, so correcting
them would be editing a document nobody is claiming is current.

Every substitution is counted and printed. A silent edit to a factual
claim about someone's career is the failure mode worth engineering
against.

Usage:
    services/sidecar/.venv/bin/python scripts/apply_resume_corrections.py --dry-run
    services/sidecar/.venv/bin/python scripts/apply_resume_corrections.py
"""

from __future__ import annotations

import argparse
import re
from pathlib import Path

EXCLUDE = {"MAINresumeV", "MAINresumeV 3"}

DASH = r"(?:-{1,3}|[–—])"

# (label, pattern, replacement)
CORRECTIONS: list[tuple[str, str, str]] = [
    # 1. Chronology, month-precision form. The month is kept and only
    #    the year moves, so "1/2018" becomes "1/2017" rather than a
    #    bare year that would lose precision the author had.
    ("Bardstown Bourbon / Sazerac boundary: 1/2018 -> 1/2017", r"\b1/2018\b", "1/2017"),
    ("Joy Global start: 1/2007 -> 1/2006", r"\b1/2007\b", "1/2006"),
    # The condensed early-career span ends where Joy Global begins, so
    # it moves with it.
    ("condensed early career: 1996 to 2007 -> 1996 to 2006", r"\b1996 to 2007\b", "1996 to 2006"),
    (
        "condensed early career: 1996 – 2007 -> 1996 – 2006",
        rf"\b1996\s*{DASH}\s*2007\b",
        "1996 – 2006",
    ),
    # 2. Employer designation.
    (
        "employer: Whiskey House, Bardstown, KY -> of Kentucky, Elizabethtown, KY",
        r"Whiskey House,\s*Bardstown,?\s*KY",
        "Whiskey House of Kentucky, Elizabethtown, KY",
    ),
    (
        "employer: Whiskey House of Kentucky, Bardstown, KY -> Elizabethtown, KY",
        r"Whiskey House of Kentucky,\s*Bardstown,?\s*KY",
        "Whiskey House of Kentucky, Elizabethtown, KY",
    ),
    # 3. Associate degree.
    (
        "degree: Electronics -> Electrical Engineering Technology",
        r"\bElectronics Engineering Technology\b",
        "Electrical Engineering Technology",
    ),
    (
        "degree: Applied Electrical -> Electrical Engineering Technology",
        r"\bApplied Electrical Engineering Technology\b",
        "Electrical Engineering Technology",
    ),
    # 4. Whiskey House end date written as a bare year.
    (
        "Whiskey House end: 2022 – 2026 -> 2022 – Jul 2026",
        rf"\b2022\s*{DASH}\s*2026\b",
        "2022 – Jul 2026",
    ),
]


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--src", type=Path, default=Path("docs/personal/resume"))
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    totals: dict[str, int] = {}
    touched = 0

    for path in sorted(args.src.glob("*.md")):
        if path.stem in EXCLUDE:
            print(f"skip  {path.name}  (superseded, different job history)")
            continue
        text = original = path.read_text(encoding="utf-8")
        notes = []
        for label, pattern, replacement in CORRECTIONS:
            text, n = re.subn(pattern, replacement, text)
            if n:
                notes.append(f"{n}x {label}")
                totals[label] = totals.get(label, 0) + n
        if text == original:
            continue
        touched += 1
        if not args.dry_run:
            path.write_text(text, encoding="utf-8")
        print(f"\n{path.name}")
        for n in notes:
            print(f"    {n}")

    verb = "would change" if args.dry_run else "changed"
    print(f"\n{'=' * 64}")
    print(f"{verb} {touched} files")
    for label, n in sorted(totals.items(), key=lambda kv: -kv[1]):
        print(f"  {n:4}  {label}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
