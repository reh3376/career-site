"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { deletePilotSessionAction, type DeleteState } from "../actions";

const initial: DeleteState = {};

// The one destructive control on the pilot pages. A native confirm on
// submit, as the whitelist's remove button does, so a stray click does
// not take a session with it; the server action redirects to the list
// on success and reports the API's message otherwise.
export function DeleteSessionButton({ id, who }: { id: string; who: string }) {
  const [state, formAction] = useActionState(deletePilotSessionAction, initial);
  return (
    <form
      action={formAction}
      onSubmit={(e) => {
        if (
          !window.confirm(
            `Delete the pilot session for ${who}? Every answer, flag, comment and timing in it goes too. This cannot be undone.`,
          )
        ) {
          e.preventDefault();
        }
      }}
      className="m-0"
    >
      <input type="hidden" name="id" value={id} />
      <div className="flex flex-wrap items-center gap-3">
        <Submit />
        {state.error ? (
          <span role="alert" className="text-xs text-signal">
            {state.error}
          </span>
        ) : null}
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
      className="inline-flex h-[30px] items-center border border-signal px-3 font-mono text-[11px] uppercase tracking-[0.14em] text-signal transition-colors hover:bg-signal hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
    >
      {pending ? "Deleting…" : "Delete this session"}
    </button>
  );
}
