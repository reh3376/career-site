"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { setPilotReviewAction, type ReviewState } from "../actions";

// The owner's curation judgement on a sitting (FSD FR-PLT-17):
// unreviewed, reviewed (vouched for), or excluded with the reason. The
// list and the exports carry it, so a reader can filter on what has
// been read and judged.
export function ReviewForm({ sessionId, status, note }: { sessionId: string; status: string; note: string }) {
  const [state, formAction] = useActionState(setPilotReviewAction, {} as ReviewState);
  return (
    <form action={formAction} className="mt-4 max-w-xl space-y-3">
      <input type="hidden" name="id" value={sessionId} />
      {state.error ? (
        <p role="alert" className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          {state.error}
        </p>
      ) : null}
      <div className="flex flex-wrap gap-4">
        {(["unreviewed", "reviewed", "excluded"] as const).map((v) => (
          <label key={v} className="flex items-center gap-2 text-sm text-ink">
            <input type="radio" name="status" value={v} defaultChecked={(status || "unreviewed") === v} className="h-4 w-4" />
            {v}
          </label>
        ))}
      </div>
      <label className="block">
        <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">reason or remark</span>
        <input
          type="text"
          name="note"
          defaultValue={note}
          maxLength={2000}
          placeholder="required when excluding"
          className="mt-1 block w-full border border-line-strong bg-canvas px-3 py-2 text-sm text-ink outline-none focus:border-accent"
        />
      </label>
      <div className="flex items-center gap-3">
        <Submit />
        {state.saved ? <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-success">Saved.</span> : null}
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
      className="inline-flex h-[30px] items-center border border-accent px-3 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
    >
      {pending ? "Saving…" : "Save judgement"}
    </button>
  );
}
