import type { Metadata } from "next";
import { Fraunces, Inter_Tight, JetBrains_Mono } from "next/font/google";
import type { ReactNode } from "react";

import { SiteFooter } from "@/components/site-footer";
import { SiteHeader } from "@/components/site-header";
import { getUiMode } from "@/lib/ui-mode";

import "./globals.css";

// Fraunces, variable serif for display. Distinctive letterforms
// (soft-square terminals, wonky ampersand, opsz variation) that read as
// crafted, not templated.
const fraunces = Fraunces({
  subsets: ["latin"],
  // Variable weight is required for next/font to accept custom axes.
  // The opsz axis drives display-vs-text optical sizing (we set 60–144
  // via font-variation-settings at the callsites); SOFT gives the
  // headlines a softer terminal that reads as crafted, not templated.
  axes: ["opsz", "SOFT"],
  style: ["normal", "italic"],
  variable: "--font-fraunces",
  display: "swap",
});

// Inter Tight, slightly condensed Inter. Precise, humanist, disappears
// when needed; the small horizontal savings vs. regular Inter give body
// text more breathing room on narrow columns.
const interTight = Inter_Tight({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-inter-tight",
  display: "swap",
});

// JetBrains Mono, reserved for numeric callouts, mono tags, and the
// system-status indicator. Signals the industrial-control aesthetic
// without hijacking body prose.
const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  variable: "--font-jetbrains-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: {
    default: "Roger Henley · Industrial automation, plant operations, applied AI",
    template: "%s · Roger Henley",
  },
  description:
    "Thirty years running regulated 24/7 industrial systems. Last eight in bourbon distillery startups. Digital transformation, process optimization, automation & control, IT/OT convergence, applied AI.",
};

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
        <SiteHeader />
        <main className="flex-1">{children}</main>
        <SiteFooter />
      </body>
    </html>
  );
}
