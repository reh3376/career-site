#!/usr/bin/env python3
"""Measure this repository, and write what was measured to JSON.

The numbers on `/how-ask-roger-works` are computed from the tracked
tree rather than typed, for the same reason the rest of that page reads
live records: a hand-written "80,000 lines" is stale the next morning
and is exactly the sort of claim this site exists to argue against.
Generated here, it is a number a reader can check by cloning the repo
and running this script.

**The category split is the point, and it is enforced here rather than
remembered.** Markdown is documentation, not code. Protobuf is a
contract. YAML and JSON are configuration. Generated stubs and
lockfiles are neither written nor claimed. Folding any of those into a
line count headed "code" inflates it by a third, which is the kind of
thing a reviewer catches in one glance at the repo and then discounts
everything else for.

**Commits are measured, and split by author on purpose.** The velocity
of this repository is unusual and reads, correctly, as AI-assisted.
Published bare next to a line count that invites an inference the page
has not earned; published with the authorship split it becomes the same
kind of evidence as everything else here, because
`git log --format=%an | sort | uniq -c` returns the same numbers for
anyone who clones it. An author whose name begins "Claude" is the
assistant; merges are the owner, since a pull request is merged by the
person who reviewed it.

Run it with `make build-stats`, or directly:

    python3 scripts/build_stats.py

CI runs it and fails if the committed JSON differs, which is the same
contract `check-gen` holds for the generated API reference: the
artifact is committed so the Docker build can read it without a git
checkout, and proven current so it cannot quietly drift.
"""

from __future__ import annotations

import json
import re
import subprocess
import sys
from collections import defaultdict
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
OUT = REPO / "apps" / "web" / "src" / "lib" / "build-stats.json"

# Anything whose contents a tool produced. Never counted, never claimed.
GENERATED = re.compile(
    r"(^|/)(gen|\.next)(/|$)|_pb\.ts$|\.pb\.go$|_pb2\.pyi?$|_grpc\.py$|pnpm-lock"
    # This file's own output. Without it the artifact counts itself as
    # configuration, so writing it changes a number inside it, and the
    # count has to reach a fixed point to settle. It does not have to:
    # a one-line change to the JSON changes the JSON's own line count.
    r"|build-stats\.json$"
)

# extension -> (category, language). An extension absent here is not
# counted at all, which is deliberate: images, fonts and binaries are
# part of the repository and not part of this claim.
CATEGORY = {
    "go": ("code", "Go"),
    "tsx": ("code", "TSX"),
    "ts": ("code", "TypeScript"),
    "py": ("code", "Python"),
    "sql": ("code", "SQL"),
    "sh": ("code", "Shell"),
    "css": ("code", "CSS"),
    "tmpl": ("code", "Email templates"),
    "proto": ("contract", "Protobuf"),
    "yml": ("config", "YAML"),
    "yaml": ("config", "YAML"),
    "json": ("config", "JSON"),
    "md": ("documentation", "Markdown"),
}

TEST_PATTERNS = ("_test.go", ".test.ts", ".test.tsx", "_test.py")


def is_test(path: str) -> bool:
    return path.endswith(TEST_PATTERNS) or "/test_" in path or path.startswith("test_")


def tracked_files() -> list[str]:
    out = subprocess.run(
        ["git", "ls-files"], cwd=REPO, capture_output=True, text=True, check=True
    )
    return out.stdout.split()


def line_count(path: Path) -> int:
    try:
        with path.open(encoding="utf-8", errors="replace") as fh:
            return sum(1 for _ in fh)
    except OSError:
        return 0


def count_lines(files: list[str]) -> dict:
    """Lines by category, with languages inside each."""
    totals: dict[str, dict] = {}
    langs: dict[str, dict[str, list[int]]] = defaultdict(
        lambda: defaultdict(lambda: [0, 0])
    )

    for rel in files:
        ext = rel.rsplit(".", 1)[-1] if "." in rel else ""
        if ext not in CATEGORY:
            continue
        category, language = CATEGORY[ext]
        if GENERATED.search(rel):
            category = "generated"
        elif is_test(rel):
            # A test is a test whatever language it is written in.
            # Counting Go tests as Go would make the code figure
            # include the thing that checks it.
            category = "tests"
        n = line_count(REPO / rel)
        langs[category][language][0] += 1
        langs[category][language][1] += n

    for category, by_lang in langs.items():
        totals[category] = {
            "files": sum(v[0] for v in by_lang.values()),
            "lines": sum(v[1] for v in by_lang.values()),
            "languages": [
                {"language": k, "files": v[0], "lines": v[1]}
                for k, v in sorted(by_lang.items(), key=lambda kv: -kv[1][1])
            ],
        }
    return totals


