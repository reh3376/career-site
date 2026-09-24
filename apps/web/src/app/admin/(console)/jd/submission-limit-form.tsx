"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { setJdLimitAction, type LimitState } from "./actions";

// How many postings one member may submit per rolling window.
//
// This is a capacity number, not a courtesy one. A review is close to
// an hour of inference and the pipeline runs one posting at a time, so
// the cap is what keeps one member's queue from being everyone else's
// wait. It lives here rather than in an environment variable because
// what it should be depends on how long a review currently takes, and
// that changes with the model, the corpus and the box.
export function SubmissionLimitForm({
  initial,
  windowHours,
}: {
  initial: number;
  windowHours: number;
}) {
  const [state, action] = useActionState<LimitState, FormData>(
    setJdLimitAction,
    {},
  );
  const current = state.limit ?? initial;
  return (
    <form action={action} className="grid gap-4 border border-line-strong bg-canvas p-5">
      <label className="block max-w-xs">
        <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          Reviews per member per {windowHours} hours
        </span>
        <input
          name="limit"
          type="number"
          step="1"
          min="0"
          max="100"
          required
          defaultValue={current}
          className="mt-1 block w-full border border-line bg-transparent px-3 py-2 font-mono text-sm text-ink focus:border-accent focus:outline-none"
        />
        <span className="mt-1 block text-xs text-ink-3">
          Counted from the submissions table over a rolling window, so it
          survives a restart. You are exempt, and evaluation runs are not
          counted.
        </span>
      </label>
      <p className="text-xs leading-relaxed text-ink-3">
        A review is close to an hour of work on one machine and they run one
        at a time, so this number times the number of active members is the
        longest queue anyone can be put behind. Set it to 0 to remove the cap
        entirely, which leaves nothing between one member with a script and
        the whole box. Lowering it never penalises a member already over the
        new number: they wait for their oldest submission to age out.
      </p>
      <div className="flex flex-wrap items-center gap-4">
        <Submit />
        {state.error ? <span className="text-sm text-danger">{state.error}</span> : null}
        {state.saved ? <span className="text-sm text-success">Saved; in force now.</span> : null}
      </div>
    </form>
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
      {pending ? "Saving..." : "Save limit"}
    </button>
  );
}
