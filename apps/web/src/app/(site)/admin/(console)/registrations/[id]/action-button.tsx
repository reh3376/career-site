"use client";

import { createContext, useContext, useId, useState } from "react";
import { useFormStatus } from "react-dom";

// One reason box shared by every action button in a section.
//
// Each button is its own <form>, because each posts a different server
// action, so a single input cannot sit in all of them. The value is held
// here instead and mirrored into each form as a hidden field. The
// alternative, a box per button, would put four of them above the
// extend-access row alone.
const ReasonContext = createContext<string>("");

export function ReasonScope({
  children,
  hint,
}: {
  children: React.ReactNode;
  hint: string;
}) {
  const [reason, setReason] = useState("");
  const id = useId();
  return (
    <ReasonContext.Provider value={reason}>
      <div className="mt-4">
        <label
          htmlFor={id}
          className="mb-2 block font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3"
        >
          reason (optional)
        </label>
        <input
          id={id}
          type="text"
          value={reason}
          maxLength={1000}
          onChange={(e) => setReason(e.target.value)}
          placeholder={hint}
          className="block w-full max-w-xl border-0 border-b border-line bg-transparent px-0 py-2 text-sm text-ink outline-none transition-colors placeholder:text-ink-4 focus:border-accent"
        />
        <p className="mt-2 text-xs text-ink-3">
          Recorded with the action in the audit log. Leave it blank if there is
          nothing to add.
        </p>
      </div>
      {children}
    </ReasonContext.Provider>
  );
}

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
  const reason = useContext(ReasonContext);
  return (
    <form action={action} className="m-0">
      <input type="hidden" name="member_id" value={memberId} />
      <input type="hidden" name="reason" value={reason} />
      {extra
        ? Object.entries(extra).map(([k, v]) => (
            <input key={k} type="hidden" name={k} value={v} />
          ))
        : null}
      <Submit tone={tone} label={label} />
    </form>
  );
}

function Submit({ tone, label }: { tone: "accent" | "signal"; label: string }) {
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
