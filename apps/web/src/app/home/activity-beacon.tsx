"use client";

import { useEffect, useRef } from "react";

import { recordActivityAction } from "./actions";

// Fires a single `view` activity event for the current page after
// mount. React Strict Mode double-invokes effects in dev — we guard
// with a ref so we only call the server action once, and the server
// action passes `clientEventId` so a genuine retry still de-dupes
// at the API layer.
//
// Kept intentionally minimal: no dwell tracking yet (that needs a
// beforeunload/pagehide handler with `navigator.sendBeacon`, which
// server actions don't naturally support). Adding dwell is a
// follow-up when we introduce content pages beyond /home.
export function ActivityBeacon({ contentId }: { contentId: string }) {
  const fired = useRef(false);
  useEffect(() => {
    if (fired.current) return;
    fired.current = true;
    void recordActivityAction({
      kind: "KIND_VIEW",
      contentId,
      clientEventId: newEventId(),
    });
  }, [contentId]);
  return null;
}

// crypto.randomUUID is available in modern browsers; the fallback
// is a good-enough per-session id in the tiny slice of user agents
// without it (Safari <15.4 and equivalents).
function newEventId(): string {
  try {
    if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
      return crypto.randomUUID();
    }
  } catch {
    /* fall through */
  }
  return "ev-" + Date.now().toString(36) + "-" + Math.random().toString(36).slice(2, 10);
}
