"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { registerAction, type RegisterState } from "./actions";

const initialState: RegisterState = {};

export function RegisterForm() {
  const [state, formAction] = useActionState(registerAction, initialState);
  const v = state.values ?? {};

  return (
    <form action={formAction} className="space-y-5" noValidate>
      {state.error ? (
        <p
          role="alert"
          className="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-900"
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
          className="mt-1 h-4 w-4 rounded border-line accent-accent"
        />
        <span>
          I understand my activity on this site and my conversations with Ask Roger will be stored
          and visible to Roger. See the{" "}
          <a href="/privacy" className="text-accent underline underline-offset-2">
            privacy policy
          </a>
          .
        </span>
      </label>

      <SubmitButton />

      <p className="text-xs text-ink-3">
        Registration is reviewed by Roger — usually within a day. You&rsquo;ll receive an email when
        he decides.
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
      <label htmlFor={id} className="mb-1 block text-sm font-medium text-ink-2">
        {label}
      </label>
      {children}
      {hint ? <p className="mt-1 text-xs text-ink-3">{hint}</p> : null}
    </div>
  );
}

function SubmitButton() {
  const { pending } = useFormStatus();
  return (
    <button
      type="submit"
      disabled={pending}
      className="inline-flex w-full items-center justify-center rounded-md bg-accent px-5 py-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-60"
    >
      {pending ? "Submitting…" : "Request access"}
    </button>
  );
}

const fieldInputClass =
  "block w-full rounded-md border border-line bg-white px-3 py-2 text-sm text-ink shadow-sm outline-none transition-colors focus:border-accent focus:ring-2 focus:ring-accent/20";
