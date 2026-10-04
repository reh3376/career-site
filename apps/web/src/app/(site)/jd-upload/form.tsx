"use client";

import Link from "next/link";
import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { submitJdAction, type Quota, type SubmitState } from "./actions";
import { JdResult } from "./result";

const initial: SubmitState = {};

const fieldInputClass =
  "block w-full border-0 border-b border-line bg-transparent px-0 py-2.5 text-base text-ink outline-none transition-colors placeholder:text-ink-4 focus:border-accent";
const labelClass =
  "mb-2 block font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3";

export function JdForm({
  threshold,
  quota,
}: {
  threshold: string;
  quota?: Quota;
}) {
  const [state, formAction] = useActionState(submitJdAction, initial);
  const v = state.values ?? {};
  // After a submission the server sends the count back; before one, the
  // page was handed the quota as it stood when it rendered.
  const shown = state.quota ?? quota;

  if (state.ok) {
    return (
      <div className="space-y-5">
        <p className="border-l-2 border-accent bg-accent/10 px-4 py-3 text-sm text-ink">
          {state.message ?? "Submission received."}
        </p>
        {state.submission_id ? (
          <p className="text-sm text-ink-2">
            This review lives at{" "}
            <Link
              href={`/jd-upload/${state.submission_id}`}
              className="font-mono text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
            >
              /jd-upload/{state.submission_id}
            </Link>
            . You can close this tab; it is listed under your submissions and
            you get an email when it finishes.
          </p>
        ) : null}
        {state.submission_id ? (
          <JdResult
            submissionId={state.submission_id}
            resultToken={state.result_token ?? ""}
            threshold={threshold}
            watch
          />
        ) : null}
        {shown ? <QuotaLine quota={shown} /> : null}
        <p className="text-sm leading-relaxed text-ink-2">
          Prefer a quick reply?{" "}
          <a
            href="/contact"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            Use the contact form
          </a>
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

      {shown ? <QuotaLine quota={shown} /> : null}

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
        <label htmlFor="apply_url" className={labelClass}>
          Link to apply (optional)
        </label>
        <input
          id="apply_url"
          name="apply_url"
          type="url"
          inputMode="url"
          defaultValue={v.applyUrl ?? ""}
          placeholder="https://careers.example.com/jobs/12345"
          maxLength={2048}
          className={fieldInputClass}
        />
        <p className="mt-2 text-xs text-ink-3">
          The posting&rsquo;s application page, so Roger can apply directly if the fit is right.
        </p>
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

// How much of the allowance is left, said plainly. A limit someone
// meets without warning reads as a fault, so the number is on the page
// before it is spent as well as after.
function QuotaLine({ quota }: { quota: Quota }) {
  const none = quota.remaining <= 0;
  const when = quota.nextSlotAt
    ? new Date(quota.nextSlotAt).toLocaleString(undefined, {
        weekday: "short",
        hour: "numeric",
        minute: "2-digit",
      })
    : "";
  return (
    <p
      className={
        "border-l-2 px-4 py-3 text-sm " +
        (none ? "border-signal text-ink" : "border-line text-ink-2")
      }
    >
      {none ? (
        <>
          You have used all {quota.limit} reviews for the last{" "}
          {quota.windowHours} hours.
          {when ? ` The next one opens ${when}.` : ""}
        </>
      ) : (
        <>
          {quota.remaining} of {quota.limit} reviews left in the next{" "}
          {quota.windowHours} hours.
        </>
      )}{" "}
      <span className="text-ink-3">
        Each review is close to an hour of work on one machine, so the queue
        has to be finite.
      </span>
    </p>
  );
}
