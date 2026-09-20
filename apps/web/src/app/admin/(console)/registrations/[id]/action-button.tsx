"use client";

import { useFormStatus } from "react-dom";

// Small client wrapper: renders a submit button inside a form whose
// action is the passed server action (approve, decline, or
// set-status). Adds a per-form pending state so the label flips while
// the round-trip is in flight. Optional `target` gets passed as a
// hidden field for set-status calls.
type Action = (formData: FormData) => Promise<void>;

export function ActionButton({
  action,
  memberId,
  target,
  tone,
  label,
}: {
  action: Action;
  memberId: string;
  target?: string;
  tone: "accent" | "signal";
  label: string;
}) {
  return (
    <form action={action} className="m-0">
      <input type="hidden" name="member_id" value={memberId} />
      {target ? <input type="hidden" name="status" value={target} /> : null}
      <Submit tone={tone} label={label} />
    </form>
  );
}

function Submit({
  tone,
  label,
}: {
  tone: "accent" | "signal";
  label: string;
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
        "inline-flex items-center border px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] transition-colors disabled:cursor-not-allowed disabled:opacity-50 " +
        cls
      }
    >
      {pending ? `${label}…` : label}
    </button>
  );
}
