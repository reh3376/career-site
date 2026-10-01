import { track } from "@/lib/events-client";

// The panel's open/used state, as an external store.
//
// sessionStorage is an external system, not React state, and reading it
// into state from an effect causes a cascading render on every mount.
// useSyncExternalStore is the API for exactly this shape: it reads the
// real value, it has a separate server snapshot so there is no
// hydration mismatch, and it re-reads when the store says something
// changed.
//
// sessionStorage rather than localStorage because the requirement is
// that the panel persists "within a session". A panel that springs open
// a week later on a shared machine is a surprise rather than a
// convenience.
//
// Every access is wrapped: storage throws outright in some privacy
// modes, and the panel has to work without memory rather than take the
// page down with it.

const OPEN_KEY = "ask-roger-open";
const EVER_KEY = "ask-roger-used";

type Snapshot = { open: boolean; everOpened: boolean };

const CLOSED: Snapshot = { open: false, everOpened: false };

// Cached because getSnapshot must return a stable reference when
// nothing has changed, or React re-renders forever.
let cached: Snapshot = CLOSED;

const listeners = new Set<() => void>();

function read(): Snapshot {
  try {
    const open = sessionStorage.getItem(OPEN_KEY) === "1";
    const everOpened = open || sessionStorage.getItem(EVER_KEY) === "1";
    if (open !== cached.open || everOpened !== cached.everOpened) {
      cached = { open, everOpened };
    }
  } catch {
    /* no memory available; whatever is cached stands */
  }
  return cached;
}

export function subscribeToPanel(cb: () => void): () => void {
  listeners.add(cb);
  return () => listeners.delete(cb);
}

export function getPanelSnapshot(): Snapshot {
  return read();
}

// The server has no sessionStorage and must render the closed state, or
// the markup it produces will not match the first client paint.
export function getPanelServerSnapshot(): Snapshot {
  return CLOSED;
}

export function setPanelOpen(open: boolean): void {
  // Opening is the top of the funnel. How many open the assistant and
  // never ask is the number that says whether the wait is the problem
  // or the invitation is, and it cannot be recovered from the answers,
  // because the people it describes produce none.
  if (open && !cached.open) {
    track("chat.opened", { surface: "panel" });
  }
  cached = { open, everOpened: cached.everOpened || open };
  try {
    sessionStorage.setItem(OPEN_KEY, open ? "1" : "0");
    if (open) sessionStorage.setItem(EVER_KEY, "1");
  } catch {
    /* the panel still works for this page without it */
  }
  for (const cb of listeners) cb();
}