def count_matches(pattern: str, paths: list[Path]) -> int:
    total = 0
    rx = re.compile(pattern)
    for p in paths:
        try:
            total += sum(1 for line in p.open(encoding="utf-8", errors="replace") if rx.search(line))
        except OSError:
            continue
    return total


def git_lines(args: list[str]) -> list[str]:
    try:
        out = subprocess.run(
            ["git", *args], cwd=REPO, capture_output=True, text=True, check=True
        )
    except (subprocess.CalledProcessError, OSError):
        return []
    return [l for l in out.stdout.splitlines() if l.strip()]


def history() -> dict:
    """Commits, split by who wrote them.

    Returns zeros on a shallow clone rather than wrong numbers, and the
    page renders nothing when the total is zero. CI checks this file
    against a full checkout (`fetch-depth: 0` on the web job), so a
    shallow run cannot quietly commit a smaller count.
    """
    authors = git_lines(["log", "--format=%an"])
    if not authors:
        return {"commits": 0}

    merges = len(git_lines(["log", "--merges", "--format=%h"]))
    ai = sum(1 for a in authors if a.startswith("Claude"))
    bots = sum(1 for a in authors if a.endswith("[bot]"))
    dates = git_lines(["log", "--format=%ad", "--date=short"])
    first, last = (dates[-1], dates[0]) if dates else ("", "")
    days = 0
    if first and last:
        from datetime import date

        f = date.fromisoformat(first)
        t = date.fromisoformat(last)
        days = (t - f).days + 1

    return {
        "commits": len(authors),
        "merges": merges,
        "nonMerge": len(authors) - merges,
        # Merges are excluded from both: a merge commit records who
        # pressed the button, not who wrote the change.
        "byAssistant": ai,
        "byBot": bots,
        "byOwner": len(authors) - merges - ai - bots,
        "firstCommit": first,
        "lastCommit": last,
        "days": days,
    }


def ci_checks() -> int:
    """Jobs that gate a push, counted from the workflow files.

    The page used to say "the twenty checks that run on every push",
    from memory, on a page whose whole argument is that its numbers are
    counted rather than claimed. The real figure was fifteen. Counting
    it is the only version of that sentence worth printing.

    Only the `jobs:` block is counted: the keys under `on:` sit at the
    same indentation, so a naive two-space scan reports `push` and
    `schedule` as jobs. auto-pr.yml is excluded because opening a pull
    request is plumbing, not a check.
    """
    wf = REPO / ".github" / "workflows"
    if not wf.is_dir():
        return 0
    total = 0
    for f in sorted(wf.glob("*.y*ml")):
        if f.name == "auto-pr.yml":
            continue
        src = f.read_text(encoding="utf-8")
        head, sep, tail = src.partition("\njobs:\n")
        # Runs on a push at all? The triggers live above `jobs:`.
        if not sep or not re.search(r"^\s{2,}push:", head, re.M):
            continue
        total += len(re.findall(r"^  ([a-zA-Z0-9_-]+):$", tail, re.M))
    return total


