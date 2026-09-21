"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { registerAction, type RegisterState } from "./actions";

const initialState: RegisterState = {};

export function RegisterForm() {
  const [state, formAction] = useActionState(registerAction, initialState);
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

      <Field id="name" label="Full name" defaultValue={v.name}>
        <input
          id="name"
          name="name"
          type="text"
          required
          autoComplete="name"
          defaultValue={v.name ?? ""}
          className={fieldInputClass}
        />
      </Field>

      <Field id="email" label="Email">
        <input
          id="email"
          name="email"
          type="email"
          required
          autoComplete="email"
          defaultValue={v.email ?? ""}
          className={fieldInputClass}
        />
      </Field>

      <Field id="password" label="Password" hint="At least 12 characters.">
        <input
          id="password"
          name="password"
          type="password"
          required
          minLength={12}
          autoComplete="new-password"
          className={fieldInputClass}
        />
      </Field>

      <Field id="organization" label="Organization" hint="Optional.">
        <input
          id="organization"
          name="organization"
          type="text"
          autoComplete="organization"
          defaultValue={v.organization ?? ""}
          className={fieldInputClass}
        />
      </Field>

      <Field id="statedRole" label="Role or title" hint="Optional.">
        <input
          id="statedRole"
          name="statedRole"
          type="text"
          autoComplete="organization-title"
          defaultValue={v.statedRole ?? ""}
          className={fieldInputClass}
        />
      </Field>

      <label className="flex items-start gap-3 text-sm text-ink-2">
        <input
          type="checkbox"
          name="consent"
          required
          className="mt-1 h-4 w-4 border-line accent-accent"
        />
        <span>
          I understand my activity on this site and my conversations with Ask
          Roger will be stored and visible to Roger. See the{" "}
          <a
            href="/privacy"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            privacy policy
          </a>
          .
        </span>
      </label>

      <SubmitButton />

      <p className="text-xs leading-relaxed text-ink-3">
        Registration is reviewed by Roger, usually within a day.
        You&rsquo;ll receive an email when he decides.
      </p>
    </form>
  );
}

function Field({
  id,
  label,
  hint,
  children,
}: {
  id: string;
  label: string;
  hint?: string;
  defaultValue?: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <label
        htmlFor={id}
        className="mb-2 block font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3"
      >
        {label}
      </label>
      {children}
      {hint ? <p className="mt-2 text-xs text-ink-3">{hint}</p> : null}
    </div>
  );
}

function SubmitButton() {
  const { pending } = useFormStatus();
  return (
    <button
      type="submit"
      disabled={pending}
      className="inline-flex items-center justify-center rounded-md bg-accent px-6 py-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-60"
    >
      {pending ? "Submitting…" : "Request access"}
    </button>
  );
}

// Underlined-field inputs matching the contact form: hairline bottom rule
// on paper, accent flip on focus. No shadow, no rounded corners.
const fieldInputClass =
  "block w-full border-0 border-b border-line bg-transparent px-0 py-2.5 text-base text-ink outline-none transition-colors placeholder:text-ink-4 focus:border-accent";
