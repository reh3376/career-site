# FR-PUB-05: rendering without JavaScript

**State as of 2026-10-12.** The landing page and the two legal pages
are genuine React Server Components with no client-side rendering
dependency, so their text, headings, and primary calls to action reach
a no-JS browser as real HTML: Next's `<Link>` always emits a plain
`<a href>`, so navigation and the landing page's three core actions
(JD review, book a meeting, Ask Roger) work with JavaScript off. What
degrades is narrower and more specific than "the chrome needs JS" would
suggest: concretely, a no-JS visitor on the landing page sees three
working link buttons but a "What is this?" explainer link next to each
that does nothing when clicked, since it opens a modal via `onClick`
with no href fallback; a visitor in OT mode sees a two-button mode
toggle that renders but is inert for the same reason; and a visitor on
any page sees a hamburger-menu button that cannot open its drawer.
None of these three is load-bearing for reading the landing or legal
pages, because the same destinations (register, sign in, the legal
pages themselves) are already reachable through plain links in the
header and footer, which are themselves ordinary server components.

## What the FSD requires

`docs/FSD.md:295`, FR-PUB-05:

> The site shall render acceptably with JavaScript disabled for the
> landing and legal pages.

Status column: "Could (partially **Shipped**)". Notes column, in full:

> Partially shipped. Pages are server-rendered so the landing and legal
> pages read without JavaScript; the mode toggle, the assistant panel
> and the action modals need it.

## What exists today

**The landing page is a server component that reads the visitor's mode
from a cookie and delegates to one of two further server
components.** `apps/web/src/app/(site)/page.tsx:1-45`: no `"use
client"`; it awaits `getUiMode()` and `getSessionUser()` server-side
and renders `<ItLanding>` or `<OtLanding>`. Neither
`apps/web/src/components/landing/it-landing.tsx` nor
`.../ot-landing.tsx` carries a `"use client"` directive (checked with
`grep -l "use client"` against both files: no match); both are server
components.

**The legal pages are plain server components with no interactivity at
all.** `apps/web/src/app/(site)/privacy/page.tsx` and
`.../terms/page.tsx` both carry no `"use client"` directive and no
client-side import; they are static text and a `Metadata` export.

**The landing page's primary calls to action are real `<a>` elements,
not JS-dependent controls.** `apps/web/src/components/landing/core-actions.tsx:1`
is `"use client"` (needed for the explainer-panel state below), but the
three destinations themselves render as `next/link`
(`core-actions.tsx:99-108`):

```tsx
<Link href={hrefFor(a, signedIn)} className={...}>
  {a.label}
</Link>
```

`next/link` always renders a real `<a href>` in the server-rendered
HTML, with or without hydration, so these three buttons (JD review,
book a meeting, Ask Roger) work as ordinary links without JavaScript,
falling back to a full page navigation.

**What is specifically inert on the landing page: the "What is this?"
explainer, a secondary disclosure, not the primary action.**
`core-actions.tsx:109-116`:

```tsx
<button type="button" onClick={() => setOpen(a.key)} aria-haspopup="dialog" ...>
  What is this?
</button>
```

This is a plain `<button>` with an `onClick` handler and no `href`
anywhere; without JavaScript, clicking it does nothing, and the
`role="dialog"` / `aria-modal="true"` panel it would open
(`AboutPanel`, `:161-170`) never renders. This matches the FSD note's
"action modals need it" claim exactly, and it is scoped precisely to
this one explainer, not to the three destinations themselves.

**The mode toggle is a `"use client"` component that renders inert
buttons without JS, and on the default IT landing page is not present
at all.** `apps/web/src/components/mode-toggle.tsx:1` is `"use
client"`; its two buttons are plain `<button onClick=...>` elements
(`:45-73`) with no form fallback. `grep -rl "ModeToggle"` across
`apps/web/src` finds it used in exactly three places:
`settings/page.tsx`, `hamburger-menu.tsx`, and
`components/landing/ot-landing.tsx:109`. It is not imported by
`it-landing.tsx`. So a first-time visitor (default IT mode, no cookie
set yet) has no mode-switch control on the landing page itself without
JavaScript; the only place to find one is inside the hamburger drawer,
which itself cannot open without JS (next point). A visitor already in
OT mode sees the toggle's two buttons render as part of the initial
HTML, but pressing either does nothing.

