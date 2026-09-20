"use client";

import { useFormStatus } from "react-dom";

import { setContactResolvedAction } from "./actions";

// Client wrapper so we can render a pending-state label on submit.
// Simple form with hidden id + target — server action does the work.
export function ResolveButton({
  id,
  currentlyResolved,
  ticketId,
}: {
  id: string;
  currentlyResolved: boolean;
  ticketId: string;
}) {
  const target = currentlyResolved ? "SUPPORT_STATUS_OPEN" : "SUPPORT_STATUS_RESOLVED";
  return (
    <form action={setContactResolvedAction} className="m-0">
      <input type="hidden" name="id" value={id} />
      <input type="hidden" name="target" value={target} />
      <Submit
        label={currentlyResolved ? `Re-open ${ticketId}` : `Mark ${ticketId} resolved`}
        pendingLabel={currentlyResolved ? "Re-opening…" : "Marking resolved…"}
        tone={currentlyResolved ? "signal" : "accent"}
      />
    </form>
  );
}

function Submit({
  label,
  pendingLabel,
  tone,
}: {
  label: string;
  pendingLabel: string;
  tone: "accent" | "signal";
}) {
  const { pending } = useFormStatus();
  const toneCls =
    tone === "signal"
      ? "border-signal text-signal hover:bg-signal hover:text-white"
      : "border-accent text-accent hover:bg-accent hover:text-white";
  return (
    <button
      type="submit"
      disabled={pending}
      className={
        "inline-flex items-center border px-3 py-1.5 font-mono text-[11px] uppercase tracking-[0.14em] transition-colors disabled:cursor-not-allowed disabled:opacity-50 " +
        toneCls
      }
    >
      {pending ? pendingLabel : label}
    </button>
  );
}
