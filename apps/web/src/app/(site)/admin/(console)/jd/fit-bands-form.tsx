"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { setFitBandsAction, type FitBandsState } from "./actions";

type Bands = { veryStrong: number; strong: number; possible: number; weak: number };

const FIELDS: { key: keyof Bands; label: string; note: string }[] = [
  { key: "veryStrong", label: "Very strong at or above", note: "résumé attached to the email" },
  { key: "strong", label: "Strong at or above", note: "also the résumé gate" },
  { key: "possible", label: "Possible at or above", note: "you review and reply" },
  { key: "weak", label: "Weak at or above", note: "you review and reply; below this is very weak" },
];

// The numbers that classify a review into five bands: very strong,
// strong, possible, weak and very weak. Four numbers describe five
// bands, because very weak is whatever falls below the weak line and
// needs no threshold of its own.
//
// Saved to app_settings; the pipeline, the result pages and the emails
// pick them up within seconds. Existing scores are re-classified on
// read, so changing a band never rewrites a stored score.
export function FitBandsForm({ initial }: { initial: Bands }) {
  const [state, action] = useActionState<FitBandsState, FormData>(setFitBandsAction, {});
  const current = state.bands ?? initial;
  return (
    <form action={action} className="grid gap-4 border border-line-strong bg-canvas p-5">
      <div className="grid gap-4 sm:grid-cols-2">
        {FIELDS.map((f) => (
          <label key={f.key} className="block">
            <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">{f.label}</span>
            <input
              name={f.key}
              type="number"
              step="0.01"
              min="0.01"
              max="1"
              required
              defaultValue={current[f.key].toFixed(2)}
              className="mt-1 block w-full border border-line bg-transparent px-3 py-2 font-mono text-sm text-ink focus:border-accent focus:outline-none"
            />
            <span className="mt-1 block text-xs text-ink-3">{f.note}</span>
          </label>
        ))}
      </div>
      <p className="text-xs leading-relaxed text-ink-3">
        Five bands, four numbers: anything below the weak line is very weak.
        Order must hold: weak &lt; possible &lt; strong &lt; very strong, all between 0 and 1.
        The calibration behind the defaults is in docs/llm-tuning-log.md; the decision
        log shows how each verdict was reached if a band feels wrong.
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
      {pending ? "Saving..." : "Save bands"}
    </button>
  );
}
