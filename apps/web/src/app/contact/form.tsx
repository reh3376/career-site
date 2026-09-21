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

// Inputs sit on the paper background with a single hairline bottom rule,
// no full-box border, no shadow. Closer to a form filled in on a
// clipboard than one clicked in a SaaS admin. The border-color flip on
// focus does the accent work without ring bloom.
const inputClass =
  "block w-full border-0 border-b border-line bg-transparent px-0 py-2.5 text-base text-ink outline-none transition-colors placeholder:text-ink-4 focus:border-accent";
const labelClass =
  "mb-2 block font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3";
const helpClass = "mt-2 text-xs text-ink-3";

// signedIn is passed from the server component so the anonymous name/email
// fields render only when relevant. initialCategory pre-selects the
// <select> (typically from a ?category=... URL param on the /contact
// route, e.g. the GitHub-cards CTA points at
// /contact?category=contributor_access).
export function ContactForm({
  signedIn,
  initialCategory,
}: {
  signedIn: boolean;
  initialCategory?: string;
}) {
  const [state, action] = useActionState(submitContactAction, initial);
  const v = state.values ?? {};

  if (state.ok) {
    return (
      <div className="border-l-2 border-accent bg-accent-soft/40 py-8 pl-6 pr-4">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
          sent
        </p>
        <h2
          className="font-display mt-3 text-2xl leading-snug text-ink"
          style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
        >
          Thanks, I&rsquo;ll get back to you.
        </h2>
        <p className="mt-4 text-sm leading-relaxed text-ink-2">
          Your message reached my inbox. Reference{" "}
          <span className="font-mono text-ink">{state.ticketId}</span>. Replies
          come from <span className="font-mono text-ink">rogerhenley345@gmail.com</span>.
        </p>
      </div>
    );
  }

  return (
    <form action={action} className="space-y-8" noValidate>
      {state.error ? (
        <p
          role="alert"
          className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink"
        >
          {state.error}
        </p>
      ) : null}

      <div>
        <label htmlFor="category" className={labelClass}>
          What&rsquo;s this about?
        </label>
        <select
          id="category"
          name="category"
          required
          defaultValue={v.category ?? initialCategory ?? ""}
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
        <div className="grid gap-8 sm:grid-cols-2">
          <div>
            <label htmlFor="name" className={labelClass}>
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
            <label htmlFor="email" className={labelClass}>
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
            <p className={helpClass}>Only used to reply to you.</p>
          </div>
        </div>
      )}

      <div>
        <label htmlFor="subject" className={labelClass}>
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
        <label htmlFor="message" className={labelClass}>
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
        <p className={helpClass}>Up to 5,000 characters. Plain text is fine.</p>
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
      {pending ? "Sending…" : "Send message"}
    </button>
  );
}
