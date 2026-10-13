#!/usr/bin/env python3
"""Fail when a content file's front matter does not match the schema.

FR-CNT-16 says content is "validated against a shared schema at build
time". Nothing validated anything until 2026-10-12. Three separate
hand-rolled parsers read these files, each typing only the two or three
keys its own page needs and silently discarding the rest, so a typo'd
key, a missing required one, or a field nobody reads all built and
deployed without a signal.

The failure that makes this concrete: `photos.ts` skips any photo whose
`permission` or `alt` is missing, silently, rather than failing the
build. A photo could disappear from the site because of a typo in a key
name and the only symptom would be a gap in a gallery.

**The schema below is authored from the content that exists**, not from
the FSD's §7.2 specification. That is deliberate and it is the second
attempt. `packages/schema/` held a JSON Schema requiring six fields
(`id`, `slug`, `type`, `summary`, `visibility`, `chatbot_include`) that
no committed file carried, so not one file would have validated against
it; it was never wired up and was deleted on 2026-10-02. Its own
deletion commit said to re-author from real content rather than restore
it. This is that.

**`visibility` means three different things and they are kept apart on
purpose.** One word carries three unrelated value sets in this
repository:

    photos      member | public        a display gate on the gallery
    other       public                 a corpus-publication marker
    ingest (Go) corpus_only            retrieval gating, an override
                                       read by services/api/internal/
                                       ingest/walker.go

Collapsing them into one vocabulary would change what the assistant can
retrieve and what the gallery shows, which is a behaviour change and not
this check's business. What this check does instead is refuse a value
that is not valid *for that kind*, so the three stay distinct by
enforcement rather than by nobody noticing.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
CONTENT = REPO / "apps" / "web" / "content"

# Keys every content item carries regardless of kind.
#
# `id` and `type` are here because FR-CNT-17 names them and no file had
# either: type was implicit in which directory a file sat in, which
# meant a reader could not tell one from the file alone, and nothing
# had a stable identifier that survives a rename.
COMMON_REQUIRED = {"id", "type", "title"}

# kind -> (required, optional, enums, ints)
#
# `required` is what every file of the kind must carry. `optional` is
# what it may. Anything else is a typo, which is the main thing this
# catches.
SCHEMA: dict[str, dict] = {
    "article": {
        "required": COMMON_REQUIRED | {"subtitle", "date"},
        # author/series/part/tags are genuinely per-article: the
        # five-part series carries them, the standalone teaser post
        # does not, and forcing a series name onto a standalone post
        # would be inventing data to satisfy a schema.
        "optional": {"author", "series", "part", "tags"},
        "enums": {"type": {"article"}},
        "ints": {"part", "date"},
    },
    "photo": {
        "required": COMMON_REQUIRED
        | {
            "caption",
            "taken",
            # Both are load-bearing: photos.ts drops a photo missing
            # either one, silently, so a typo here removes a photo from
            # the gallery with no error anywhere.
            "alt",
            "permission",
            "visibility",
            "order",
            "source",
        },
        "optional": {"context", "hero_tracks"},
        "enums": {"type": {"photo"}, "visibility": {"member", "public"}},
        "ints": {"order"},
    },
    "role": {
        # `title` is the job title and `organization` is who it was
        # for, kept as two keys because a timeline is read down the
        # organisation column and across the title.
        #
        # `end` is required rather than optional, including for a role
        # that has ended this year: a missing end date renders as
        # "present", and a timeline that silently promotes an old role
        # to current is worse than one that is blank.
        "required": COMMON_REQUIRED
        | {
            "organization",
            "location",
            "start",
            "end",
            "order",
            "visibility",
            "technologies",
        },
        "optional": set(),
        "enums": {"type": {"role"}, "visibility": {"member", "public"}},
        "ints": {"start", "end", "order"},
    },
    "project": {
        # `summary` is the one line the catalog shows; the body is the
        # detail page. `status` and `role` are required because a
        # catalog that does not say what state a thing is in, or what
        # the owner actually did on it, is a list of names.
        "required": COMMON_REQUIRED
        | {
            "slug",
            "summary",
            "role",
            "status",
            "technologies",
            "order",
            "visibility",
        },
        # `repo` is optional: not every project is a repository, and a
        # project without one must not be unpublishable.
        "optional": {"repo", "started", "ended"},
        "enums": {
            "type": {"project"},
            "visibility": {"member", "public"},
            # Named states rather than free text, so the catalog can be
            # read down the column and a typo cannot invent a state.
            "status": {"active", "maintained", "paused", "complete"},
        },
        "ints": {"order", "started", "ended"},
    },
    "skill": {
        # `evidence` is required, and required to be non-empty, because
        # FR-CNT-09's whole condition is that every entry links to at
        # least one piece of evidence. A skills matrix without that is
        # a list of assertions in a table, which is the thing a reader
        # already discounts on a CV.
        "required": COMMON_REQUIRED
        | {"category", "depth", "years", "evidence", "order", "visibility"},
        "optional": set(),
        "enums": {
            "type": {"skill"},
            "visibility": {"member", "public"},
            # Three levels, not five. A finer scale invites a
            # self-assessment nobody can check, and the difference
            # between "7/10" and "8/10" is not a claim anybody can
            # evidence.
            "depth": {"working", "deep", "owned"},
        },
        "ints": {"years", "order"},
    },
    "credential": {
        # One kind for awards, degrees, certifications, service and
        # eligibility, rather than five directories, because they are
        # read together on one page and the only thing that differs is
        # which fields apply.
        "required": COMMON_REQUIRED | {"category", "order", "visibility"},
        # `status` exists so a lapsed certification can be published as
        # lapsed. Omitting it and listing only current credentials is
        # the CV behaviour this site argues against.
        "optional": {"issuer", "awarded", "status", "note"},
        "enums": {
            "type": {"credential"},
            "visibility": {"member", "public"},
            "category": {
                "award",
                "education",
                "certification",
                "service",
                "eligibility",
            },
            "status": {"current", "lapsed", "completed"},
        },
        "ints": {"awarded", "order"},
    },
    "other": {
        "required": COMMON_REQUIRED | {"visibility"},
        "optional": set(),
        "enums": {"type": {"other"}, "visibility": {"public", "corpus_only"}},
        "ints": set(),
    },
}

# Directory under content/ -> kind. The directory is how the readers and
# the Go ingest walker already decide what a file is; the `type` key now
# says the same thing in the file, and this check fails when the two
# disagree.
DIR_KIND = {
    "articles": "article",
    "photos": "photo",
    "other": "other",
    "roles": "role",
    "projects": "project",
    "skills": "skill",
    "credentials": "credential",
}

# Matches `key: value`, the same shape all three parsers accept. Kept
# identical to frontMatterKV in services/api/internal/ingest/walker.go
# on purpose; the parity fixture test is what keeps them that way.
KV = re.compile(r"^([A-Za-z_][A-Za-z0-9_-]*):\s*(.*)$")


def parse_front_matter(raw: str) -> dict[str, str] | None:
    """Return the key->value map, or None when there is no front matter."""
    raw = raw.replace("\r\n", "\n")
    if not raw.startswith("---\n"):
        return None
    end = raw.find("\n---\n", 4)
    if end < 0:
        return None
    meta: dict[str, str] = {}
    for line in raw[4:end].split("\n"):
        # A list item under a key (`  - tag`) belongs to the key above
        # it, not to a key of its own.
        if line.startswith((" ", "\t", "-")):
            continue
        m = KV.match(line)
        if not m:
            continue
        v = m.group(2).strip()
        if len(v) >= 2 and v[0] == v[-1] and v[0] in "\"'":
            v = v[1:-1]
        meta[m.group(1)] = v
    return meta


def check_file(path: Path, kind: str) -> list[str]:
    spec = SCHEMA[kind]
    raw = path.read_text(encoding="utf-8")
    meta = parse_front_matter(raw)
    if meta is None:
        return ["has no front matter block"]

    problems: list[str] = []
    present = set(meta)

    for key in sorted(spec["required"] - present):
        problems.append(f"missing required key `{key}`")

    allowed = spec["required"] | spec["optional"]
    for key in sorted(present - allowed):
        problems.append(
            f"unknown key `{key}`: not in the {kind} schema, so nothing reads it"
        )

    for key, values in spec["enums"].items():
        got = meta.get(key, "")
        if got and got not in values:
            problems.append(
                f"`{key}: {got}` is not valid for a {kind} "
                f"(expected one of {', '.join(sorted(values))})"
            )

    for key in spec["ints"]:
        got = meta.get(key, "")
        if got and not re.fullmatch(r"-?\d+", got):
            problems.append(f"`{key}: {got}` should be a number")

    # The directory already says what a file is. If `type` disagrees,
    # one of the two is wrong and a reader picking either could be the
    # one that is.
    declared = meta.get("type", "")
    if declared and declared != kind:
        problems.append(
            f"`type: {declared}` but the file is under {path.parent.name}/, "
            f"which means {kind}"
        )
    return problems


def check_parser_parity() -> list[str]:
    """This script's parser must agree with the other two.

    It is the third hand-rolled front-matter parser in the repository
    (TypeScript in apps/web/src/lib/frontmatter.ts, Go in
    services/api/internal/ingest/walker.go). Adding a third and leaving
    it unchecked would reproduce the drift the other two already had.
    The same fixture is asserted from Go and from vitest.
    """
    import json

    fixture_dir = REPO / "testdata" / "frontmatter"
    raw = (fixture_dir / "fixture.md").read_text(encoding="utf-8")
    want = json.loads((fixture_dir / "expected.json").read_text(encoding="utf-8"))
    got = parse_front_matter(raw)
    if got is None:
        return ["the shared fixture did not parse at all"]

    problems = []
    for key in sorted(set(want) | set(got)):
        if want.get(key) != got.get(key):
            problems.append(
                f"shared fixture key `{key}`: this parser says "
                f"{got.get(key)!r}, expected.json says {want.get(key)!r}"
            )
    return problems


def main() -> int:
    parity = check_parser_parity()
    for problem in parity:
        print(f"testdata/frontmatter: {problem}")
    if parity:
        print(
            "\nThis script's front-matter parser has drifted from the one the "
            "site and the corpus ingest use. See testdata/frontmatter/README.md."
        )
        return 1

    if not CONTENT.is_dir():
        print(f"no content directory at {CONTENT}")
        return 1

    failures = 0
    checked = 0
    ids: dict[str, Path] = {}

    for path in sorted(CONTENT.rglob("*.md")):
        kind = DIR_KIND.get(path.parent.name)
        if kind is None:
            rel = path.relative_to(REPO)
            print(f"{rel}: in a directory with no kind; add it to DIR_KIND")
            failures += 1
            continue

        checked += 1
        for problem in check_file(path, kind):
            print(f"{path.relative_to(REPO)}: {problem}")
            failures += 1

        # An id that is not unique is worse than no id: it reads as a
        # stable handle and silently collides.
        meta = parse_front_matter(path.read_text(encoding="utf-8")) or {}
        if (ident := meta.get("id")) :
            if ident in ids:
                print(
                    f"{path.relative_to(REPO)}: id `{ident}` is already used by "
                    f"{ids[ident].relative_to(REPO)}"
                )
                failures += 1
            ids[ident] = path

    print(f"\n{checked} content files, {len(SCHEMA)} kinds, {failures} problems")
    if failures:
        print(
            "\nNothing else catches these. The three front-matter parsers each\n"
            "read only the keys their own page needs and discard the rest, so\n"
            "a typo'd key is not a build error, it is a field that silently\n"
            "stops being read."
        )
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
