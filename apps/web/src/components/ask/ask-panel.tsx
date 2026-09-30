"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useSyncExternalStore } from "react";

import {
  getPanelServerSnapshot,
  getPanelSnapshot,
  setPanelOpen,
  subscribeToPanel,
} from "./panel-state";
import { Thread } from "./thread";

// The side panel, mounted once in the root layout.
//
// It lives there rather than in each page because FR-CHAT-01 asks for
// the panel state to survive navigation, and in the App Router a layout
// does not remount between routes while a page does. So the thread
// stays alive as the reader moves around the site, which is the whole
// point: a question asked on one page is still answered on the next.
//
// The thread is only mounted once the panel has been opened. Mounting
// it eagerly would open a conversation on the server for every page
// view by every signed-in member, most of whom never ask anything, and
// that row would be indistinguishable in the data from someone who did.
//
// Open state is kept in sessionStorage, not localStorage: "within a
// session" is what the requirement says, and a panel that springs open
// a week later on a shared machine is a surprise rather than a
// convenience.

export function AskPanel() {
  const pathname = usePathname();
  const { open, everOpened } = useSyncExternalStore(
    subscribeToPanel,
    getPanelSnapshot,
    getPanelServerSnapshot,
  );

  // Escape closes it, which is the one keyboard convention a docked
  // panel has to honour.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setPanelOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  // The full page is the same assistant at a bigger size. Two of it on
  // one screen is noise, and the panel's launcher competing with the
  // page it duplicates is worse.
  if (pathname?.startsWith("/ask")) return null;

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setPanelOpen(true)}
        aria-expanded={false}
        className="fixed right-4 bottom-4 z-40 flex items-center gap-2 border border-accent bg-canvas px-4 py-2.5 font-mono text-[11px] tracking-[0.12em] text-accent uppercase shadow-sm transition-colors hover:bg-accent hover:text-white"
      >
        Ask Roger
      </button>
    );
  }

  return (
    <aside
      aria-label="Ask Roger"
      className="fixed inset-x-0 bottom-0 z-40 flex h-[min(80vh,42rem)] flex-col border-t border-line-strong bg-canvas shadow-lg sm:inset-x-auto sm:right-4 sm:bottom-4 sm:w-[26rem] sm:border sm:shadow-xl"
    >
      <header className="flex shrink-0 items-baseline justify-between gap-3 border-b border-line px-4 py-3">
        <div>
          <p className="font-display text-base text-ink">Ask Roger</p>
          <p className="font-mono text-[11px] tracking-[0.1em] text-ink-3 uppercase">
            AI assistant ·{" "}
            <Link
              href="/how-ask-roger-works"
              className="underline underline-offset-2 hover:text-accent"
            >
              how this works
            </Link>
          </p>
        </div>
        <div className="flex items-center gap-3">
          <Link
            href="/ask"
            className="font-mono text-[11px] tracking-[0.12em] text-ink-3 uppercase underline underline-offset-2 hover:text-accent"
          >
            Full page
          </Link>
          <button
            type="button"
            onClick={() => setPanelOpen(false)}
            aria-label="Close Ask Roger"
            className="font-mono text-[11px] tracking-[0.12em] text-ink-3 uppercase underline underline-offset-2 hover:text-accent"
          >
            Close
          </button>
        </div>
      </header>

      <div className="min-h-0 flex-1 px-4 pb-4">
        {everOpened ? <Thread /> : null}
      </div>
    </aside>
  );
}