**The hamburger menu, the one place the mode toggle and most of the
navigation otherwise live, is entirely client-rendered and cannot open
without JS, but is redundant with always-visible links elsewhere.**
`apps/web/src/components/hamburger-menu.tsx:1` is `"use client"`; it
holds the full menu tree, grouped by section, with the open/close state
driven by `useState`/`useRef` (visible in the file's imports,
`:1-8`). Its trigger is rendered inside the always-present
`SiteHeader` (a server component, `site-header.tsx:1-20`, no `"use
client"`), so the header itself appears normally; only the drawer's
contents are unreachable without JS. This is mitigated, not solved, by
two facts checked directly: `SiteHeader` itself renders a plain
`<Link href="/register">` ("Request access") for anonymous visitors
(`site-header.tsx:97-99`), though it is hidden below the `sm` breakpoint
(`className="hidden ... sm:inline-block"`, `:97`); and `SiteFooter`
(`site-footer.tsx:1-5`, no `"use client"`) renders its own plain
`<Link href="/register">` (`site-footer.tsx:70`) with no responsive
hiding, so a mobile, no-JS visitor on the landing page or either legal
page still has one working, always-visible path to register, even
though the header's mobile CTA and the entire hamburger drawer are
unavailable to them.

**The assistant panel is member-gated and does not mount for the
anonymous or signed-out visitors this requirement is actually about.**
`apps/web/src/app/(site)/layout.tsx:29-51`: `AskPanel` renders only
`{member ? <AskPanel /> : null}` (`:48`), where `member` comes from
`getSessionUser()` resolved server-side (`:32-37`). A visitor reading
the landing page or a legal page without an account, the FR-PUB-05
audience, never has `AskPanel` mounted at all, with or without
JavaScript. A signed-in member who happens to open `/privacy` would
have it mounted (it is `"use client"`, confirmed via
`grep -l "use client" apps/web/src/components/ask/ask-panel.tsx`), and
it would be inert without JS, but this is a narrower case than the FSD
note's phrasing suggests.

**No `<noscript>` fallback exists anywhere.** `grep -rn "noscript"
apps/web/src --include="*.tsx"` returns nothing. A no-JS visitor is
never told anything is degraded; the dead "What is this?" buttons and
mode-toggle buttons simply do nothing when pressed, with no visible
difference from a working button until the click fails to do anything.

## What is missing

1. No `<noscript>` or equivalent notice anywhere tells a no-JS visitor
   that specific controls (the explainer panels, the mode toggle, the
   hamburger drawer) will not function.
2. The default IT-mode landing page has no mode-switch affordance
   reachable without JavaScript at all (OT mode's inline toggle at
   least renders, inertly; IT mode's does not render anywhere outside
   the client-only hamburger drawer).
3. The "What is this?" explainer buttons on the landing page have no
   non-JS fallback (e.g., a details/summary element, or linking to an
   anchor on a dedicated "how it works" page); `/how-ask-roger-works`
   already exists as a public route per `docs/FSD.md:296`'s
   `PUBLIC_PATHS` list, but `core-actions.tsx` does not link to it or
   its equivalents for JD review / meetings from the explainer buttons.

## Dependencies and constraints

- The three primary landing actions and all header/footer navigation
  already work without JS; fixing the remaining gaps is additive
  (fallback content or links), not a rework of what already degrades
  correctly.
