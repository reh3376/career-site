"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { submitContactAction, type ContactState } from "./actions";

const initial: ContactState = {};

const CATEGORIES = [
  { key: "general_question", label: "General question" },
  { key: "bug_report", label: "Bug report" },
  { key: "feature_request", label: "Feature request" },
  { key: "contributor_access", label: "Contributor access on a GitHub repo" },
  { key: "press_inquiry", label: "Press / interview inquiry" },
  { key: "other", label: "Other" },
] as const;

const inputClass =
  "block w-full rounded-md border border-line bg-white px-3 py-2 text-sm text-ink shadow-sm outline-none transition-colors focus:border-accent focus:ring-2 focus:ring-accent/20";

// signedIn is passed from the server component so the anonymous name/email
// fields render only when relevant.
export function ContactForm({ signedIn }: { signedIn: boolean }) {
  const [state, action] = useActionState(submitContactAction, initial);
  const v = state.values ?? {};

  if (state.ok) {
    return (
      <div className="rounded-lg border border-green-200 bg-green-50 px-6 py-8">
        <p className="mb-2 text-xs font-medium uppercase tracking-widest text-green-700">
          Sent
        </p>
        <h2 className="mb-3 text-xl font-semibold text-ink">Thanks — I&rsquo;ll get back to you.</h2>
        <p className="text-sm leading-relaxed text-ink-2">
          Your message reached Roger&rsquo;s inbox. Reference: <span className="font-mono">{state.ticketId}</span>.
          Replies come from <span className="font-mono">rogerhenley345@gmail.com</span>.
        </p>
      </div>
    );
  }

  return (
    <form action={action} className="space-y-5" noValidate>
      {state.error ? (
        <p
          role="alert"
          className="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-900"
        >
          {state.error}
        </p>
      ) : null}

      <div>
        <label htmlFor="category" className="mb-1 block text-sm font-medium text-ink-2">
          What&rsquo;s this about?
        </label>
        <select
          id="category"
          name="category"
          required
          defaultValue={v.category ?? ""}
          className={inputClass}
        >
          <option value="" disabled>
            Pick one
          </option>
          {CATEGORIES.map((c) => (
            <option key={c.key} value={c.key}>
              {c.label}
            </option>
          ))}
        </select>
      </div>

      {!signedIn && (
        <>
          <div>
            <label htmlFor="name" className="mb-1 block text-sm font-medium text-ink-2">
              Your name
            </label>
            <input
              id="name"
              name="name"
              type="text"
              required
              autoComplete="name"
              defaultValue={v.name ?? ""}
              className={inputClass}
            />
          </div>
          <div>
            <label htmlFor="email" className="mb-1 block text-sm font-medium text-ink-2">
              Your email
            </label>
            <input
              id="email"
              name="email"
              type="email"
              required
              autoComplete="email"
              defaultValue={v.email ?? ""}
              className={inputClass}
            />
            <p className="mt-1 text-xs text-ink-3">Only used to reply to you.</p>
          </div>
        </>
      )}

      <div>
        <label htmlFor="subject" className="mb-1 block text-sm font-medium text-ink-2">
          Subject
        </label>
        <input
          id="subject"
          name="subject"
          type="text"
          required
          maxLength={200}
          defaultValue={v.subject ?? ""}
          className={inputClass}
        />
      </div>

      <div>
        <label htmlFor="message" className="mb-1 block text-sm font-medium text-ink-2">
          Message
        </label>
        <textarea
          id="message"
          name="message"
          required
          rows={8}
          maxLength={5000}
          defaultValue={v.message ?? ""}
          className={inputClass + " resize-y"}
        />
        <p className="mt-1 text-xs text-ink-3">Up to 5,000 characters. Plain text is fine.</p>
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
      className="inline-flex w-full items-center justify-center rounded-md bg-accent px-5 py-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-60"
    >
      {pending ? "Sending…" : "Send"}
    </button>
  );
}
