"use client";

import { useActionState, useState } from "react";
import { useFormStatus } from "react-dom";

import { reviewDecisionAction, type ReviewState } from "./actions";

type Props = {
  decisionId: string;
  options: string[];
  current?: string;
  note?: string;
  /**
   * Rubric dimensions for this decision kind, when it has one. Empty
   * for the classification kinds, where the verdict is the whole grade.
   */
  dimensions?: string[];
  currentDimensions?: Record<string, string>;
  /**
   * The model's own answer, for a kind whose output is prose. Its
   * presence is what turns this form from a label into a training
   * example: the box below starts as the model's wording and the owner
   * edits it, so a correction costs an edit rather than a retype.
   */
  modelAnswer?: string;
  currentAnswer?: string;
};

const DIMENSION_VALUES = ["yes", "partial", "no", "n/a"];

// What each dimension is actually asking, in a few words. Without
// these, "scope" and "length" are guesses, and a rubric answered from a
// guess is worse than no rubric: the marks still count toward the rates
// the acceptance criteria are stated in.
const DIMENSION_HELP: Record<string, string> = {
  grounded: "everything it said is in the passages it was shown",
  voice: "sounds like you wrote it",
};

// One review form per decision: the owner's verdict in the vocabulary
// that applies to this decision kind, plus a note. Saving again
// overwrites.
//
// The result of the save is shown. Before, a rejected review was
// indistinguishable from a saved one, so a row that could not be graded
// simply stayed in the queue with no explanation.
export function ReviewForm({
  decisionId,
  options,
  current,
  note,
  dimensions = [],
  currentDimensions = {},
  modelAnswer,
  currentAnswer,
}: Props) {
  const [state, action] = useActionState<ReviewState, FormData>(
    reviewDecisionAction,
    {},
  );
  // Seeded with the model's wording so the common correction is an
  // edit. The original travels in a hidden field and the server drops
  // an unchanged answer, so leaving it alone records no correction
  // rather than a preference pair of a thing against itself.
  const [answer, setAnswer] = useState(currentAnswer || modelAnswer || "");
  const edited = Boolean(modelAnswer) && answer.trim() !== modelAnswer?.trim();

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
      {modelAnswer === undefined ? null : (
        <input type="hidden" name="model_answer" value={modelAnswer} />
      )}
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

      {dimensions.length === 0 ? null : (
        <fieldset className="mt-4">
          <legend className="mb-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            how it did, per thing
          </legend>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[30rem] border-collapse text-sm">
              <thead>
                <tr>
                  <th className="w-1/2 py-1 text-left font-normal text-ink-3" />
                  {DIMENSION_VALUES.map((v) => (
                    <th
                      key={v}
                      className="px-2 py-1 text-center font-mono text-[11px] font-normal tracking-[0.1em] text-ink-3"
                    >
                      {v}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {dimensions.map((d) => (
                  <tr key={d} className="border-t border-line">
                    <td className="py-2 pr-3 text-ink">
                      {d}
                      <span className="ml-2 text-ink-3">
                        {DIMENSION_HELP[d] ?? ""}
                      </span>
                    </td>
                    {DIMENSION_VALUES.map((v) => (
                      <td key={v} className="px-2 py-2 text-center">
                        <input
                          type="radio"
                          name={`dim_${d}`}
                          value={v}
                          defaultChecked={currentDimensions[d] === v}
                          aria-label={`${d}: ${v}`}
                          className="accent-accent"
                        />
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </fieldset>
      )}

      {modelAnswer === undefined ? null : (
        <label className="mt-4 block">
          <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            how you would have answered
          </span>
          <span className="mt-1 block text-sm text-ink-3">
            Edit this into your own words. This is what a future model is
            trained to say, so it matters more than the verdict does. Leave it
            alone if the answer was right.
          </span>
          <textarea
            name="human_answer"
            value={answer}
            onChange={(e) => setAnswer(e.target.value)}
            rows={5}
            maxLength={8000}
            className="mt-2 w-full border border-line bg-transparent px-3 py-2 text-sm leading-relaxed text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
          />
          <span className="mt-1 block font-mono text-[11px] text-ink-3">
            {edited
              ? "edited: this will be saved as a training target"
              : "unchanged: nothing will be saved as a correction"}
          </span>
        </label>
      )}

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
