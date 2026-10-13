"use client";

import { useSyncExternalStore } from "react";

// Is JavaScript actually running this control?
//
// False on the server and through hydration, true once React is
// driving the page in a browser. Components use it to keep a
// JavaScript-only behaviour (a modal, an optimistic DOM flip) off the
// path a no-JavaScript visitor takes, so the markup underneath stays
// the real behaviour rather than a decoration. FR-PUB-05.
//
// Written as an external-store snapshot rather than `useState` plus an
// effect, which is both what this actually is (a fact about the
// environment, not state React owns) and what the lint rules here
// allow: `react-hooks/set-state-in-effect` rejects the effect form.
// `hamburger-menu.tsx` reads localStorage the same way.
//
// Nothing ever changes, so `subscribe` has nothing to subscribe to and
// returns an unsubscribe that does nothing.
const noop = () => () => {};

export function useEnhanced(): boolean {
  return useSyncExternalStore(
    noop,
    () => true,
    () => false,
  );
}
