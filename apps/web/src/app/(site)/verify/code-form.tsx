"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { verifyByCodeAction, type CodeState } from "./actions";
import { Verified } from "./verified";

// Email plus the six digits from the email. The address is prefilled
// when the email's link brought the person here; it is editable because
// the person may have registered with a different one than they think.
export function CodeForm({ initialEmail }: { initialEmail: string }) {
  const [state, formAction] = useActionState(verifyByCodeAction, { values: { email: initialEmail } } as CodeState);
  if (state.ok) {
    return <Verified name={state.name ?? ""} status={state.status ?? ""} />;
  }
  return (
    <form action={formAction} className="mt-8 max-w-md space-y-5">
      {state.error ? (
        <p role="alert" className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          {state.error}
        </p>
      ) : null}
      <label className="block">
        <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">email</span>
        <input
          type="email"
          name="email"
          required
          autoComplete="email"
          defaultValue={state.values?.email ?? initialEmail}
          className="mt-1 block w-full border border-line-strong bg-canvas px-3 py-2 text-sm text-ink outline-none focus:border-accent"
        />
      </label>
      <label className="block">
        <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">code from the email</span>
        <input
          type="text"
          name="code"
          required
          inputMode="numeric"
          autoComplete="one-time-code"
          pattern="[0-9]{6}"
          maxLength={6}
          placeholder="six digits"
          className="mt-1 block w-full border border-line-strong bg-canvas px-3 py-2 font-mono text-lg tracking-[0.3em] text-ink outline-none focus:border-accent"
        />
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
      {pending ? "Checking…" : "Verify"}
    </button>
  );
}
