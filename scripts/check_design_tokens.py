#!/usr/bin/env python3
"""Fail when a colour utility names a token nobody defined.

Tailwind drops a utility it does not recognise. It does not warn, it
does not fail the build, it emits no rule at all, and the element
simply renders without that property. So a typo in a colour class is
invisible until a person looks at the right element in the right mode
and notices something is wrong.

That has now happened twice on this site:

    bg-canvas        28 uses, --color-canvas never defined. The job
                     detail modal rendered with no background, which
                     read as a transparency bug.
    accent-strong    13 uses across 8 admin surfaces, all
                     hover:text-accent-strong. The real token is
                     --color-accent-hover, so those links had no hover
                     state at all for weeks.

Both were found by eye, one of them only because a modal was unreadable.

The check is deliberately narrow. It does not try to know every
Tailwind utility, because that list changes with the framework and a
check that guesses produces noise nobody reads. Instead it learns the
project's own palette from globals.css and only judges classes that
claim to be part of it:

  * every `--color-x` in globals.css is a defined token
  * the first segment of each token name is a family (ink, paper,
    accent, ...)
  * a class like `text-accent-strong` names family `accent`, so it is
    making a claim about this palette and must resolve
  * `text-sm`, `border-2`, `bg-[#fff]` name no family here and are
    left alone

That way a new token needs no change to this script, and a utility the
framework owns is never second-guessed.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
CSS = REPO / "apps" / "web" / "src" / "app" / "globals.css"
SRC = REPO / "apps" / "web" / "src"

# The utilities that take a colour. Gradient stops included because
# from-/via-/to- resolve against the same palette.
PREFIXES = (
    "text", "bg", "border", "ring", "outline", "divide", "fill",
    "stroke", "shadow", "accent", "caret", "decoration", "from", "via", "to",
)

TOKEN_RE = re.compile(r"--color-([a-z0-9-]+)")

# A class as it appears in JSX: optionally prefixed by variants
# (hover:, sm:, dark:, group-hover:, focus-visible:, ...) and possibly
# an opacity suffix (/50). The variants are stripped before matching.
CLASS_RE = re.compile(r"[A-Za-z0-9:_\-\[\]/.%#()]+")
VARIANT_RE = re.compile(r"^(?:[a-z-]+(?:\[[^\]]*\])?:)+")


def defined_tokens() -> set[str]:
    if not CSS.is_file():
        sys.exit(f"cannot read {CSS}")
    return set(TOKEN_RE.findall(CSS.read_text(encoding="utf-8")))


def families(tokens: set[str]) -> set[str]:
    """First segment of every token, which is what a class must claim
    in order to be judged at all."""
    return {t.split("-", 1)[0] for t in tokens}


def candidate_classes(text: str) -> set[str]:
    """Every whitespace-delimited token in the file that could be a
    class. Reading className attributes properly would need a JSX
    parser; over-collecting is safe because only strings claiming a
    known family are judged."""
    return set(CLASS_RE.findall(text))


def check_file(path: Path, tokens: set[str], fams: set[str]) -> list[tuple[str, str]]:
    problems: list[tuple[str, str]] = []
    seen: set[str] = set()
    for raw in candidate_classes(path.read_text(encoding="utf-8")):
        cls = VARIANT_RE.sub("", raw)
        cls = cls.split("/", 1)[0]  # drop an opacity suffix
        if "[" in cls or not cls:
            continue  # arbitrary value, the framework's business
        for p in PREFIXES:
            if not cls.startswith(p + "-"):
                continue
            name = cls[len(p) + 1:]
            if not name or name.split("-", 1)[0] not in fams:
                break  # names no family of ours; not our business
            if name not in tokens and raw not in seen:
                seen.add(raw)
                problems.append((raw, f"--color-{name}"))
            break
    return problems


def main() -> int:
    tokens = defined_tokens()
    fams = families(tokens)
    if not tokens:
        sys.exit(f"no --color-* tokens found in {CSS}; has the palette moved?")

    failures = 0
    for path in sorted(SRC.rglob("*")):
        if path.suffix not in {".tsx", ".ts", ".jsx", ".js", ".css"}:
            continue
        if "/gen/" in str(path):
            continue  # generated
        for cls, want in check_file(path, tokens, fams):
            rel = path.relative_to(REPO)
            print(f"{rel}: {cls} needs {want}, which globals.css does not define")
            failures += 1

    print(f"\n{len(tokens)} tokens, {len(fams)} families, {failures} unresolved")
    if failures:
        print(
            "\nTailwind emits no rule for an unknown utility, so each of these\n"
            "renders with the property simply missing. Define the token in\n"
            "globals.css or use one that exists."
        )
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
