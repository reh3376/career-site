import { Suspense } from "react";
import type { Metadata } from "next";

import { OG_IMAGE } from "@/lib/site";
import { getTurnstileSiteKey } from "@/lib/turnstile";

import { Briefing } from "./briefing";

const TITLE = "The decision test";
const DESCRIPTION =
  "A fifteen-minute test of what happens to judgement, and to confidence in it, as attention is loaded.";

export const metadata: Metadata = {
  title: TITLE,
  description: DESCRIPTION,
  // Not indexed while collection is open. A participant who arrives via
  // search rather than via the invitation has not been briefed by the
  // person who asked them, and the sample is meant to be recruited.
  robots: { index: false, follow: false },
  // This page is recruited for by link, so the preview card a social
  // network builds from these tags is the first thing most participants
  // will see of it.
  //
  // Without this block the page inherited the root layout's Open Graph
  // tags, which describe the career rather than the test and set
  // `og:url` to the site root. A card offering a thirty-year career
  // summary, under an invitation to spend fifteen minutes on a test,
  // is an invitation a reader is right to distrust. `noindex` above
  // does not prevent this: it keeps the page out of search results,
  // while a social crawler fetches the page anyway to build its card.
  alternates: { canonical: "/decision-test" },
  openGraph: {
    type: "website",
    title: TITLE,
    description: DESCRIPTION,
    url: "/decision-test",
    // Named again rather than inherited: overriding `openGraph`
    // replaces the parent object entirely, so leaving this out drops
    // the image and the card degrades to a small text-only one.
    images: [OG_IMAGE],
  },
  twitter: {
    card: "summary_large_image",
    title: TITLE,
    description: DESCRIPTION,
    images: [OG_IMAGE.url],
  },
};

// Suspense because the briefing reads ?synthetic=1 with
// useSearchParams, which opts a page out of static prerendering unless
// the reader sits behind a boundary. There is nothing worth falling
// back to: the briefing is the page, and it renders in the same tick.
export default function DecisionTestPage() {
  return (
    <Suspense>
      <Briefing turnstileSiteKey={getTurnstileSiteKey()} />
    </Suspense>
  );
}
