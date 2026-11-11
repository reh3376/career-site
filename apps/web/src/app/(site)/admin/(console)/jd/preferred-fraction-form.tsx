"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { setPreferredFractionAction, type FractionState } from "./actions";

// One number: the share of its weight a preferred requirement counts
// at in the match record's figures (FSD FR-HF-23). A required item
// counts in full, an ideal one only as bonus credit, and this is the
// setting for the class in between.
export function PreferredFractionForm({ initial }: { initial: number }) {
  const [state, action] = useActionState<FractionState, FormData>(
    setPreferredFractionAction,
    {},
  );
  const current = state.fraction ?? initial;
  return (
    <form action={action} className="grid gap-4 border border-line-strong bg-canvas p-5">
      <label className="block sm:max-w-xs">
        <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          a preferred item counts at
        </span>
        <input
          name="fraction"
          type="number"
          step="0.05"
          min="0.05"
          max="1"
          required
          defaultValue={current.toFixed(2)}
          className="mt-1 block w-full border border-line bg-transparent px-3 py-2 font-mono text-sm text-ink focus:border-accent focus:outline-none"
        />
        <span className="mt-1 block text-xs text-ink-3">
          of its weight. 0.25 is a quarter.
        </span>
      </label>
      <p className="text-xs leading-relaxed text-ink-3">
        Applies to the three figures on the match record, from the next run
        on. A record already written keeps the fraction it was computed with.
        The score that decides whether a résumé is written does not use it yet.
      </p>
      <div className="flex flex-wrap items-center gap-4">
        <Submit />
        {state.error ? <span className="text-sm text-danger">{state.error}</span> : null}
        {state.saved ? <span className="text-sm text-success">Saved; in force for the next run.</span> : null}
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
      {pending ? "Saving..." : "Save"}
    </button>
  );
}
