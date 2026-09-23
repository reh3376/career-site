"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { reviewDecisionAction, type ReviewState } from "./actions";

type Props = {
  decisionId: string;
  options: string[];
  current?: string;
  note?: string;
};

// One review form per decision: the owner's verdict in the vocabulary
// that applies to this decision kind, plus a note. Saving again
// overwrites.
//
// The result of the save is shown. Before, a rejected review was
// indistinguishable from a saved one, so a row that could not be graded
// simply stayed in the queue with no explanation.
export function ReviewForm({ decisionId, options, current, note }: Props) {
  const [state, action] = useActionState<ReviewState, FormData>(
    reviewDecisionAction,
    {},
  );

  // No vocabulary means the server will refuse this kind, so say that
  // here rather than offering buttons that cannot work.
  if (options.length === 0) {
    return (
      <p className="mt-4 border-t border-line pt-4 text-sm text-ink-2">
        This kind of decision has no review vocabulary yet, so it cannot be
        graded. That is a gap in the tool, not in the decision.
      </p>
    );
  }

  return (
    <form action={action} className="mt-4 border-t border-line pt-4">
      <input type="hidden" name="decision_id" value={decisionId} />
      <fieldset className="flex flex-wrap items-center gap-x-5 gap-y-2">
        <legend className="mb-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          your verdict
        </legend>
        {options.map((o) => (
          <label
            key={o}
            className="inline-flex cursor-pointer items-center gap-2 text-sm text-ink"
          >
            <input
              type="radio"
              name="human_verdict"
              value={o}
              defaultChecked={current === o}
              required
              className="accent-accent"
            />
            {o.replace(/_/g, " ")}
          </label>
        ))}
      </fieldset>
      <label className="mt-3 block">
        <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          note
        </span>
        <textarea
          name="human_note"
          defaultValue={note ?? ""}
          rows={2}
          maxLength={4000}
          placeholder="Why, in a sentence. What the evidence really shows, or what is missing from the corpus."
          className="mt-1 w-full border border-line bg-transparent px-3 py-2 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
      </label>
      <div className="mt-3 flex flex-wrap items-center gap-4">
        <Submit hasLabel={Boolean(current)} />
        {state.error ? (
          <p className="text-sm text-danger">{state.error}</p>
        ) : null}
        {state.saved ? <p className="text-sm text-signal">Saved.</p> : null}
      </div>
    </form>
  );
}

function Submit({ hasLabel }: { hasLabel: boolean }) {
  const { pending } = useFormStatus();
  return (
    <button
      type="submit"
      disabled={pending}
      className="inline-flex items-center border border-accent px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
    >
      {pending ? "Saving..." : hasLabel ? "Update review" : "Save review"}
    </button>
  );
}
