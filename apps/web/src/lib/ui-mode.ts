import "server-only";

import { cookies } from "next/headers";

// The site ships two visual modes:
//   * "it" — the default. Standard web presentation ("blueprint editorial").
//   * "ot" — the HMI mode. The frontend re-renders parts of the surface
//            to feel like a plant HMI / SCADA screen (dark panels,
//            monospace type, tag-name labels, tile grid, status
//            chips). The switch is server-driven via a cookie so the
//            first paint is already correct — no client-side flash.
export type UiMode = "it" | "ot";

export const UI_MODE_COOKIE = "ui_mode";

// Cookie is intentionally NOT HttpOnly because the toggle control also
// updates the DOM optimistically before the server round-trip; keeping
// it readable by JS avoids a full-page flash on toggle. It's a UI
// preference, not a security boundary — leaking it does nothing.
export async function getUiMode(): Promise<UiMode> {
  const jar = await cookies();
  const raw = jar.get(UI_MODE_COOKIE)?.value;
  return raw === "ot" ? "ot" : "it";
}
