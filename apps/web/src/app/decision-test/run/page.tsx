"use client";

import dynamic from "next/dynamic";

// The run screen's route. Outside the (site) group on purpose: no
// header, no footer, and no Ask Roger panel.
//
// The panel is the one that matters. It can open over a question, and a
// signed-in member would then be taking a different test from an
// anonymous visitor: two populations in different conditions, which
// makes the comparison between them worthless. It is absent here
// because it is not mounted, rather than hidden, since a hidden panel
// is still mounted and can still open.
//
// ssr: false, deliberately. The screen's first act is to read the
// session handed over in sessionStorage, which does not exist on the
// server. Rendering it server-side would mean emitting a placeholder
// and then replacing it, so the alternatives were a hydration mismatch
// or a second render pass that exists only to work around the first.
// There is nothing here worth server-rendering: the page has no content
// until the browser supplies the session.
const RunScreen = dynamic(() => import("./run-screen").then((m) => m.RunScreen), {
  ssr: false,
});

export default function RunPage() {
  return <RunScreen />;
}
