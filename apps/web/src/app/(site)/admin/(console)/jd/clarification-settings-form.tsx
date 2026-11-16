"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { setClarificationSettingsAction, type ClarificationSettingsState } from "./actions";

// The clarification loop's two settings (FSD FR-HF-22, FR-HF-24):
// whether a review asks the candidate by itself, which withholds the
// fit until the questions are closed, and how long it waits. It ships
// switched off, so the first review that waits is one the owner chose
// to start.
export function ClarificationSettingsForm({ initialAuto, initialDays }: { initialAuto: boolean; initialDays: number }) {
  const [state, action] = useActionState<ClarificationSettingsState, FormData>(setClarificationSettingsAction, {});
  const auto = state.auto ?? initialAuto;
  const days = state.days ?? initialDays;
  return (
    <form
      key={`${auto}:${days}:${state.saved ? "saved" : ""}`}
      action={action}
      className="grid gap-4 border border-line-strong bg-canvas p-5"
    >
      <label className="flex items-start gap-3 text-sm leading-relaxed text-ink">
        <input type="checkbox" name="auto" defaultChecked={auto} className="mt-1 accent-accent" />
        <span>
          Ask me by itself, and give no fit until the questions are closed.
          <span className="mt-1 block text-xs text-ink-3">
            Applies to a difference of wording, a different tool, a neighbouring setting and an unproven relationship.
            The recruiter is told the review is waiting on you and by when it will finish regardless.
          </span>
        </span>
      </label>
      <label className="block sm:max-w-xs">
        <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">a withheld review waits at most</span>
        <input
          name="days"
          type="number"
          step="1"
          min="1"
          max="14"
          required
          defaultValue={days}
          className="mt-1 block w-full border border-line bg-transparent px-3 py-2 font-mono text-sm text-ink focus:border-accent focus:outline-none"
        />
        <span className="mt-1 block text-xs text-ink-3">
          days. After that, open questions close unanswered and the fit is issued from what is evidenced. You are
          reminded a day before.
        </span>
      </label>
      <p className="text-xs leading-relaxed text-ink-3">
        Takes effect for the next review. A review already waiting keeps the limit it was given. An evaluation run never
        waits.
      </p>
      <div className="flex flex-wrap items-center gap-4">
        <Submit />
        {state.error ? <span className="text-sm text-danger">{state.error}</span> : null}
        {state.saved ? <span className="text-sm text-success">Saved; in force for the next review.</span> : null}
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
