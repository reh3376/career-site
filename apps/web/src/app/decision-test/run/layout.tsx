import type { Metadata } from "next";
import type { ReactNode } from "react";

// A layout purely to carry metadata.
//
// page.tsx has to be a client component, because it loads the run
// screen with ssr disabled, and a client component cannot export
// metadata. Without this the run screen would be the one page of the
// three that a crawler could index. It renders nothing without a
// session, so there is nothing to index, but a search result pointing
// at a blank screen is still a worse outcome than no search result.
export const metadata: Metadata = {
  title: "The decision test",
  robots: { index: false, follow: false },
};

export default function RunLayout({ children }: { children: ReactNode }) {
  return <>{children}</>;
}
