import type { ReactNode } from "react";

import { AskPanel } from "@/components/ask/ask-panel";
import { SiteFooter } from "@/components/site-footer";
import { SiteHeader } from "@/components/site-header";
import { getSessionUser } from "@/lib/session-user";

// The site's chrome: header, footer, and the assistant panel.
//
// This lives in a route group rather than in the root layout because
// one screen must not have it. The decision test runs for fifteen
// timed minutes with a participant's attention under deliberate load,
// and the Ask Roger panel can open over the top of a question. A
// signed-in member and an anonymous visitor would then be taking
// different tests: one of them has a chat surface that can appear
// mid-decision. That is not a cosmetic difference, it is two
// populations in different experimental conditions, which makes any
// comparison between them worthless.
//
// Next.js always renders the root layout, so a nested route cannot
// remove chrome that the root draws. Hence this: the root keeps the
// document shell, and everything that wants the site around it sits in
// here. Route groups do not appear in URLs, so nothing moved as far as
// a visitor or a search engine is concerned.
//
// To add a page without the chrome, put it outside this group rather
// than hiding things with CSS. A hidden panel is still mounted and can
// still open.
export default async function SiteLayout({ children }: { children: ReactNode }) {
  // Whether to offer the assistant at all. A failure here must not take
  // the whole site down for a garnish, so it degrades to "no panel".
  let member = false;
  try {
    member = Boolean(await getSessionUser());
  } catch {
    member = false;
  }

  return (
    <>
      <SiteHeader />
      <main className="flex-1">{children}</main>
      <SiteFooter />
      {/* Mounted in the layout, not in each page, so the thread
          survives navigation (FR-CHAT-01). Rendered only for a
          signed-in member: an anonymous visitor cannot use the
          assistant and should not be offered it. */}
      {member ? <AskPanel /> : null}
    </>
  );
}
