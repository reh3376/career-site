"use client";

import { useActionState, useState } from "react";
import { useFormStatus } from "react-dom";

import { statementBody } from "@/lib/match-record";

import { answerItemAction, withdrawAnswerAction, type AnswerState, type WithdrawState } from "../actions";
import { when, type ApiField, type ApiItem } from "../lib";

// One follow-up question and its answer (FSD FR-HF-13, FR-HF-14). The
// card shows the posting's own words, the question, and the inputs the
// question offers. There are always three ways to answer: an answer,
// "I have not done this", and "prefer not to answer"; none of them is
// the default and none is hidden.

const mono = "font-mono text-[11px] uppercase tracking-[0.14em]";
const input = "mt-1 block w-full border border-line bg-transparent px-3 py-2 text-sm text-ink focus:border-accent focus:outline-none";
const primary =
  "inline-flex items-center border border-accent px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-50";
const quiet =
  "inline-flex items-center border border-line-strong px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2 transition-colors hover:border-ink hover:text-ink disabled:cursor-not-allowed disabled:opacity-50";

const STATE: Record<string, { label: string; tone: string }> = {
  pending: { label: "not answered yet", tone: "text-signal" },
  answered: { label: "answered", tone: "text-success" },
  not_applicable: { label: "you have not done this", tone: "text-ink-2" },
  declined: { label: "you preferred not to answer", tone: "text-ink-2" },
  withdrawn: { label: "answer withdrawn", tone: "text-ink-3" },
};

export function QuestionCard({ item, index, total, open }: { item: ApiItem; index: number; total: number; open: boolean }) {
  const status = item.answer?.status ?? "pending";
  const state = STATE[status] ?? STATE.pending;
  const quote = (item.sourceQuote ?? "").replace(/^…/, "");
  return (
    <li className="border border-line-strong bg-canvas p-5 sm:p-6">
      <div className={`flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 ${mono} text-ink-3`}>
        <span>
          question {index} of {total} <span className="text-ink-4">·</span> requirement {item.requirementLocalId}
        </span>
        <span className={state.tone}>{state.label}</span>
      </div>

      <p className={`mt-5 ${mono} text-ink-3`}>The posting says</p>
      <p className="mt-1 text-base italic leading-relaxed text-ink-2">&ldquo;{quote}&rdquo;</p>

      <p className={`mt-5 ${mono} text-ink-3`}>The question</p>
      <p className="mt-1 text-base leading-relaxed text-ink">{item.question}</p>

      {open ? <AnswerForm item={item} /> : <Closed item={item} />}
    </li>
  );
}

function valuesOf(item: ApiItem, name: string): string[] {
  return item.answer?.values?.[name]?.values ?? [];
}

function AnswerForm({ item }: { item: ApiItem }) {
  const [state, action] = useActionState<AnswerState, FormData>(answerItemAction, {});
  const max = item.freeTextMax ?? 600;
  const [note, setNote] = useState(item.answer?.freeText ?? "");
  const status = item.answer?.status ?? "pending";
  return (
    <form action={action} className="mt-5">
      <input type="hidden" name="item_id" value={item.id ?? ""} />
      <div className="grid gap-5">
        {(item.fields ?? []).map((f) => (
          <FieldInput key={f.name} field={f} chosen={valuesOf(item, f.name ?? "")} />
        ))}
        <label className="block">
          <span className={`${mono} text-ink-3`}>
            In your own words{item.freeTextRequired ? "" : " (optional)"}
          </span>
          <textarea
            name="free_text"
            rows={4}
            maxLength={max}
            value={note}
            onChange={(e) => setNote(e.target.value)}
            className={input}
          />
          <span className="mt-1 flex justify-between text-xs text-ink-3">
            <span>
              {item.freeTextRequired
                ? "Say in a few sentences what you did there. It is kept as you write it."
                : "Anything that helps a reader place the answer. It is kept as you write it."}
            </span>
            <span className="font-mono">
              {note.length}/{max}
            </span>
          </span>
        </label>
      </div>

      <div className="mt-5 flex flex-wrap items-center gap-3">
        <Press intent="answered" className={primary} idle={status === "answered" ? "Save the changed answer" : "Save this answer"} />
        <Press intent="not_applicable" className={quiet} idle="I have not done this" />
        <Press intent="declined" className={quiet} idle="Prefer not to answer" />
      </div>
      <p className="mt-3 max-w-2xl text-xs leading-relaxed text-ink-3">
        &ldquo;I have not done this&rdquo; settles the requirement as not met on your word. &ldquo;Prefer not to
        answer&rdquo; leaves it unresolved and the question is not asked again.
      </p>
      {state.error ? <p className="mt-3 border-l-2 border-danger pl-3 text-sm text-danger">{state.error}</p> : null}
      {!state.error && status !== "pending" && item.answeredAt ? (
        <p className="mt-3 text-sm text-ink-2">
          Saved {when(item.answeredAt)}. You can change it until you submit the answers.
        </p>
      ) : null}
      {item.statement ? <Statement text={item.statement} /> : null}
    </form>
  );
}

