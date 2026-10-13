#!/usr/bin/env python3
"""Fail when a page offers a control that does nothing without JavaScript.

FR-PUB-05: "The site shall render acceptably with JavaScript disabled
for the landing and legal pages."

**What "acceptably" was taken to mean, because the row does not say.**
A visitor with JavaScript off should be offered only controls that
work. Text and navigation already survived, because the landing and
legal pages are server components and `next/link` emits a real
`<a href>` either way. What did not survive were four controls that
render identically to working ones and do nothing when pressed: three
"What is this?" explainers and the hamburger trigger. A dead control is
worse than an absent one, because the visitor cannot tell which it is
until they press it.

**What this checks.** Every `<button>` in the server-rendered markup of
the pages named below must be one of:

- a submit inside a `<form>`, which posts without JavaScript (Next
  server actions are progressively enhanced, so this really works);
- marked `data-requires-js`, which the `<noscript>` rule in the root
  layout hides when there is no JavaScript to run it.

A `<summary>` is not a button and needs no marking: `<details>` is a
native disclosure that works with scripting off.

**Why a source scan and not a rendered-page scan.** This reads the
built HTML from a running server, not the JSX, because the question is
what actually reaches the browser. A component can look fine in source
and still render a bare button through three layers of composition.

**How to run it.** Against a built app:

    pnpm --dir apps/web build && pnpm --dir apps/web start &
    python3 scripts/check_nojs.py http://127.0.0.1:3000

CI runs it against its own build. `deploy/live-check.sh` runs the same
assertions against production, which is the one that proves it for
real visitors rather than for a build.
"""

from __future__ import annotations

import re
import sys
import urllib.error
import urllib.request
from html.parser import HTMLParser

# The pages FR-PUB-05 names. Listed explicitly rather than discovered,
# so adding one is a deliberate decision about whether this requirement
# covers it.
#
# `/` appears twice because it is two different pages. The site ships
# an editorial mode and an HMI mode, chosen by the `ui_mode` cookie and
# rendered by two separate component trees, and the HMI one carries
# controls the editorial one does not: a first pass that checked only
# the default missed four of them.
PAGES = [
    ("/", None),
    ("/", "ui_mode=ot"),
    ("/privacy", None),
    ("/terms", None),
]

# Parsed, not pattern-matched.
#
# The first two versions of this file scanned with regular expressions
# and CodeQL rejected both, correctly. `</script>` missed `</script >`,
# which HTML allows; `<button[^>]*>` then missed `<button data-x="a>b">`,
# because an attribute value may contain the character the pattern uses
# as its terminator. Each fix revealed the next case, which is the
# signal to stop: matching HTML with regular expressions is a class of
# bug, not one bug, and the standard library already ships a parser that
# handles all of it.
#
# HTMLParser also solves the script problem outright. It reads `script`
# and `style` content as raw character data rather than markup, so a
# `<button>` written inside a JavaScript string is never seen as a tag
# and cannot be reported as a control that is not on the page.


class PageScan(HTMLParser):
    """What the served markup offers a visitor, in the terms this check needs.

    Buttons are recorded with the form depth they were found at, rather
    than filtered as they are seen: a button inside a form submits
    without JavaScript whatever its `type`, because submit is the
    default, so nesting is the question and `type` is not.
    """

    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self.form_depth = 0
        self.noscript_depth = 0
        self._style_in_noscript = False
        self.noscript_css: list[str] = []
        self.buttons: list[tuple[dict[str, str | None], str, int]] = []
        self.marked = 0
        self.anchors = 0
        self._open_button: tuple[dict[str, str | None], list[str], int] | None = None

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        a = dict(attrs)
        if "data-requires-js" in a:
            self.marked += 1
        if tag == "form":
            self.form_depth += 1
        elif tag == "noscript":
            self.noscript_depth += 1
        elif tag == "style" and self.noscript_depth:
            self._style_in_noscript = True
        elif tag == "a" and "href" in a:
            self.anchors += 1
        elif tag == "button":
            self._open_button = (a, [], self.form_depth)

    def handle_endtag(self, tag: str) -> None:
        if tag == "form" and self.form_depth:
            self.form_depth -= 1
        elif tag == "noscript" and self.noscript_depth:
            self.noscript_depth -= 1
        elif tag == "style":
            self._style_in_noscript = False
        elif tag == "button" and self._open_button is not None:
            attrs, text, depth = self._open_button
            label = " ".join("".join(text).split())[:60] or "(no visible text)"
            self.buttons.append((attrs, label, depth))
            self._open_button = None

    def handle_data(self, data: str) -> None:
        if self._open_button is not None:
            self._open_button[1].append(data)
        if self._style_in_noscript:
            self.noscript_css.append(data)


