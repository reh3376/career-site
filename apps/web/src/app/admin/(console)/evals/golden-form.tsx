"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { upsertGoldenAction, type SaveState } from "./actions";

// Adding a posting to the golden set. The only claim being made is
// which side of the gate it belongs on, because that stays true when
// the model changes and a score does not.
export function GoldenForm() {
  const [state, action] = useActionState<SaveState, FormData>(
    upsertGoldenAction,
    {},
  );
  return (
    <details className="border border-line bg-paper-2 p-5">
      <summary className="cursor-pointer font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        add or replace a posting
      </summary>
      <form action={action} className="mt-4 space-y-3">
        <div className="grid gap-3 sm:grid-cols-2">
          <Field
            label="name (unique)"
            name="name"
            placeholder="bosch-plant-manager"
            required
          />
          <label className="block">
            <span className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-4">
              belongs
            </span>
            <select
              name="expected_gate"
              defaultValue="above"
              className="mt-1 block w-full border border-line bg-canvas px-3 py-2 text-sm text-ink"
            >
              <option value="above">Above the gate (a real fit)</option>
              <option value="below">Below the gate (not a fit)</option>
            </select>
          </label>
          <Field
            label="role hint"
            name="role_hint"
            placeholder="Director of Manufacturing Technology"
          />
          <Field
            label="employer hint"
            name="employer_hint"
            placeholder="Bosch"
          />
        </div>
        <label className="block">
          <span className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-4">
            posting text
          </span>
          <textarea
            name="jd_text"
            rows={8}
            required
            className="mt-1 block w-full border border-line bg-canvas px-3 py-2 font-mono text-[12px] leading-snug text-ink"
          />
        </label>
        <Field
          label="why it is in the set"
          name="note"
          placeholder="the leadership requirement it used to miss"
        />
        <Submit />
        {state.error ? (
          <p className="text-sm text-danger">{state.error}</p>
        ) : null}
        {state.saved ? <p className="text-sm text-signal">Saved.</p> : null}
      </form>
    </details>
  );
}

function Field({
  label,
  name,
  placeholder,
  required,
}: {
  label: string;
  name: string;
  placeholder?: string;
  required?: boolean;
}) {
  return (
    <label className="block">
      <span className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-4">
        {label}
      </span>
      <input
        type="text"
        name={name}
        placeholder={placeholder}
        required={required}
        className="mt-1 block w-full border border-line bg-canvas px-3 py-2 text-sm text-ink"
      />
    </label>
  );
}

function Submit() {
  const { pending } = useFormStatus();
  return (
    <button
      type="submit"
      disabled={pending}
      className="inline-flex items-center border border-accent px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
    >
      {pending ? "Saving..." : "Save posting"}
    </button>
  );
}
