"use client";

import { useActionState } from "react";
import { PasswordInput } from "@/components/password-input";
import { useFormStatus } from "react-dom";

import { resetPasswordAction, type ResetState } from "./actions";

const fieldInputClass =
  "block w-full border-0 border-b border-line bg-transparent px-0 py-2.5 text-base text-ink outline-none transition-colors placeholder:text-ink-4 focus:border-accent";
const labelClass =
  "mb-2 block font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3";

export function ResetForm({ email }: { email: string }) {
  const initial: ResetState = { values: { email } };
  const [state, formAction] = useActionState(resetPasswordAction, initial);
  const v = state.values ?? {};

  return (
    <form action={formAction} className="space-y-8" noValidate>
      {state.error ? (
        <p
          role="alert"
          className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink"
        >
          {state.error}
        </p>
      ) : null}

      <div>
        <label htmlFor="email" className={labelClass}>
          Email
        </label>
        <input
          id="email"
          name="email"
          type="email"
          required
          autoComplete="email"
          defaultValue={v.email ?? email}
          className={fieldInputClass}
        />
      </div>

      <div>
        <label htmlFor="new_password" className={labelClass}>
          New password
        </label>
        <PasswordInput
          id="new_password"
          name="new_password"
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
        <PasswordInput
          id="confirm"
          name="confirm"
          required
          minLength={12}
          autoComplete="new-password"
          className={fieldInputClass}
        />
      </div>

      <div>
        <label htmlFor="code" className={labelClass}>
          5-digit code from the email
        </label>
        <input
          id="code"
          name="code"
          type="text"
          inputMode="numeric"
          pattern="[0-9]{5}"
          maxLength={5}
          required
          autoComplete="one-time-code"
          placeholder="12345"
          className={fieldInputClass + " font-mono tracking-[0.3em]"}
        />
        <p className="mt-2 text-xs text-ink-3">
          Good for 15 minutes from when it was sent. Five wrong tries
          and you will need a new one.
        </p>
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
