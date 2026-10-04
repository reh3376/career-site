"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { forgotPasswordAction, type ForgotState } from "./actions";

const initial: ForgotState = {};

const fieldInputClass =
  "block w-full border-0 border-b border-line bg-transparent px-0 py-2.5 text-base text-ink outline-none transition-colors placeholder:text-ink-4 focus:border-accent";
const labelClass =
  "mb-2 block font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3";

export function ForgotForm() {
  const [state, formAction] = useActionState(forgotPasswordAction, initial);
  const v = state.values ?? {};

  if (state.submitted) {
    return (
      <div className="space-y-4">
        <p className="border-l-2 border-accent bg-accent/10 px-4 py-3 text-sm text-ink">
          {state.message}
        </p>
        <p className="text-sm text-ink-3">
          Nothing arrived after a couple of minutes? Check your spam
          folder, or try again with the exact address on your account.
        </p>
      </div>
    );
  }

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
          defaultValue={v.email ?? ""}
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
      {pending ? "Sending…" : "Send reset link"}
    </button>
  );
}
