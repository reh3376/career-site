#!/usr/bin/env python3
"""Fail when CI tests on a runtime the images do not ship.

Nothing owns the relationship between a Dockerfile's base image and the
workflow `env:` that is supposed to test against it. They are edited by
different people for different reasons, and dependabot edits one of
them without ever seeing the other.

That is exactly what happened on 2026-09-29. Dependabot raised the
sidecar base from `python:3.12-slim-bookworm` to `python:3.14-slim-bookworm`
and the api base from `golang:1.26-alpine` to `golang:1.27-alpine`.
Both merged green. CI went on testing, linting and vulnerability
scanning against Python 3.12 and Go 1.26 for as long as it took someone
to notice by hand.

The risk is not theoretical for Python: 3.13 and 3.14 removed standard
library modules, so sidecar code that passes on 3.12 can fail inside
the container, and the suite would have said nothing. Embedding and
résumé rendering both live there.

Go is milder, because `go.mod` declares a language version and a newer
toolchain builds it forward, which is why the language version in
`go.mod` is deliberately not compared here. A 1.27-specific regression
would still go unseen, which is enough reason to pin the toolchain.

The check reads both sides and compares. It is intentionally dumb: no
version ranges, no compatibility rules, just "these two numbers should
be the same number, and here is where each came from".
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
WORKFLOWS = (".github/workflows/ci.yml", ".github/workflows/security.yml")

# Each entry: the Dockerfile, a pattern capturing its base version, and
# the workflow variable that must agree with it.
PAIRS = (
    ("services/api/Dockerfile", r"^FROM\s+golang:([0-9.]+)-", "GO_VERSION"),
    ("services/sidecar/Dockerfile", r"^FROM\s+python:([0-9.]+)-", "PYTHON_VERSION"),
    ("apps/web/Dockerfile", r"^FROM\s+node:([0-9.]+)-", "NODE_VERSION"),
)


def image_version(rel: str, pattern: str) -> tuple[str, str] | None:
    """The base version a Dockerfile pins, and where. Returns None when
    the file pins none, which is not an error: a distroless or scratch
    stage has no version to compare."""
    path = REPO / rel
    if not path.is_file():
        return None
    rx = re.compile(pattern, re.MULTILINE)
    found: set[str] = set()
    for m in rx.finditer(path.read_text(encoding="utf-8")):
        found.add(m.group(1))
    if not found:
        return None
    if len(found) > 1:
        # Multi-stage builds that disagree with themselves are their own
        # bug, and a worse one than drift against CI.
        print(f"{rel}: stages disagree on the base version: {', '.join(sorted(found))}")
        return ("", rel)
    return (found.pop(), rel)


def workflow_versions(rel: str) -> dict[str, str]:
    path = REPO / rel
    if not path.is_file():
        return {}
    out: dict[str, str] = {}
    for m in re.finditer(r'^\s{2}([A-Z_]+):\s*"([^"]+)"', path.read_text(encoding="utf-8"), re.MULTILINE):
        out[m.group(1)] = m.group(2)
    return out


def main() -> int:
    failures = 0
    checked = 0
    for dockerfile, pattern, var in PAIRS:
        got = image_version(dockerfile, pattern)
        if got is None:
            continue
        version, _ = got
        if not version:
            failures += 1
            continue
        for wf in WORKFLOWS:
            env = workflow_versions(wf)
            if var not in env:
                continue
            checked += 1
            if env[var] != version:
                failures += 1
                print(
                    f"{wf}: {var} is \"{env[var]}\" but {dockerfile} ships "
                    f"{version}. CI is testing a runtime the image does not use."
                )
            else:
                print(f"ok  {var} {version}  ({dockerfile} == {wf})")

    print(f"\n{checked} comparisons, {failures} mismatched")
    if failures:
        print(
            "\nA base image moved without the workflow that tests it. Update the\n"
            "env: block in the workflows above, or pin the image back. Leaving\n"
            "them apart means a runtime-specific failure ships without CI ever\n"
            "running against the runtime that fails."
        )
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
