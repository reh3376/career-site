"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { loginAction, type LoginState } from "./actions";

const initial: LoginState = {};

const fieldInputClass =
  "block w-full border-0 border-b border-line bg-transparent px-0 py-2.5 text-base text-ink outline-none transition-colors placeholder:text-ink-4 focus:border-accent";
const labelClass =
  "mb-2 block font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3";

export function LoginForm({ next }: { next?: string } = {}) {
  const [state, formAction] = useActionState(loginAction, initial);
  const v = state.values ?? {};

  return (
    <form action={formAction} className="space-y-8" noValidate>
      {/* Hidden pass-through of the intended post-login destination.
          The action validates the value before redirecting so this is
          not an open-redirect vector. */}
      {next ? <input type="hidden" name="next" value={next} /> : null}
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

      <div>
        <label htmlFor="password" className={labelClass}>
          Password
        </label>
        <input
          id="password"
          name="password"
          type="password"
          required
          autoComplete="current-password"
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
      {pending ? "Signing in…" : "Sign in"}
    </button>
  );
}
