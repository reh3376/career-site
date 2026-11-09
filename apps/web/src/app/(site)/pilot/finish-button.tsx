"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { finishPilotAction, type FinishState } from "./actions";

const initial: FinishState = {};

// Closes the session. Offered once the survey is complete; the two
// reading parts may be unfinished, and the button says so rather than
// blocking, because a participant who read neither has still given us
// the survey, which is the pilot's first purpose.
export function FinishButton({ recordDone, workflowDone }: { recordDone: boolean; workflowDone: boolean }) {
  const [state, formAction] = useActionState(finishPilotAction, initial);
  const missing = [!recordDone ? "the match record" : null, !workflowDone ? "the workflow" : null].filter(Boolean);
  return (
    <form action={formAction} className="space-y-3">
      {state.error ? (
        <p role="alert" className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          {state.error}
        </p>
      ) : null}
      {missing.length ? (
        <p className="text-sm text-ink-3">
          You have not marked {missing.join(" or ")} as read. You can finish anyway and come back to comment later.
        </p>
      ) : null}
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
      {pending ? "Finishing…" : "Finish the pilot"}
    </button>
  );
}
