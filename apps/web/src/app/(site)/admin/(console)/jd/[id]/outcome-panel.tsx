"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import {
  recordJdFeedbackAction,
  setJdOutcomeAction,
  type FeedbackState,
  type OutcomeState,
} from "../actions";

// Ground truth and judgment (data layer D3).
//
// Everything else on this page is the system's account of itself. This
// is the only place a human says whether the answer was right, and
// whether the job went anywhere. Both are on the submission page
// rather than in a report, because the moment you know is the moment
// you are looking at the posting.

export type Outcome = {
  status?: string;
  decided_on?: string;
  decidedOn?: string;
  note?: string;
};

export type Feedback = {
  target?: string;
  rating?: string;
  note?: string;
  source?: string;
  run_id?: string;
  runId?: string;
};

// Ordered as a posting usually moves, so the control reads like a
// timeline rather than an alphabetical list.
const OUTCOMES: { value: string; label: string }[] = [
  { value: "not_pursued", label: "Did not pursue" },
  { value: "applied", label: "Applied" },
  { value: "screening", label: "Screening" },
  { value: "interview", label: "Interview" },
  { value: "offer", label: "Offer" },
  { value: "rejected", label: "Rejected" },
  { value: "no_response", label: "No response" },
  { value: "withdrew", label: "Withdrew" },
];

// Not thumbs. "Too generous" and "too harsh" point at opposite fixes,
// and one bit of signal cannot tell them apart.
const SCORE_RATINGS: { value: string; label: string }[] = [
  { value: "accurate", label: "About right" },
  { value: "too_generous", label: "Too generous" },
  { value: "too_harsh", label: "Too harsh" },
  { value: "unusable", label: "Unusable" },
];

const RESUME_RATINGS: { value: string; label: string }[] = [
  { value: "would_send", label: "Would send" },
  { value: "needs_edits", label: "Needs edits" },
  { value: "wrong", label: "Wrong" },
];

export function OutcomePanel({
  submissionId,
  runId,
  outcome,
  feedback,
  hasResume,
}: {
  submissionId: string;
  runId: string;
  outcome?: Outcome;
  feedback: Feedback[];
  hasResume: boolean;
}) {
  const current = outcome?.status ?? "";
  const decided = outcome?.decided_on ?? outcome?.decidedOn ?? "";
  const scoreRating = feedback.find((f) => f.target === "score")?.rating ?? "";
  const resumeRating =
    feedback.find((f) => f.target === "resume")?.rating ?? "";

  return (
    <section className="mt-10 border border-line-strong bg-paper-2 p-5">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        outcome and judgment
      </p>
      <p className="mt-2 max-w-2xl text-sm leading-relaxed text-ink-2">
        The rest of this page is the system&rsquo;s account of itself. This is
        where you say whether it was right. Without it there is no way to know
        whether a high score predicts anything.
      </p>

      <div className="mt-6 grid gap-8 lg:grid-cols-2">
        <OutcomeForm
          submissionId={submissionId}
          current={current}
          decided={decided}
          note={outcome?.note ?? ""}
        />
        <div className="space-y-6">
          <RatingForm
            submissionId={submissionId}
            runId={runId}
            target="score"
            legend="Was the score right?"
            options={SCORE_RATINGS}
            current={scoreRating}
          />
          {hasResume ? (
            <RatingForm
              submissionId={submissionId}
              runId={runId}
              target="resume"
              legend="Is the résumé sendable?"
              options={RESUME_RATINGS}
              current={resumeRating}
            />
          ) : null}
        </div>
      </div>
    </section>
  );
}

function OutcomeForm({
  submissionId,
  current,
  decided,
  note,
}: {
  submissionId: string;
  current: string;
  decided: string;
  note: string;
}) {
  const [state, action] = useActionState<OutcomeState, FormData>(
    setJdOutcomeAction,
    {},
  );
  return (
    <form action={action} className="space-y-3">
      <input type="hidden" name="submission_id" value={submissionId} />
      <label className="block">
        <span className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-4">
          what happened
        </span>
        <select
          name="status"
          defaultValue={current}
          className="mt-1 block w-full border border-line bg-canvas px-3 py-2 text-sm text-ink"
        >
          <option value="">Not recorded</option>
          {OUTCOMES.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </label>
      <label className="block">
        <span className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-4">
          when (optional)
        </span>
        <input
          type="date"
          name="decided_on"
          defaultValue={decided}
          className="mt-1 block w-full border border-line bg-canvas px-3 py-2 text-sm text-ink"
        />
      </label>
      <label className="block">
        <span className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-4">
          note (optional)
        </span>
        <textarea
          name="note"
          defaultValue={note}
          rows={2}
          className="mt-1 block w-full border border-line bg-canvas px-3 py-2 text-sm text-ink"
        />
      </label>
      <Submit label="Save outcome" />
      <Status
        saved={state.saved}
        error={state.error}
        savedText="Outcome saved."
      />
    </form>
  );
}

function RatingForm({
  submissionId,
  runId,
  target,
  legend,
  options,
  current,
}: {
  submissionId: string;
  runId: string;
  target: string;
  legend: string;
  options: { value: string; label: string }[];
  current: string;
}) {
  const [state, action] = useActionState<FeedbackState, FormData>(
    recordJdFeedbackAction,
    {},
  );
  return (
    <form action={action}>
      <input type="hidden" name="submission_id" value={submissionId} />
      <input type="hidden" name="run_id" value={runId} />
      <input type="hidden" name="target" value={target} />
      <fieldset className="m-0 border-0 p-0">
        <legend className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-4">
          {legend}
        </legend>
        <div className="mt-2 flex flex-wrap gap-2">
          {options.map((o) => (
            <button
              key={o.value}
              type="submit"
              name="rating"
              value={o.value}
              className={
                "border px-3 py-1.5 font-mono text-[11px] uppercase tracking-[0.14em] transition-colors " +
                (current === o.value
                  ? "border-accent bg-accent text-white"
                  : "border-line text-ink-2 hover:border-accent hover:text-accent")
              }
            >
              {o.label}
            </button>
          ))}
        </div>
      </fieldset>
      <Status saved={state.saved} error={state.error} savedText="Recorded." />
    </form>
  );
}

function Submit({ label }: { label: string }) {
  const { pending } = useFormStatus();
  return (
    <button
      type="submit"
      disabled={pending}
      className="inline-flex items-center border border-accent px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
    >
      {pending ? "Saving..." : label}
    </button>
  );
}

function Status({
  saved,
  error,
  savedText,
}: {
  saved?: boolean;
  error?: string;
  savedText: string;
}) {
  if (error) return <p className="mt-2 text-sm text-danger">{error}</p>;
  if (saved) return <p className="mt-2 text-sm text-signal">{savedText}</p>;
  return null;
}
