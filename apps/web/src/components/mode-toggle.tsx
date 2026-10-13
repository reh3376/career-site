"use client";

import { useTransition } from "react";

import { setUiModeAction } from "@/app/actions/ui-mode";
import { track } from "@/lib/events-client";
import { useEnhanced } from "@/lib/use-enhanced";
import type { UiMode } from "@/lib/ui-mode";

// A segmented two-button control that flips the site between IT and
// OT modes. Optimistically flips the `data-mode` attribute on <html>
// so the token switch happens BEFORE the server round-trip lands,
// then submits the server action which persists the cookie and
// revalidates the layout. The optimistic flip is why the cookie is
// not HttpOnly (see lib/ui-mode.ts).
//
// The markup underneath is a plain <form> posting to that same server
// action, with the two buttons as submits. With JavaScript the click
// handler takes over and the form never submits; without it the form
// posts, the server sets the cookie, and the page comes back in the
// chosen mode. One code path for the action, two for the trigger,
// which is what progressive enhancement costs here. FR-PUB-05.
export function ModeToggle({ current }: { current: UiMode }) {
  const [pending, startTransition] = useTransition();
  // False through hydration, true once the browser is running the
  // handlers below. Until then the form submit is the real behaviour.
  const enhanced = useEnhanced();

  const select = (mode: UiMode) => {
    if (pending || mode === current) return;
    track("landing.mode_switch", { from: current, to: mode }, { flush: true });
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

  // Click wins when there is JavaScript; the form submit is what
  // happens when there is not. Returning early without preventDefault
  // is the no-JavaScript path, and swallowing the event there would
  // make the control dead rather than merely slower.
  //
  // Written inline per button rather than as one curried handler
  // because `react-hooks/immutability` reads a curried handler as
  // putting `select`'s `document.cookie` write outside the component.
  return (
    <form
      action={setUiModeAction}
      role="group"
      aria-label="Display mode"
      className="inline-flex items-stretch border border-line-strong text-xs font-mono uppercase tracking-[0.14em]"
    >
      <button
        type="submit"
        name="mode"
        value="it"
        aria-pressed={current === "it"}
        onClick={(e) => {
          if (!enhanced) return;
          e.preventDefault();
          select("it");
        }}
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
        type="submit"
        name="mode"
        value="ot"
        aria-pressed={current === "ot"}
        onClick={(e) => {
          if (!enhanced) return;
          e.preventDefault();
          select("ot");
        }}
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
    </form>
  );
}
