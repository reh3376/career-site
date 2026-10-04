import type { Metadata } from "next";
import localFont from "next/font/local";
import type { ReactNode } from "react";

import { EventBeacon } from "@/components/event-beacon";
import { getUiMode } from "@/lib/ui-mode";
import { OG_IMAGE, SITE_NAME, SITE_ORIGIN } from "@/lib/site";

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

const DEFAULT_TITLE =
  "Roger Henley · Industrial automation, plant operations, applied AI";
const DEFAULT_DESCRIPTION =
  "Thirty years running regulated 24/7 industrial systems. Last eleven in distilled-spirits startups. Digital transformation, process optimization, automation & control, IT/OT convergence, applied AI.";

// This URL mostly reaches people as a link: in a résumé header, a
// LinkedIn post, a recruiter's email. Without these tags it rendered as
// bare text in every one of those places, with no title, no description
// and no image, which is the worst possible first impression for a page
// whose whole job is a first impression.
//
// metadataBase is what lets the relative image path above resolve to an
// absolute URL, which every card scraper requires.
export const metadata: Metadata = {
  metadataBase: new URL(SITE_ORIGIN),
  title: { default: DEFAULT_TITLE, template: "%s · Roger Henley" },
  description: DEFAULT_DESCRIPTION,
  alternates: { canonical: "/" },
  openGraph: {
    type: "website",
    siteName: SITE_NAME,
    title: DEFAULT_TITLE,
    description: DEFAULT_DESCRIPTION,
    url: SITE_ORIGIN,
    images: [OG_IMAGE],
  },
  twitter: {
    card: "summary_large_image",
    title: DEFAULT_TITLE,
    description: DEFAULT_DESCRIPTION,
    images: [OG_IMAGE.url],
  },
};

// The document shell, and nothing else.
//
// Header, footer and the assistant panel moved to the (site) route
// group on 2026-10-03. Next.js always renders this layout, so anything
// drawn here appears on every page with no way for a route to opt out,
// and the decision test needs a screen with no chrome at all: the Ask
// Roger panel opening over a timed question would put members and
// anonymous visitors in different experimental conditions. See
// (site)/layout.tsx.
//
// What stays here is what genuinely belongs to every page: the fonts,
// the mode attribute that has to be set before first paint, the global
// stylesheet, and the event beacon, which records page views across
// the whole site including screens without chrome.
export default async function RootLayout({ children }: { children: ReactNode }) {
  // The site ships two visual modes: `it` (default, editorial web) and
  // `ot` (HMI/SCADA feel). Rendering the cookie server-side sets the
  // `data-mode` attribute on <html> BEFORE first paint so there's no
  // flash of the wrong mode. Design tokens in globals.css switch off
  // this attribute; pages that want mode-specific structure branch on
  // getUiMode() themselves.
  const mode = await getUiMode();
  return (
    <html
      lang="en"
      data-mode={mode}
      className={`${fraunces.variable} ${interTight.variable} ${jetbrainsMono.variable}`}
    >
      <body className="flex min-h-screen flex-col bg-paper text-ink">
        {children}
        <EventBeacon />
      </body>
    </html>
  );
}
