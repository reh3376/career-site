# ADR 0028 — Frontend design system: blueprint editorial

**Status:** Accepted, 2026-09-19 (owner decision). Resolves the "UI is very basic and not aesthetically pleasing" feedback that landed after the first-cut approval-gated auth flow reached production.

## Context

The first version of the site was correct — approval-gated registration worked end-to-end, the auth loop was tested, CI was green, prod was reachable — but it looked like a Tailwind starter. Owner feedback (verbatim): *"the UI is very basic and not aesthetically pleasing at all. You need to make major improvements to the frontend UI."* Follow-up: *"there are 20 or 30 images that can be used in the UI that are specifically related to work. I do not want to make it appear that I only work… create a visually appealing UI that presents me as a professional with a personal life."*

The site is public and represents a career practice. It needs a distinctive visual identity that (a) reads as intentional, not templated; (b) reflects Roger's actual work (regulated 24/7 industrial systems, distilleries, digital transformation) rather than generic "modern SaaS"; and (c) balances professional presence with human context.

## Decision

A single design system, checked in as CSS tokens (`apps/web/src/app/globals.css`) + `next/font` imports (`apps/web/src/app/layout.tsx`), applied across every page and both shared components (`site-header`, `site-footer`).

**Palette — cool paper + blueprint blue + safety orange.**

- Paper: `#f6f6f4` / `#eeede8` / `#e5e4de` (cool off-white, not warm cream). Deliberately avoids the *"warm cream + terracotta"* palette that has become the AI-generated-page tell.
- Ink: `#111417` / `#3a3f45` / `#7c828a` (near-black through greys).
- Accent — **blueprint teal** `#164e63`. Reads as technical / control-screen / drafting-blue. Used for links, buttons, hover states, one italic display accent.
- Signal — **safety orange** `#c2410c`. Reserved for the "system · nominal" live indicator and mono status kickers. Never for buttons or general emphasis.

**Type — three faces, distinct roles.**

- **Fraunces** (variable serif, with `opsz` and `SOFT` axes) — display type. Loaded via `next/font/google` as a variable font so per-headline optical sizing and softness are tuned inline via `font-variation-settings`. Carries the personality; one italic weight is used as a single accent on the hero.
- **Inter Tight** — body copy. Slightly condensed vs regular Inter; buys back column width on narrow layouts.
- **JetBrains Mono** — numeric callouts, mono status labels, colophon `dl` rows. The industrial-control aesthetic without hijacking body prose.

**Layout — editorial, not card-grid.**

- One column of type, image bands that break the reading rhythm.
- The five practice areas (`OFFERINGS`) render as a rule-divided list with the offering name in the display face on the left and the description on the right — deliberately *not* five identical rounded cards.
- Real numbers (`30yr`, `8yr`, `24/7`) are set in the display face at hero size and treated as design elements, not stats-row filler.
- Images alternate between industrial work (copper condenser, Hobet dragline) and human context (Roger at a University of Kentucky podium, home workshop with the dog visible, Whiskey House team meeting). Sourced from private originals under `docs/personal/images/` (gitignored), EXIF-stripped and re-encoded before publication into `apps/web/public/images/`.

**Motion — one moment.**

- A single pulsing dot behind the header's `system · nominal` indicator (2.4s ease-in-out cycle, `prefers-reduced-motion` disables). No section fade-ins, no scroll-triggered reveals, no hover jumps on cards — those are the AI defaults this system rejects.

**Forms — underlined, on paper.**

- Contact / register / login inputs use a single hairline bottom rule with an accent-color flip on focus. No boxed inputs, no shadows, no rounded corners on fields. Reads like a form filled out on a clipboard, closer to Roger's actual work than a SaaS admin panel.

## Consequences

- **Brand consistency is enforced by tokens.** A future rebrand rewrites one `@theme` block in `globals.css` and every page updates.
- **Docs discipline (per ADR-0027) requires a design ADR for further changes to these tokens** — palette shifts, font-family swaps, or motion additions must land with an updated ADR entry (this one or its successor). Small in-page style tweaks do not.
- **The design has a stated identity: "blueprint editorial."** New pages / features that need visual choices should reach for the same tokens (`bg-paper`, `text-ink`, `text-accent`, `font-display`, `font-mono`, kicker + rule-plot patterns), not invent their own.
- **`next/font` requires Fraunces to load as a true variable font** (no explicit `weight` array) so the `opsz` and `SOFT` axes are accepted. Documented in `apps/web/src/app/layout.tsx`.
- **Images under `apps/web/public/images/` are EXIF-stripped and downsized** at ingest via a one-off Python + Pillow script kept in the session scratchpad. If more images ship, add them the same way — a light `make images` target is the natural next step but is not blocking today.
- **This ADR does not close the topic.** A Phase 2 gallery, a projects surface, and the `/home` member dashboard will all extend the system — each with its own follow-up ADR if the visual language changes materially.
