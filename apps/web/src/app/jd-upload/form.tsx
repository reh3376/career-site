"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { submitJdAction, type SubmitState } from "./actions";

const initial: SubmitState = {};

const fieldInputClass =
  "block w-full border-0 border-b border-line bg-transparent px-0 py-2.5 text-base text-ink outline-none transition-colors placeholder:text-ink-4 focus:border-accent";
const labelClass =
  "mb-2 block font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3";

export function JdForm() {
  const [state, formAction] = useActionState(submitJdAction, initial);
  const v = state.values ?? {};

  if (state.ok) {
    return (
      <div className="space-y-5">
        <p className="border-l-2 border-accent bg-accent/10 px-4 py-3 text-sm text-ink">
          {state.message ?? "Submission received."}
        </p>
        {state.submission_id ? (
          <p className="font-mono text-xs text-ink-3">
            ref{" "}
            <span className="text-ink">#{state.submission_id}</span>,
            keep this if you want to check back later.
          </p>
        ) : null}
        <p className="text-sm leading-relaxed text-ink-2">
          Roger will follow up personally if the fit looks right.
          Prefer a quick reply?{" "}
          <a
            href="/contact"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            Use the contact form
          </a>{" "}
         , same inbox.
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
        <label htmlFor="jd_text" className={labelClass}>
          Job description
        </label>
        <textarea
          id="jd_text"
          name="jd_text"
          required
          minLength={100}
          maxLength={50000}
          rows={16}
          defaultValue={v.jdText ?? ""}
          placeholder="Paste the full posting here, title, responsibilities, requirements, comp band if you have one. The more you give, the better the match."
          className="block w-full border border-line-strong bg-canvas px-3 py-2 font-mono text-[13px] leading-snug text-ink outline-none focus:border-accent"
        />
        <p className="mt-2 text-xs text-ink-3">
          Minimum 100 characters. PDF upload lands next, for now,
          paste the text.
        </p>
      </div>

      <div className="grid gap-6 sm:grid-cols-2">
        <div>
          <label htmlFor="role_hint" className={labelClass}>
            Role (optional)
          </label>
          <input
            id="role_hint"
            name="role_hint"
            type="text"
            defaultValue={v.roleHint ?? ""}
            placeholder="e.g. Sr. Manager, Manufacturing AI"
            maxLength={200}
            className={fieldInputClass}
          />
        </div>
        <div>
          <label htmlFor="employer_hint" className={labelClass}>
            Employer (optional)
          </label>
          <input
            id="employer_hint"
            name="employer_hint"
            type="text"
            defaultValue={v.employerHint ?? ""}
            placeholder="e.g. Anthropic"
            maxLength={200}
            className={fieldInputClass}
          />
        </div>
      </div>

      <div>
        <label htmlFor="contact_email" className={labelClass}>
          Your email (optional)
        </label>
        <input
          id="contact_email"
          name="contact_email"
          type="email"
          defaultValue={v.contactEmail ?? ""}
          placeholder="so Roger can follow up without you keeping this tab open"
          autoComplete="email"
          maxLength={254}
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
      {pending ? "Submitting…" : "Submit for review"}
    </button>
  );
}
