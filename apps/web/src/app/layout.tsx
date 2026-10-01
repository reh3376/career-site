import type { Metadata } from "next";
import localFont from "next/font/local";
import type { ReactNode } from "react";

import { EventBeacon } from "@/components/event-beacon";
import { SiteFooter } from "@/components/site-footer";
import { AskPanel } from "@/components/ask/ask-panel";
import { SiteHeader } from "@/components/site-header";
import { getUiMode } from "@/lib/ui-mode";
import { getSessionUser } from "@/lib/session-user";

import "./globals.css";

// The three faces are self-hosted from src/fonts (variable WOFF2, latin
// subset, SIL Open Font License) so a build never depends on
// fonts.googleapis.com: two builds in one afternoon failed when
// Turbopack could not fetch the Google Fonts CSS. Same families, same
// axes, same CSS variables as before; the callsites are unchanged.

// Fraunces, variable serif for display. Distinctive letterforms
// (soft-square terminals, wonky ampersand, opsz variation) that read as
// crafted, not templated. The file carries the opsz and SOFT axes; the
// callsites set them with font-variation-settings.
const fraunces = localFont({
  src: [
    { path: "../fonts/fraunces-latin.woff2", style: "normal" },
    { path: "../fonts/fraunces-italic-latin.woff2", style: "italic" },
  ],
  weight: "100 900",
  variable: "--font-fraunces",
  display: "swap",
});

// Inter Tight, slightly condensed Inter. Precise, humanist, disappears
// when needed; the small horizontal savings vs. regular Inter give body
// text more breathing room on narrow columns.
const interTight = localFont({
  src: "../fonts/inter-tight-latin.woff2",
  weight: "100 900",
  variable: "--font-inter-tight",
  display: "swap",
});

// JetBrains Mono, reserved for numeric callouts, mono tags, and the
// system-status indicator. Signals the industrial-control aesthetic
// without hijacking body prose.
const jetbrainsMono = localFont({
  src: "../fonts/jetbrains-mono-latin.woff2",
  weight: "100 800",
  variable: "--font-jetbrains-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: {
    default: "Roger Henley · Industrial automation, plant operations, applied AI",
    template: "%s · Roger Henley",
  },
  description:
    "Thirty years running regulated 24/7 industrial systems. Last eleven in distilled-spirits startups. Digital transformation, process optimization, automation & control, IT/OT convergence, applied AI.",
};

export default async function RootLayout({ children }: { children: ReactNode }) {
  // The site ships two visual modes: `it` (default, editorial web) and
  // `ot` (HMI/SCADA feel). Rendering the cookie server-side sets the
  // `data-mode` attribute on <html> BEFORE first paint so there's no
  // flash of the wrong mode. Design tokens in globals.css switch off
  // this attribute; pages that want mode-specific structure branch on
  // getUiMode() themselves.
  const mode = await getUiMode();
  // Whether to offer the assistant at all. A failure here must not take
  // the whole site down for a garnish, so it degrades to "no panel".
  let member = false;
  try {
    member = Boolean(await getSessionUser());
  } catch {
    member = false;
  }
  return (
    <html
      lang="en"
      data-mode={mode}
      className={`${fraunces.variable} ${interTight.variable} ${jetbrainsMono.variable}`}
    >
      <body className="flex min-h-screen flex-col bg-paper text-ink">
        <SiteHeader />
        <main className="flex-1">{children}</main>
        <SiteFooter />
        {/* Mounted in the layout, not in each page, so the thread
            survives navigation (FR-CHAT-01). Rendered only for a
            signed-in member: an anonymous visitor cannot use the
            assistant and should not be offered it. */}
        {member ? <AskPanel /> : null}
        <EventBeacon />
      </body>
    </html>
  );
}
