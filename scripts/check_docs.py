#!/usr/bin/env python3
"""Fail when documentation that is supposed to track the code has stopped.

`AGENTS.md` "Docs discipline" and `NFR-DOC-01` list the documents a
behaviour change must update in the same commit. Most of those rules
are kept by whoever remembers them, which works until it does not: an
audit on 2026-10-14 found `SERVICES.md`'s public route list missing
four routes shipped the previous day, and migration 00062 with no row
at all, added by the same person who had read the rule that morning.

Two of those rules are mechanical, so they are checked here rather than
remembered:

**Every migration has a row in SERVICES.md.** The file is the operator's
map of the system; a migration absent from it is a schema change nobody
reading that document knows happened.

**The FSD's current-state table matches the generated counts.** A
hand-written tally of requirement states beside a generated one always
loses; the first version of that table was wrong by one before its own
commit finished.

**Every public route appears in SERVICES.md.** `public-routes.ts` is the
one list that drives the proxy, `robots.txt` and the sitemap, so a route
added there is public immediately. If `SERVICES.md` does not know, its
route table describes a smaller site than the one running.

The rules this does NOT check are the ones that need judgement: whether
an `FR-*` row was amended, whether an ADR was written, whether the
tuning log records the reasoning. A check that pretended to cover those
would be worse than none, because it would read as coverage.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import json

REPO = Path(__file__).resolve().parent.parent
SERVICES = REPO / "SERVICES.md"
MIGRATIONS = REPO / "services" / "api" / "internal" / "db" / "migrations"
ROUTES_TS = REPO / "apps" / "web" / "src" / "lib" / "public-routes.ts"

# Routes a reader does not need in an operator's route table: the
# token-bearing flows, which are documented as "verify / reset /
# one-click approval" rather than individually, and the decision test's
# internal screens, which are reachable only by taking it.
ROUTE_EXEMPT = {
    "/verify",
    "/reset-password",
    "/forgot-password",
    "/register/check-email",
    "/admin/decision",
    "/decision-test/run",
    "/decision-test/thanks",
}


def migrations_documented(text: str) -> list[str]:
    missing = []
    for f in sorted(MIGRATIONS.glob("*.sql")):
        num = f.name.split("_", 1)[0]
        if num in text or f.stem in text or f.name in text:
            continue
        missing.append(f.name)
    return missing


def public_paths() -> list[str]:
    src = ROUTES_TS.read_text(encoding="utf-8")
    block = re.search(r"export const PUBLIC_PATHS = \[(.*?)\] as const;", src, re.S)
    if not block:
        return []
    return re.findall(r'"([^"]+)"', block.group(1))


def routes_documented(text: str) -> list[str]:
    return [
        p
        for p in public_paths()
        if p not in ROUTE_EXEMPT and f"`{p}`" not in text
    ]


# The FSD's "Current state" table, which must agree with build-stats.
FSD = REPO / "docs" / "FSD.md"
STATS = REPO / "apps" / "web" / "src" / "lib" / "build-stats.json"
FSD_ROWS = {
    "Shipped": "requirementsShipped",
    "Partially shipped": "requirementsPartial",
    "Dropped, decided against": "requirementsDropped",
    "Deferred, blocked on content": "requirementsDeferred",
    "Not built": "requirementsNotBuilt",
}


def fsd_table_agrees() -> list[str]:
    """Compare the *current-state* table, not the dated audit above it.

    Both tables use the same row labels, so a document-wide search finds
    the 2026-10-01 audit first and reports its historical figures as
    drift. The audit is a record of one day and is correct as written;
    only the section below it claims to describe now.
    """
    if not (FSD.exists() and STATS.exists()):
        return []
    stats = json.loads(STATS.read_text(encoding="utf-8"))["structure"]
    whole = FSD.read_text(encoding="utf-8")
    start = whole.find("## 2b. Current state")
    if start == -1:
        return ["docs/FSD.md has no \"## 2b. Current state\" section"]
    end = whole.find("\n## ", start + 4)
    text = whole[start : end if end != -1 else len(whole)]
    problems = []
    for label, key in FSD_ROWS.items():
        m = re.search(rf"^\| {re.escape(label)} \| (\d+) \|$", text, re.M)
        if not m:
            problems.append(
                f"FSD.md's current-state table has no row for {label!r}"
            )
            continue
        said, actual = int(m.group(1)), stats[key]
        if said != actual:
            problems.append(
                f"FSD.md says {said} {label!r}; counting the status column "
                f"gives {actual}. Run scripts/build_stats.py and correct the table."
            )
    return problems


def main() -> int:
    if not SERVICES.exists():
        print("SERVICES.md is missing")
        return 1
    text = SERVICES.read_text(encoding="utf-8")

    problems = 0

    missing_migrations = migrations_documented(text)
    for m in missing_migrations:
        print(
            f"SERVICES.md has no row for {m}. AGENTS.md: "
            '"Adds a migration, give it a row in SERVICES.md."'
        )
        problems += 1

    missing_routes = routes_documented(text)
    for r in missing_routes:
        print(
            f"SERVICES.md does not mention `{r}`, which public-routes.ts "
            "serves publicly. Its route table describes a smaller site "
            "than the one running."
        )
        problems += 1

    for msg in fsd_table_agrees():
        print(msg)
        problems += 1

    total_m = len(list(MIGRATIONS.glob("*.sql")))
    total_r = len([p for p in public_paths() if p not in ROUTE_EXEMPT])
    print(
        f"\n{total_m} migrations and {total_r} public routes checked against "
        f"SERVICES.md, the FSD's current-state table checked against the "
        f"generated counts, {problems} problems"
    )
    if problems:
        print(
            "\nThese are the docs-discipline rules that can be checked\n"
            "mechanically. The ones needing judgement, amending an FR row,\n"
            "writing an ADR, logging the reasoning, are still yours."
        )
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