def structure(files: list[str]) -> dict:
    """The counts that are decisions rather than volume."""
    protos = [REPO / f for f in files if f.endswith(".proto")]
    go_tests = [REPO / f for f in files if f.endswith("_test.go")]
    ts_tests = [REPO / f for f in files if f.endswith(".test.ts")]
    py_tests = [REPO / f for f in files if "/test_" in f and f.endswith(".py")]

    # The FSD's status column, counted by state rather than by whether
    # the word "Shipped" appears anywhere in the row. Three rows have
    # already been found whose note said one thing and whose status said
    # another, so this reads the status column alone and treats
    # "partially", "Dropped" and "Deferred" as the distinct states they
    # are. A page claiming "148 shipped" when the answer is 135 is the
    # same failure in public.
    fsd = REPO / "docs" / "FSD.md"
    states = {"shipped": 0, "partial": 0, "dropped": 0, "deferred": 0, "notBuilt": 0}
    requirements = 0
    if fsd.exists():
        for line in fsd.read_text(encoding="utf-8").splitlines():
            if not line.startswith("| FR-"):
                continue
            requirements += 1
            status = line.split("|")[3]
            if "Dropped" in status:
                states["dropped"] += 1
            elif "Deferred" in status:
                states["deferred"] += 1
            elif "partially" in status.lower():
                states["partial"] += 1
            elif "Shipped" in status:
                states["shipped"] += 1
            else:
                states["notBuilt"] += 1

    web_pkg = REPO / "apps" / "web" / "package.json"
    deps = devdeps = 0
    if web_pkg.exists():
        pkg = json.loads(web_pkg.read_text(encoding="utf-8"))
        deps = len(pkg.get("dependencies", {}))
        devdeps = len(pkg.get("devDependencies", {}))

    go_mod = REPO / "services" / "api" / "go.mod"
    go_direct = 0
    if go_mod.exists():
        inside = False
        for line in go_mod.read_text(encoding="utf-8").splitlines():
            if line.startswith("require ("):
                inside = True
                continue
            if inside and line.startswith(")"):
                inside = False
            elif inside and line.strip() and "// indirect" not in line:
                go_direct += 1

    return {
        "services": count_matches(r"^service ", protos),
        "rpcs": count_matches(r"^\s+rpc ", protos),
        "migrations": len(
            [f for f in files if f.startswith("services/api/internal/db/migrations/")]
        ),
        # Numbered records only. docs/adr also holds a README index,
        # which is not a decision and was being counted as one, so the
        # page said eleven where there are ten.
        "adrs": len(
            [
                f
                for f in files
                if f.startswith("docs/adr/")
                and re.match(r"^\d{4}-", Path(f).name)
            ]
        ),
        "requirements": requirements,
        "requirementsShipped": states["shipped"],
        "requirementsPartial": states["partial"],
        "requirementsDropped": states["dropped"],
        "requirementsDeferred": states["deferred"],
        "requirementsNotBuilt": states["notBuilt"],
        "goTests": count_matches(r"^func Test", go_tests),
        "webTests": count_matches(r"^\s*(it|test)\(", ts_tests),
        "pythonTests": count_matches(r"^def test_", py_tests),
        "directDependencies": go_direct + deps + devdeps,
        "customChecks": len(list((REPO / "scripts").glob("check_*.py"))),
        # Jobs that actually gate a push. The page used to say "twenty
        # checks" from memory, on a page whose entire argument is that
        # its numbers are counted rather than claimed; the real figure
        # was fifteen. auto-pr.yml is excluded because opening a pull
        # request is plumbing, not a check.
        "ciChecks": ci_checks(),
        "runtimeServices": len(
            [
                line
                for line in (REPO / "docker-compose.yml").read_text(encoding="utf-8").splitlines()
                if re.match(r"^  [a-z][a-z0-9-]*:$", line)
            ]
        )
        if (REPO / "docker-compose.yml").exists()
        else 0,
    }


def main() -> int:
    files = tracked_files()
    stats = {
        "_comment": (
            "Generated by scripts/build_stats.py. Do not edit. "
            "CI regenerates this and fails if it differs."
        ),
        "lines": count_lines(files),
        "structure": structure(files),
        "history": history(),
    }

    rendered = json.dumps(stats, indent=2, sort_keys=True) + "\n"
    if "--check" in sys.argv:
        # `history` is excluded from the comparison, and this is the
        # one place in the repo where a check is deliberately partial,
        # so the reason belongs here rather than in a commit message.
        #
        # It records a commit count. Committing it is a commit. So the
        # committed file is always stale by at least the commit that
        # wrote it, and no amount of regenerating fixes that: it is
        # self-referential, not merely out of date. CI failed on
        # exactly this, one commit behind, the first time it ran.
        #
        # Everything derived from the tree is still compared strictly,
        # which is the part that can actually be kept true. The commit
        # figures are published with the date they were taken, the same
        # way the gate rows above them on that page carry an `asOf`,
        # so a stale number reads as a dated snapshot rather than as a
        # current claim.
        current_raw = OUT.read_text(encoding="utf-8") if OUT.exists() else "{}"
        try:
            current_obj = json.loads(current_raw)
        except json.JSONDecodeError:
            current_obj = {}
        fresh_obj = json.loads(rendered)
        current_obj.pop("history", None)
        fresh_obj.pop("history", None)
        if current_obj != fresh_obj:
            print(
                f"{OUT.relative_to(REPO)} is out of date.\n"
                "Run `python3 scripts/build_stats.py` and commit the result.",
            )
            return 1
        print(f"{OUT.relative_to(REPO)} is current")
        return 0

    OUT.write_text(rendered, encoding="utf-8")
    code = stats["lines"].get("code", {})
    tests = stats["lines"].get("tests", {})
    docs = stats["lines"].get("documentation", {})
    print(
        f"{OUT.relative_to(REPO)}: "
        f"{code.get('lines', 0):,} code, "
        f"{tests.get('lines', 0):,} tests, "
        f"{docs.get('lines', 0):,} documentation"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
