"use client";

import { useTransition } from "react";

import { setUiModeAction } from "@/app/actions/ui-mode";
import type { UiMode } from "@/lib/ui-mode";

// A segmented two-button control that flips the site between IT and
// OT modes. Optimistically flips the `data-mode` attribute on <html>
// so the token switch happens BEFORE the server round-trip lands,
// then submits the server action which persists the cookie and
// revalidates the layout. The optimistic flip is why the cookie is
// not HttpOnly (see lib/ui-mode.ts).
export function ModeToggle({ current }: { current: UiMode }) {
  const [pending, startTransition] = useTransition();

  const select = (mode: UiMode) => {
    if (pending || mode === current) return;
    // Optimistic DOM flip so the whole page re-tokens immediately.
    // If the server action fails, the next paint reconciles with the
    // real cookie state.
    if (typeof document !== "undefined") {
      document.documentElement.setAttribute("data-mode", mode);
      try {
        document.cookie = `ui_mode=${mode}; Path=/; Max-Age=${60 * 60 * 24 * 365}; SameSite=Lax`;
      } catch {
        // Cookie writes can throw in some privacy contexts; ignore.
      }
    }
    startTransition(async () => {
      const fd = new FormData();
      fd.set("mode", mode);
      await setUiModeAction(fd);
    });
  };

  return (
    <div
      role="group"
      aria-label="Display mode"
      className="inline-flex items-stretch border border-line-strong text-xs font-mono uppercase tracking-[0.14em]"
    >
      <button
        type="button"
        aria-pressed={current === "it"}
        onClick={() => select("it")}
        disabled={pending}
        className={
          "px-4 py-2 transition-colors " +
          (current === "it"
            ? "bg-accent text-white"
            : "bg-transparent text-ink-2 hover:text-accent")
        }
      >
        IT
      </button>
      <span aria-hidden="true" className="w-px bg-line-strong" />
      <button
        type="button"
        aria-pressed={current === "ot"}
        onClick={() => select("ot")}
        disabled={pending}
        className={
          "px-4 py-2 transition-colors " +
          (current === "ot"
            ? "bg-accent text-white"
            : "bg-transparent text-ink-2 hover:text-accent")
        }
      >
        OT
      </button>
    </div>
  );
}
