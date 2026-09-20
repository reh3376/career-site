"use client";

import { useFormStatus } from "react-dom";

// Small client wrapper: renders a submit button inside a form whose
// action is the passed server action (approve, decline, set-status,
// extend-access). Adds a per-form pending state so the label flips
// while the round-trip is in flight. `extra` is a bag of arbitrary
// hidden fields the specific action needs (e.g. { status: "..." },
// { mode: "days", days: "30" }).
type Action = (formData: FormData) => Promise<void>;

export function ActionButton({
  action,
  memberId,
  extra,
  tone,
  label,
}: {
  action: Action;
  memberId: string;
  extra?: Record<string, string>;
  tone: "accent" | "signal";
  label: string;
}) {
  return (
    <form action={action} className="m-0">
      <input type="hidden" name="member_id" value={memberId} />
      {extra
        ? Object.entries(extra).map(([k, v]) => (
            <input key={k} type="hidden" name={k} value={v} />
          ))
        : null}
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
