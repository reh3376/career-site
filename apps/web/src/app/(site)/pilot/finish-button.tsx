"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { finishPilotAction, type FinishState } from "./actions";

const initial: FinishState = {};

// Closes the session. Offered only once all three parts are done: the
// first live run (2026-11-09) showed that a Finish offered earlier is
// taken and the readings never happen, so the home page does not render
// this until then and the API refuses to close a session before then.
export function FinishButton() {
  const [state, formAction] = useActionState(finishPilotAction, initial);
  return (
    <form action={formAction} className="space-y-3">
      {state.error ? (
        <p role="alert" className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          {state.error}
        </p>
      ) : null}
      <p className="text-sm text-ink-3">
        All three parts are done. Finishing closes your session; you can still come back to the record and the workflow
        to change comments.
      </p>
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