- The mode toggle's own design is deliberately optimistic and
  client-only by construction (`mode-toggle.tsx:9-14`, comment: it
  flips `data-mode` on `<html>` before the server round-trip lands);
  any non-JS fallback would need a second code path (e.g. a `<form>`
  posting to the existing `setUiModeAction` server action) rather than
  reusing the current button markup, since the whole point of the
  current design is to avoid waiting on that round-trip when JS is
  available.
- The hamburger drawer's contents are not otherwise duplicated in
  static markup; a non-JS fallback for navigation specifically (as
  opposed to the register CTA, which already has a footer fallback)
  would mean either exposing a plain nav list outside the drawer or
  accepting that the drawer is a JS-only convenience layered on top of
  already-sufficient footer/header links.

## Decisions the owner must make

1. Whether "renders acceptably" is already satisfied by the current
   state (readable content, working primary CTAs, working footer
   navigation, with only secondary affordances degraded) or requires
   closing the three specific gaps above (noscript notice, an IT-mode
   toggle fallback, explainer fallback links).
2. If pursuing a toggle fallback: whether to add a `<form
   action={setUiModeAction}>` pair of submit buttons that work without
   JS (progressively enhanced by the existing optimistic client
   behavior when JS is present), since the server action it already
   calls (`setUiModeAction`) does not itself require a client
   component to invoke.
3. Whether the "What is this?" explainer content is worth exposing as
   plain links to `/how-ask-roger-works` (or new equivalents for JD
   review and meetings) for the no-JS case specifically, or whether
   that duplicates content better left to the modal alone.
4. Whether a `<noscript>` notice is wanted at all, given the owner's
   stated preference (per other FSD items in this cluster) for
   explicit, honest status over silent degradation.

## Task checklist

1. Decide scope per Decision 1; if the current state is accepted as
   sufficient, update `docs/FSD.md:295`'s status note to be precise
   about exactly what degrades (this document's "What exists today"
   section) rather than the current three-item list, which does not
   distinguish the gated, already-irrelevant `AskPanel` case from the
   two landing-page gaps that do matter.
2. If building a toggle fallback (Decision 2): add a server-renderable
   `<form>` pair invoking `setUiModeAction` directly, rendered
   alongside (not replacing) the existing `ModeToggle` client
   component, visible specifically when JS is unavailable, and confirm
   it also covers the IT-mode landing page, which has no toggle at all
   today.
3. If adding explainer fallbacks (Decision 3): link each "What is
   this?" button's label to a real anchor or page for the no-JS case,
   consistent with the model already used for `/how-ask-roger-works`.
4. If adding a `<noscript>` notice (Decision 4): place it in the shared
   `(site)/layout.tsx` so it covers landing and both legal pages
   without per-page duplication.
5. Verify live, with JavaScript disabled in a real browser: load `/`,
   `/privacy`, and `/terms`; confirm the three landing CTAs and all
   header/footer links navigate correctly (expected to already pass);
   confirm whichever gaps were chosen for remediation now behave as
   decided, rather than relying on reading the code alone.
6. Update `docs/FSD.md:295` to Shipped, or to a corrected partially-shipped
   note, once the live check in step 5 is done.

## What could not be verified

- This was verified by reading the component tree (`"use client"`
  directives, what each component renders, and what Next's `<Link>`
  guarantees), not by loading the live site in a browser with
  JavaScript actually disabled. No browser-based check was run in this
  pass.
- Whether any other client component on the landing or legal pages
  (beyond `CoreActions`/`CoreActionsOt`, `ModeToggle`, and
  `HamburgerMenu`) renders visible-but-inert controls without JS was
  not exhaustively checked; the search was scoped to components
  reachable from `page.tsx`, `it-landing.tsx`, `ot-landing.tsx`,
  `privacy/page.tsx`, `terms/page.tsx`, and the shared `(site)/layout.tsx`.
- Screen-reader or other assistive-technology behavior on the dead
  "What is this?" and mode-toggle buttons without JS (e.g., whether
  `aria-haspopup="dialog"` on a button that does nothing is actively
  misleading versus merely inert) was not evaluated.
