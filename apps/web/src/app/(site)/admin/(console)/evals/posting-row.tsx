"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import {
  labelGoldenAction,
  setGoldenActiveAction,
  type LabelState,
} from "./actions";

// One posting in the golden set, with its full text available.
//
// The text is here because a posting cannot be labelled without being
// read, and a posting drawn at random arrives with nobody having read
// it. An unlabelled posting is a question waiting for an answer, not a
// gap: evaluations skip it rather than guessing, so it costs nothing to
// leave one sitting until there is time to judge it properly.

export type Posting = {
  id?: string | number;
  name?: string;
  jd_text?: string;
  jdText?: string;
  role_hint?: string;
  roleHint?: string;
  employer_hint?: string;
  employerHint?: string;
  expected_gate?: string;
  expectedGate?: string;
  selection?: string;
  source?: string;
  note?: string;
  active?: boolean;
  last_score?: number;
  lastScore?: number;
  last_passed?: boolean;
  lastPassed?: boolean;
};

export function PostingRow({ p }: { p: Posting }) {
  const id = String(p.id);
  const gate = p.expected_gate ?? p.expectedGate ?? "";
  const text = p.jd_text ?? p.jdText ?? "";
  const role = p.role_hint ?? p.roleHint ?? "";
  const employer = p.employer_hint ?? p.employerHint ?? "";
  const last = p.last_score ?? p.lastScore;
  const passed = p.last_passed ?? p.lastPassed;
  const active = p.active !== false;
  const unlabelled = gate === "";

  return (
    <li
      className={
        "border-y border-line py-4 " +
        (unlabelled ? "border-l-2 border-l-accent pl-4" : "")
      }
    >
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <div className="min-w-0">
          <p className="text-sm text-ink">
            {p.name}
            <span className="ml-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              {p.selection === "random" ? "random" : "chosen"}
              {unlabelled ? " · needs a label" : ` · expect ${gate}`}
              {!active ? " · retired" : ""}
            </span>
          </p>
          {role || employer ? (
            <p className="mt-0.5 text-sm text-ink-2">
              {role}
              {employer ? ` at ${employer}` : ""}
            </p>
          ) : null}
          {p.note ? (
            <p className="mt-0.5 text-sm text-ink-3">{p.note}</p>
          ) : null}
          {p.source ? (
            <p className="mt-0.5 font-mono text-[11px] text-ink-4">
              {p.source}
            </p>
          ) : null}
        </div>
        <div className="flex items-center gap-4">
          <span className="font-mono text-sm text-ink">
            {last === undefined ? (
              <span className="text-ink-4">not scored</span>
            ) : (
              <>
                {Number(last).toFixed(3)}{" "}
                <span className={passed ? "text-signal" : "text-danger"}>
                  {passed ? "ok" : "miss"}
                </span>
              </>
            )}
          </span>
          <form action={setGoldenActiveAction} className="m-0">
            <input type="hidden" name="id" value={id} />
            <input
              type="hidden"
              name="active"
              value={active ? "false" : "true"}
            />
            <button
              type="submit"
              className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3 underline decoration-line underline-offset-4 hover:text-accent"
            >
              {active ? "retire" : "restore"}
            </button>
          </form>
        </div>
      </div>

      {unlabelled ? <LabelForm id={id} /> : null}

      <details className="mt-3">
        <summary className="cursor-pointer font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          read the posting ({text.length.toLocaleString()} characters)
        </summary>
        <pre className="mt-3 max-h-[32rem] overflow-auto whitespace-pre-wrap border border-line bg-paper-2 p-4 font-mono text-[12px] leading-snug text-ink-2">
          {text}
        </pre>
      </details>
    </li>
  );
}

function LabelForm({ id }: { id: string }) {
  const [state, action] = useActionState<LabelState, FormData>(
    labelGoldenAction,
    {},
  );
  return (
    <form
      action={action}
      className="mt-3 border-l-2 border-accent bg-paper-2 px-4 py-3"
    >
      <input type="hidden" name="id" value={id} />
      <p className="text-sm text-ink-2">
        One question only: does the candidate have the skill set to do this job?
        Not whether the level, the pay, the location or the hours suit him,
        which are parameter filters and not this system&rsquo;s business. A role
        well below his level still gets a yes if he can do it.
      </p>
      <label className="mt-2 block">
        <span className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-4">
          why, in a sentence (optional)
        </span>
        <input
          type="text"
          name="note"
          className="mt-1 block w-full border border-line bg-canvas px-3 py-2 text-sm text-ink"
        />
      </label>
      <div className="mt-3 flex flex-wrap items-center gap-2">
        <Choice value="above" label="Yes, he can do it" />
        <Choice value="below" label="No, he cannot" />
        {state.error ? (
          <span className="text-sm text-danger">{state.error}</span>
        ) : null}
        {state.saved ? (
          <span className="text-sm text-signal">Labelled.</span>
        ) : null}
      </div>
    </form>
  );
}

function Choice({ value, label }: { value: string; label: string }) {
  const { pending } = useFormStatus();
  return (
    <button
      type="submit"
      name="expected_gate"
      value={value}
      disabled={pending}
      className="border border-line px-3 py-1.5 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2 transition-colors hover:border-accent hover:text-accent disabled:opacity-50"
    >
      {label}
    </button>
  );
}
