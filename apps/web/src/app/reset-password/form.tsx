"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { resetPasswordAction, type ResetState } from "./actions";

const initial: ResetState = {};

const fieldInputClass =
  "block w-full border-0 border-b border-line bg-transparent px-0 py-2.5 text-base text-ink outline-none transition-colors placeholder:text-ink-4 focus:border-accent";
const labelClass =
  "mb-2 block font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3";

export function ResetForm({ token }: { token: string }) {
  const [state, formAction] = useActionState(resetPasswordAction, initial);

  return (
    <form action={formAction} className="space-y-8" noValidate>
      <input type="hidden" name="token" value={token} />

      {state.error ? (
        <p
          role="alert"
          className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink"
        >
          {state.error}
        </p>
      ) : null}

      <div>
        <label htmlFor="new_password" className={labelClass}>
          New password
        </label>
        <input
          id="new_password"
          name="new_password"
          type="password"
          required
          minLength={12}
          autoComplete="new-password"
          className={fieldInputClass}
        />
        <p className="mt-2 text-xs text-ink-3">
          At least 12 characters. Anything that appears in a known
          breach is rejected.
        </p>
      </div>

      <div>
        <label htmlFor="confirm" className={labelClass}>
          Confirm new password
        </label>
        <input
          id="confirm"
          name="confirm"
          type="password"
          required
          minLength={12}
          autoComplete="new-password"
          className={fieldInputClass}
        />
      </div>

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
      {pending ? "Updating…" : "Set new password"}
    </button>
  );
}
