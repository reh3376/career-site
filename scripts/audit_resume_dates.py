#!/usr/bin/env python3
"""Compare every tailored résumé's dates against the canonical chronology.

Roger writes a fresh résumé per posting, so the same job is retyped
dozens of times and a date can drift in one file without anyone
noticing. A recruiter who compares two of his résumés and finds
different years for the same employer has found a credibility problem,
not a typo.

This is the mechanical pass: it finds every employer mention and the
date range nearest to it, and reports only what disagrees with the
chronology. Judgment is left to whoever reads the report, because the
interesting cases are ambiguous by nature: "2022 - Present" is not
wrong so much as stale, and a résumé that omits an employer entirely
may be a deliberate cut rather than an error.

The chronology is personal data and lives in docs/personal (gitignored)
rather than in this file.

Usage:
    services/sidecar/.venv/bin/python scripts/audit_resume_dates.py
    services/sidecar/.venv/bin/python scripts/audit_resume_dates.py --json
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

# A year range in any of the punctuations Word and its authors produce:
# hyphen, en dash, em dash, "to". The right-hand side may be a year, a
# month and year, or a word like Present.
YEAR = r"(?:19|20)\d{2}"
MONTH = r"(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]*"
SIDE = rf"(?:{MONTH}\.?\s+)?{YEAR}|Present|present|PRESENT|Current|current"
RANGE = re.compile(rf"\b((?:{MONTH}\.?\s+)?{YEAR})\s*(?:[-–—]|to)\s*({SIDE})\b")

# How many lines either side of an employer mention to search for its
# dates. Layouts vary: some put role and dates on one line with the
# employer on the next, some invert it, some use a table.
WINDOW = 3


def norm(side: str) -> str:
    """Reduce a date side to something comparable: the year, or PRESENT."""
    s = side.strip()
    if re.fullmatch(r"(?i)present|current", s):
        return "PRESENT"
    m = re.search(YEAR, s)
    return m.group(0) if m else s


def load_chronology(path: Path) -> dict:
    data = json.loads(path.read_text(encoding="utf-8"))
    for job in data["employment"]:
        job["_from"] = norm(job["from"])
        job["_to"] = norm(job["to"])
    return data


def employers_on(line: str, chron: dict) -> list[dict]:
    """Which employers a line names."""
    low = line.lower()
    return [j for j in chron["employment"] if any(a.lower() in low for a in j["aliases"])]


def attribute_ranges(lines: list[str], chron: dict) -> tuple[dict[str, list], list[dict]]:
    """Attach each date range to the employer it belongs to.

    Ranges are attributed rather than searched for. Searching outward
    from an employer name finds the neighbouring job's dates just as
    readily as its own, which in an experience block, where entries sit
    three lines apart, is wrong about as often as it is right.

    A line naming two or more employers with a single range is a
    condensed early career ("Lucent, Bell Atlantic, Columbia Gas
    1996-2006"). That is a deliberate summary, not a drifted date, so
    it gets its own category instead of one mismatch per employer.
    """
    by_employer: dict[str, list] = {}
    condensed: list[dict] = []

    for i, line in enumerate(lines):
        ranges = list(RANGE.finditer(line))
        if not ranges:
            continue
        named = employers_on(line, chron)

        if len(named) > 1:
            m = ranges[0]
            condensed.append(
                {
                    "kind": "condensed",
                    "employers": [j["employer"] for j in named],
                    "found": f"{norm(m.group(1))}-{norm(m.group(2))}",
                    "context": line.strip()[:110],
                }
            )
            continue

        if not named:
            # No employer on this line: attribute to the nearest one
            # within the window, but only if exactly one is in reach,
            # so an ambiguous gap is left unattributed rather than
            # guessed at.
            near = []
            for d in range(1, WINDOW + 1):
                for j in (i - d, i + d):
                    if 0 <= j < len(lines):
                        near.extend(employers_on(lines[j], chron))
                if near:
                    break
            if len({j["employer"] for j in near}) != 1:
                continue
            named = [near[0]]

        m = ranges[0]
        by_employer.setdefault(named[0]["employer"], []).append(
            (norm(m.group(1)), norm(m.group(2)), line.strip()[:110])
        )

    return by_employer, condensed


def audit_file(path: Path, chron: dict) -> list[dict]:
    """Findings for one résumé. Empty list means it agrees."""
    lines = path.read_text(encoding="utf-8", errors="replace").split("\n")
    by_employer, condensed = attribute_ranges(lines, chron)
    findings: list[dict] = list(condensed)

    condensed_employers = {e for c in condensed for e in c["employers"]}

    for job in chron["employment"]:
        mentioned = any(employers_on(line, chron) and job in employers_on(line, chron) for line in lines)
        if not mentioned:
            findings.append(
                {"kind": "absent", "employer": job["employer"], "detail": "not mentioned"}
            )
            continue

        found = by_employer.get(job["employer"], [])
        if not found:
            # Already accounted for by a condensed line; not a gap.
            if job["employer"] in condensed_employers:
                continue
            findings.append(
                {"kind": "no-dates", "employer": job["employer"], "detail": "mentioned, no date range nearby"}
            )
            continue

        if any(f == job["_from"] and t == job["_to"] for f, t, _ in found):
            continue

        # "2022 - Present" where the master says "2022 - Jul 2026" is
        # its own category: the start agrees and the end is stale, which
        # is a different fix from a wrong start year.
        stale = [c for c in found if c[0] == job["_from"] and c[1] == "PRESENT"]
        if stale:
            findings.append(
                {
                    "kind": "stale-present",
                    "employer": job["employer"],
                    "expected": f"{job['_from']}-{job['_to']}",
                    "found": f"{stale[0][0]}-Present",
                    "context": stale[0][2],
                }
            )
            continue

        best = found[0]
        findings.append(
            {
                "kind": "mismatch",
                "employer": job["employer"],
                "expected": f"{job['_from']}-{job['_to']}",
                "found": f"{best[0]}-{best[1]}",
                "context": best[2],
            }
        )

    return findings


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--src", type=Path, default=Path("docs/personal/extracted/resume"))
    ap.add_argument("--chronology", type=Path, default=Path("docs/personal/chronology.json"))
    ap.add_argument("--json", action="store_true", help="machine-readable output")
    ap.add_argument(
        "--context",
        action="store_true",
        help="also print the source line each finding came from. It is verbatim résumé text, so this is off by default.",
    )
    args = ap.parse_args()

    if not args.chronology.exists():
        print(f"no chronology at {args.chronology}", file=sys.stderr)
        return 2
    chron = load_chronology(args.chronology)

    report = {}
    for path in sorted(args.src.iterdir()):
        if path.suffix != ".txt":
            continue
        report[path.name] = audit_file(path, chron)

    if args.json:
        print(json.dumps(report, indent=2))
        return 0

    clean = [n for n, f in report.items() if not f]
    dirty = {n: f for n, f in report.items() if f}

    # Absence is common and usually deliberate, so it is summarised
    # rather than listed per file; a mismatch is rare and always worth
    # reading in full.
    for name, findings in sorted(dirty.items()):
        hard = [f for f in findings if f["kind"] in ("mismatch", "stale-present")]
        if not hard:
            continue
        print(f"\n{name}")
        for f in hard:
            if f["kind"] == "mismatch":
                print(f"  MISMATCH  {f['employer']}: expected {f['expected']}, found {f['found']}")
            else:
                print(f"  STALE     {f['employer']}: expected {f['expected']}, found {f['found']}")
            # The context line is verbatim text from a résumé, so it is
            # the one piece of output that reproduces the document
            # rather than describing it. Off by default: the employer
            # and the two dates are enough to decide whether a finding
            # is real, and a report that is safe to paste into a chat
            # or redirect to a file is more useful than one that is
            # not. --context brings it back for the cases where the
            # surrounding wording is what settles the question.
            if args.context:
                print(f"            {f['context']}")

    counts: dict[str, int] = {}
    for findings in report.values():
        for f in findings:
            counts[f["kind"]] = counts.get(f["kind"], 0) + 1

    print(f"\n{'=' * 60}")
    print(f"{len(report)} résumés audited, {len(clean)} agree with the chronology on every employer they mention")
    for kind in ("mismatch", "stale-present", "no-dates", "absent"):
        if kind in counts:
            print(f"  {kind:14} {counts[kind]}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
