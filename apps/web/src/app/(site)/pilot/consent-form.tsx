"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { startPilotAction, type ConsentState } from "./actions";

const initial: ConsentState = {};

// The consent screen's controls (FSD FR-PLT-03). The text around them
// is in page.tsx; this records exactly two things, the consent and the
// recording preference, and nothing else.
export function ConsentForm() {
  const [state, formAction] = useActionState(startPilotAction, initial);
  return (
    <form action={formAction} className="space-y-6" noValidate>
      {state.error ? (
        <p role="alert" className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          {state.error}
        </p>
      ) : null}
      <label className="flex items-start gap-3 text-sm text-ink">
        <input type="checkbox" name="consent" required className="mt-1 h-4 w-4" />
        <span>I understand what is being asked and what is kept, and I want to take part.</span>
      </label>
      <label className="flex items-start gap-3 text-sm text-ink-2">
        <input type="checkbox" name="recording_ok" className="mt-1 h-4 w-4" />
        <span>You may record the call. (Optional.)</span>
      </label>
      <Submit />
    </form>
  );
}

function Submit() {
  const { pending } = useFormStatus();
  return (
    <button
      type="submit"
      disabled={pending}
      className="inline-flex items-center justify-center rounded-md bg-accent px-6 py-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-60"
    >
      {pending ? "Starting…" : "Start"}
    </button>
  );
}
