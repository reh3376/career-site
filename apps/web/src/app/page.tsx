import { ItLanding } from "@/components/landing/it-landing";
import { OtLanding } from "@/components/landing/ot-landing";
import { getUiMode } from "@/lib/ui-mode";

// Landing page. Reads the visitor's saved UI mode server-side and
// renders the matching landing surface. IT mode is the editorial
// default; OT mode re-renders the same content as a plant HMI
// overview screen. See ADR-0029 (frontend IT/OT dual mode).
export const dynamic = "force-dynamic";

export default async function HomePage() {
  const mode = await getUiMode();
  return mode === "ot" ? <OtLanding /> : <ItLanding />;
}
