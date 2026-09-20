"use client";

import { useFormStatus } from "react-dom";

import {
  approveRegistrationAction,
  declineRegistrationAction,
} from "./actions";

// Approve / Decline pair, rendered only for rows in PENDING_APPROVAL.
// Each button is its own form so pending state is per-button.
export function DecisionButtons({ memberId }: { memberId: string }) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <form action={approveRegistrationAction} className="m-0">
        <input type="hidden" name="member_id" value={memberId} />
        <Btn tone="accent" label="Approve" pendingLabel="Approving…" />
      </form>
      <form action={declineRegistrationAction} className="m-0">
        <input type="hidden" name="member_id" value={memberId} />
        <Btn tone="signal" label="Decline" pendingLabel="Declining…" />
      </form>
    </div>
  );
}

function Btn({
  tone,
  label,
  pendingLabel,
}: {
  tone: "accent" | "signal";
  label: string;
  pendingLabel: string;
}) {
  const { pending } = useFormStatus();
  const cls =
    tone === "signal"
      ? "border-signal text-signal hover:bg-signal hover:text-white"
      : "border-accent text-accent hover:bg-accent hover:text-white";
  return (
    <button
      type="submit"
      disabled={pending}
      className={
        "inline-flex items-center border px-3 py-1.5 font-mono text-[11px] uppercase tracking-[0.14em] transition-colors disabled:cursor-not-allowed disabled:opacity-50 " +
        cls
      }
    >
      {pending ? pendingLabel : label}
    </button>
  );
}