function Press({ intent, className, idle }: { intent: string; className: string; idle: string }) {
  const { pending } = useFormStatus();
  return (
    <button type="submit" name="intent" value={intent} disabled={pending} formNoValidate className={className}>
      {pending ? "Saving..." : idle}
    </button>
  );
}

function FieldInput({ field, chosen }: { field: ApiField; chosen: string[] }) {
  const name = `f:${field.name ?? ""}`;
  const options = field.options ?? [];
  const legend = (
    <span className={`${mono} text-ink-3`}>
      {field.label}
      {field.required ? "" : " (optional)"}
    </span>
  );
  if (field.kind === "multi" || (field.kind === "choice" && options.length <= 6)) {
    const type = field.kind === "multi" ? "checkbox" : "radio";
    return (
      <fieldset>
        <legend>{legend}</legend>
        {options.length === 0 ? (
          <p className="mt-1 text-sm text-ink-3">Nothing to choose from here. Use the box below.</p>
        ) : (
          <div className="mt-2 grid gap-2">
            {options.map((o) => (
              <label key={o.value} className="flex items-start gap-3 text-sm leading-relaxed text-ink">
                <input
                  type={type}
                  name={name}
                  value={o.value}
                  defaultChecked={chosen.includes(o.value ?? "")}
                  className="mt-1 accent-accent"
                />
                <span>{o.label}</span>
              </label>
            ))}
          </div>
        )}
        {field.hint ? <p className="mt-1 text-xs text-ink-3">{field.hint}</p> : null}
      </fieldset>
    );
  }
  if (field.kind === "choice") {
    return (
      <label className="block sm:max-w-xl">
        {legend}
        <select name={name} defaultValue={chosen[0] ?? ""} className={input}>
          <option value="">Choose one</option>
          {options.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
        {field.hint ? <span className="mt-1 block text-xs text-ink-3">{field.hint}</span> : null}
      </label>
    );
  }
  if (field.kind === "number") {
    return (
      <label className="block sm:max-w-xs">
        {legend}
        <input
          name={name}
          type="number"
          step="any"
          min="0"
          max={field.max && field.max > 0 ? field.max : undefined}
          defaultValue={chosen[0] ?? ""}
          className={`${input} font-mono`}
        />
        {field.hint ? <span className="mt-1 block text-xs text-ink-3">{field.hint}</span> : null}
      </label>
    );
  }
  return (
    <label className="block sm:max-w-xl">
      {legend}
      <input
        name={name}
        type="text"
        maxLength={field.maxLen && field.maxLen > 0 ? field.maxLen : undefined}
        defaultValue={chosen[0] ?? ""}
        className={input}
      />
      {field.hint ? <span className="mt-1 block text-xs text-ink-3">{field.hint}</span> : null}
    </label>
  );
}

function Statement({ text }: { text: string }) {
  return (
    <div className="mt-5 border-l-2 border-line-strong pl-4">
      <p className={`${mono} text-ink-3`}>Your answer as a reader of the record sees it</p>
      <p className="mt-1 text-sm leading-relaxed text-ink-2">{statementBody(text)}</p>
    </div>
  );
}

// A closed questionnaire: the answer as it was given, and the one thing
// still open to the candidate, which is to take it back.
function Closed({ item }: { item: ApiItem }) {
  const status = item.answer?.status ?? "pending";
  const [state, action] = useActionState<WithdrawState, FormData>(withdrawAnswerAction, {});
  if (status === "pending") {
    return <p className="mt-5 text-sm text-ink-3">This question closed without an answer.</p>;
  }
  return (
    <div className="mt-2">
      {item.statement ? <Statement text={item.statement} /> : null}
      {status === "answered" || status === "not_applicable" ? (
        <form action={action} className="mt-5 flex flex-wrap items-center gap-4">
          <input type="hidden" name="item_id" value={item.id ?? ""} />
          <Withdraw />
          <span className="max-w-xl text-xs leading-relaxed text-ink-3">
            Taking an answer back stops it being evidence. A verdict that rests on it is marked stale until the posting
            is scored again.
          </span>
          {state.error ? <span className="text-sm text-danger">{state.error}</span> : null}
        </form>
      ) : null}
      {status === "withdrawn" ? (
        <p className="mt-5 text-sm text-ink-3">You withdrew this answer. It is no longer evidence.</p>
      ) : null}
    </div>
  );
}

function Withdraw() {
  const { pending } = useFormStatus();
  return (
    <button type="submit" disabled={pending} className={quiet}>
      {pending ? "Withdrawing..." : "Withdraw this answer"}
    </button>
  );
}