def scan(doc: str) -> PageScan:
    s = PageScan()
    s.feed(doc)
    s.close()
    return s


def buttons_outside_forms(doc: str) -> list[tuple[dict[str, str | None], str]]:
    """Every button that no form would submit."""
    return [(a, label) for a, label, depth in scan(doc).buttons if depth == 0]


def hides_marked_controls(page: PageScan) -> bool:
    """Is the one rule that makes `data-requires-js` mean anything present?"""
    css = " ".join(page.noscript_css)
    return "[data-requires-js]" in css and NO_DISPLAY.search(css) is not None


NO_DISPLAY = re.compile(r"display\s*:\s*none", re.I)


def check(base: str) -> int:
    problems = 0
    checked = 0

    for path, cookie in PAGES:
        where = path if cookie is None else f"{path} ({cookie})"
        req = urllib.request.Request(base.rstrip("/") + path)
        if cookie:
            req.add_header("Cookie", cookie)
        try:
            with urllib.request.urlopen(req, timeout=30) as r:  # noqa: S310
                doc = r.read().decode("utf-8", "replace")
        except urllib.error.URLError as e:
            print(f"{where}: could not be fetched: {e}")
            return 1

        page = scan(doc)

        # The marking only does anything because of one CSS rule in the
        # root layout. Without it, `data-requires-js` is an attribute
        # nothing reads, every marked control is visible again, and the
        # button check below still passes: the two have to be asserted
        # together or the mechanism can be deleted silently.
        if not hides_marked_controls(page):
            print(
                f"{where}: the <noscript> rule that hides "
                "[data-requires-js] is missing from <head>, so marking a "
                "control hides nothing and every marked control is live "
                "again without JavaScript"
            )
            problems += 1

        if not page.marked:
            print(
                f"{where}: nothing carries data-requires-js, which on this "
                "page means either the marking was removed or this is not "
                "the build that was meant to be checked"
            )
            problems += 1

        for attrs, label, depth in page.buttons:
            if depth:
                continue
            checked += 1
            if "data-requires-js" in attrs:
                continue
            print(
                f'{where}: the control "{label}" is a button outside a form '
                "and is not marked data-requires-js, so without JavaScript "
                "it renders normally and does nothing when pressed"
            )
            problems += 1

        if page.anchors < 5:
            print(
                f"{where}: only {page.anchors} links, which means navigation "
                "does not survive with scripting off"
            )
            problems += 1

    print(
        f"\n{len(PAGES)} pages, {checked} buttons outside a form examined, "
        f"{problems} problems"
    )
    if problems:
        print(
            "\nFR-PUB-05. A control that renders like any other and does\n"
            "nothing when pressed is worse than one that is absent: the\n"
            "visitor cannot tell the difference until they press it. Give\n"
            "it a no-JavaScript path (a form, or a <details> disclosure),\n"
            "or mark it data-requires-js so it is hidden when there is no\n"
            "JavaScript to run it."
        )
        return 1
    return 0


def self_test() -> int:
    """Guard the tag patterns against the whitespace they must tolerate.

    Three cases, each of which a regular-expression scanner got wrong
    before this file was rewritten around HTMLParser, and each of which
    invents or hides a control:

    - `</script >`, which the first script filter did not match, so the
      script's string contents were scanned as markup;
    - `</form >` and `</button >`, which left a form's submit buttons
      looking loose;
    - an attribute value containing `>`, which `[^>]*` terminates on.

    A checker that reports a control nobody can see is a checker that
    gets switched off, so the shapes are pinned here rather than left
    for the next person to rediscover.
    """
    doc = (
        '<script >var x = "<button>ghost</button>";</script >'
        '<form><button name="mode">IT</button ></form >'
        '<button data-x="a>b">loose</button >'
    )
    labels = [label for _, label in buttons_outside_forms(doc)]
    if labels != ["loose"]:
        print(f"self-test failed: expected ['loose'], got {labels}")
        return 1

    page = scan(
        "<noscript ><style>[data-requires-js]{display:none}</style></noscript >"
    )
    if not hides_marked_controls(page):
        print("self-test failed: the noscript rule was not recognised")
        return 1

    print("self-test ok: spaced tags, attribute values containing '>', "
          "and buttons inside script strings")
    return 0


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--self-test":
        sys.exit(self_test())
    sys.exit(check(sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:3000"))
