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

import html
import re
import sys
import urllib.error
import urllib.request

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

BUTTON = re.compile(r"(?is)<button\b([^>]*)>(.*?)</button>")
FORM = re.compile(r"(?is)<form\b.*?</form>")
# The rule in the root layout that makes the marking mean anything.
NOSCRIPT_RULE = re.compile(
    r"(?is)<noscript>.*?\[data-requires-js\].*?display\s*:\s*none.*?</noscript>"
)


def label_of(inner: str) -> str:
    text = html.unescape(re.sub(r"(?s)<[^>]+>", " ", inner))
    return " ".join(text.split())[:60] or "(no visible text)"


def buttons_outside_forms(doc: str) -> list[tuple[str, str]]:
    """Every button that a form would not submit.

    Buttons inside a form are removed first rather than tested for
    type="submit", because a submit is the default type and a button
    written without one still submits.
    """
    stripped = FORM.sub(" ", doc)
    return [(m.group(1), label_of(m.group(2))) for m in BUTTON.finditer(stripped)]


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

        # Scripts hold template strings that look like markup.
        doc = re.sub(r"(?is)<script.*?</script>", " ", doc)

        # The marking only does anything because of one CSS rule in the
        # root layout. Without it, `data-requires-js` is an attribute
        # nothing reads, every marked control is visible again, and the
        # button check above still passes: the two have to be asserted
        # together or the mechanism can be deleted silently.
        if not NOSCRIPT_RULE.search(doc):
            print(
                f"{where}: the <noscript> rule that hides "
                "[data-requires-js] is missing from <head>, so marking a "
                "control hides nothing and every marked control is live "
                "again without JavaScript"
            )
            problems += 1

        if "data-requires-js" not in doc:
            print(
                f"{where}: nothing carries data-requires-js, which on this "
                "page means either the marking was removed or this is not "
                "the build that was meant to be checked"
            )
            problems += 1

        for attrs, label in buttons_outside_forms(doc):
            checked += 1
            if "data-requires-js" in attrs:
                continue
            print(
                f'{where}: the control "{label}" is a button outside a form '
                "and is not marked data-requires-js, so without JavaScript "
                "it renders normally and does nothing when pressed"
            )
            problems += 1

        anchors = len(re.findall(r"(?i)<a\b[^>]*href=", doc))
        if anchors < 5:
            print(
                f"{where}: only {anchors} links, which means navigation does "
                "not survive with scripting off"
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


if __name__ == "__main__":
    sys.exit(check(sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:3000"))
