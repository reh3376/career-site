#!/usr/bin/env python3
"""Fail when a link in the app points nowhere, or opens a tab unsafely.

FR-CNT-19 asks for two things: external links open in a new tab with
`rel="noopener"`, and CI checks link health. The first was true by
convention, repeated by hand across roughly fifteen files with nothing
enforcing the pairing. The second did not exist.

**What this checks, and why these two and not others.**

*Internal links resolve to a route.* A renamed or deleted page leaves
every link to it returning 404, and nothing else in the build notices:
Next.js does not verify that a `href="/x"` has a page behind it, and the
string is as valid as any other. This is the half worth having in CI
because it is deterministic, needs no network, and is exactly the
failure that a refactor introduces quietly.

*Every `target="_blank"` carries `rel` with `noopener`.* Without it the
opened page gets a `window.opener` handle back to this one. Modern
browsers default to `noopener` for `target=_blank`, so this is belt and
braces rather than a live hole, which is the honest reason to let a
checker hold it rather than ask people to remember.

**What this deliberately does not check: whether an external URL is
reachable.** That needs the network, and a check that fails because
GitHub rate-limited a CI runner is a check people learn to re-run until
it passes, which is worse than not having it. Reachability belongs in
`deploy/live-check.sh`, which already talks to the network and runs
against the real site, and that is where it lives.

**Content is checked too, and today contains no links at all** — not
one markdown link, internal or external, across all sixteen files. That
is worth stating because the requirement says "link health on content
changes", and a checker that silently matches nothing looks identical to
a checker that passes.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
APP = REPO / "apps" / "web" / "src" / "app"
SRC = REPO / "apps" / "web" / "src"
CONTENT = REPO / "apps" / "web" / "content"

# Paths served by something other than a page.tsx: route handlers, the
# public directory, and the API, which Caddy sends to the Go service.
# Each is a deliberate entry rather than a prefix match, so a typo in a
# link cannot be excused by a prefix somebody added years ago.
NON_PAGE_ROUTES = {
    "/robots.txt",
    "/sitemap.xml",
    "/icon.svg",
}


def routes() -> set[str]:
    """Every path the app router serves, from the file tree.

    Both `page.tsx` and `route.ts`: the app router serves a path from
    either, and looking only for pages reported /admin/decisions/export
    as broken when it is a route handler returning a CSV. A checker's
    first finding being wrong is how a checker gets switched off.

    Segments in parentheses are route groups and contribute nothing to
    the URL. A `[param]` segment matches any one segment, and is kept as
    a marker so matching can treat it as a wildcard.
    """
    out: set[str] = set()
    served = list(APP.rglob("page.tsx")) + list(APP.rglob("route.ts"))
    for page in served:
        rel = page.relative_to(APP).parent
        segs = [s for s in rel.parts if not (s.startswith("(") and s.endswith(")"))]
        out.add("/" + "/".join(segs) if segs else "/")
    return out


def matches(link: str, known: set[str]) -> bool:
    if link in known or link in NON_PAGE_ROUTES:
        return True
    want = [s for s in link.strip("/").split("/") if s]
    for route in known:
        have = [s for s in route.strip("/").split("/") if s]
        if len(have) != len(want):
            continue
        if all(h == w or (h.startswith("[") and h.endswith("]")) for h, w in zip(have, want)):
            return True
    return False


# href="/x" and the object form used by nav arrays, href: "/x".
INTERNAL = re.compile(r"""href\s*[:=]\s*["']\s*(/[^"'#?]*)["']""")
EXTERNAL = re.compile(r"""href\s*[:=]\s*["'](https?://[^"']+)["']""")
ANCHOR = re.compile(r"<a\s[^>]*>", re.S)


ROUTES_TS = SRC / "lib" / "public-routes.ts"


def indexable() -> list[str]:
    src = ROUTES_TS.read_text(encoding="utf-8")
    block = re.search(r"export const INDEXABLE_PATHS = \[(.*?)\] as const;", src, re.S)
    return re.findall(r'"([^"]+)"', block.group(1)) if block else []


def page_for(route: str) -> Path | None:
    """The page.tsx behind a route, looking through (group) directories."""
    want = route.strip("/")
    for p in APP.rglob("page.tsx"):
        parts = [x for x in p.parent.relative_to(APP).parts
                 if not (x.startswith("(") and x.endswith(")"))]
        if "/".join(parts) == want:
            return p
    return None


def canonicals_declared() -> list[str]:
    """Every indexable page must declare its own canonical and card.

    Next merges `metadata` shallowly, so a page that sets only a title
    and description inherits the root layout's `openGraph` object whole
    and its `alternates.canonical` of "/". Both failures are silent and
    invisible from inside the app: the page renders correctly, and only
    a crawler or a pasted link shows it. An audit on 2026-10-14 found
    six indexable pages in that state, each one asking Google to treat
    it as a duplicate of the homepage and each one previewing in
    LinkedIn with the homepage's thirty-year-career headline.

    `lib/site.ts` has `pageMetadata()` for this; the rule is here so
    that forgetting it fails a push rather than quietly costing the
    page its place in the index.
    """
    problems = []
    for route in indexable():
        if route == "/":
            continue  # the homepage, where the inherited canonical is right
        page = page_for(route)
        if page is None:
            problems.append(f"{route} is in INDEXABLE_PATHS but no page.tsx serves it")
            continue
        text = page.read_text(encoding="utf-8")
        if "pageMetadata(" in text or "generateMetadata" in text or "alternates" in text:
            continue
        problems.append(
            f"{page.relative_to(REPO)} serves {route}, which is in "
            "INDEXABLE_PATHS, but declares no canonical. It inherits the "
            'root layout\'s canonical of "/" and the homepage\'s share '
            "card. Wrap its metadata in pageMetadata({ ..., path: "
            f'"{route}" }}) from @/lib/site.'
        )
    return problems


def main() -> int:
    known = routes()
    if len(known) < 20:
        print(f"only {len(known)} routes found under {APP}; the walk is broken "
              "and this check would pass by matching nothing")
        return 1

    problems = 0
    internal_seen = 0
    external_seen = 0

    for path in sorted(SRC.rglob("*.tsx")):
        if "/gen/" in str(path):
            continue
        text = path.read_text(encoding="utf-8")
        rel = path.relative_to(REPO)

        for link in INTERNAL.findall(text):
            internal_seen += 1
            if not matches(link, known):
                print(f"{rel}: links to {link}, which no page.tsx serves")
                problems += 1

        external_seen += len(EXTERNAL.findall(text))

        # rel/target pairing, per anchor rather than per line, because
        # the attributes are usually on separate lines.
        for tag in ANCHOR.findall(text):
            if 'target="_blank"' not in tag and "target={'_blank'}" not in tag:
                continue
            m = re.search(r"""rel\s*=\s*["']([^"']*)["']""", tag)
            if not m or "noopener" not in m.group(1):
                flat = " ".join(tag.split())[:90]
                print(f"{rel}: target=\"_blank\" without rel=\"noopener\": {flat}")
                problems += 1

    # Content, which the requirement names explicitly.
    content_links = 0
    for path in sorted(CONTENT.rglob("*.md")):
        text = path.read_text(encoding="utf-8")
        for target in re.findall(r"\]\(([^)]+)\)", text):
            content_links += 1
            if target.startswith("/") and not matches(target.split("#")[0], known):
                print(f"{path.relative_to(REPO)}: links to {target}, which no page serves")
                problems += 1

    for msg in canonicals_declared():
        print(msg)
        problems += 1

    print(f"\n{len(known)} routes, {internal_seen} internal links, "
          f"{external_seen} external, {content_links} in content, "
          f"{len(indexable())} indexable pages checked for a canonical, "
          f"{problems} problems")
    if problems:
        print("\nNext.js does not check that a href has a page behind it, so a\n"
              "renamed route leaves every link to it returning 404 with nothing\n"
              "failing. Reachability of external URLs is checked separately, in\n"
              "deploy/live-check.sh, against the running site.")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
