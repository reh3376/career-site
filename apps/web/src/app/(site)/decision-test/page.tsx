import type { Metadata } from "next";

import { Briefing } from "./briefing";

export const metadata: Metadata = {
  title: "The decision test",
  description:
    "A fifteen-minute test of what happens to judgement, and to confidence in it, as attention is loaded.",
  // Not indexed while collection is open. A participant who arrives via
  // search rather than via the invitation has not been briefed by the
  // person who asked them, and the sample is meant to be recruited.
  robots: { index: false, follow: false },
};

export default function DecisionTestPage() {
  return <Briefing />;
}
