"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import {
  addQaPhrasingAction,
  deleteQaPhrasingAction,
  type QaState,
} from "./actions";

export type Phrasing = {
  id: string;
  text: string;
  canonical: boolean;
  embedded: boolean;
};

// The ways one entry can be asked.
//
// Shown with whether each has an embedding yet, because a phrasing
// without one never matches. That is the quietest failure in the whole
// feature: the entry is present, correct, approved, and unreachable,
// and nothing about the answer a visitor gets would say so.
export function Phrasings({
  entryId,
  phrasings,
}: {
  entryId: string;
  phrasings: Phrasing[];
}) {
  const [state, action] = useActionState<QaState, FormData>(
    addQaPhrasingAction,
    {},
  );

  return (
    <div className="mt-4 border-t border-line pt-4">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        ways it gets asked
      </p>
      <ul className="mt-2 space-y-1">
        {phrasings.map((p) => (
          <li key={p.id} className="flex items-baseline gap-3 text-sm">
            <span className={p.embedded ? "text-ink" : "text-ink-3"}>
              {p.text}
            </span>
            {p.canonical ? (
              <span className="font-mono text-[11px] text-ink-3">
                the question
              </span>
            ) : null}
            {p.embedded ? null : (
              <span className="font-mono text-[11px] text-danger">
                not embedded yet, cannot match
              </span>
            )}
            {p.canonical ? null : (
              <form action={deleteQaPhrasingAction} className="inline">
                <input type="hidden" name="entry_id" value={entryId} />
                <input type="hidden" name="phrasing_id" value={p.id} />
                <button
                  type="submit"
                  className="font-mono text-[11px] text-ink-3 underline underline-offset-2 hover:text-danger"
                >
                  remove
                </button>
              </form>
            )}
          </li>
        ))}
      </ul>

      <form action={action} className="mt-3 flex flex-col gap-2 sm:flex-row">
        <input type="hidden" name="entry_id" value={entryId} />
        <input
          name="text"
          maxLength={500}
          placeholder="another wording people use"
          className="w-full border border-line bg-transparent px-3 py-2 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
        <AddButton />
      </form>
      {state.error ? (
        <p className="mt-2 text-sm text-danger">{state.error}</p>
      ) : null}
    </div>
  );
}

function AddButton() {
  const { pending } = useFormStatus();
  return (
    <button
      type="submit"
      disabled={pending}
      className="shrink-0 border border-line px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2 transition-colors hover:border-accent hover:text-accent disabled:opacity-50"
    >
      {pending ? "Adding..." : "Add"}
    </button>
  );
}
