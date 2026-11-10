"use client";

import { useState, useTransition } from "react";

import { upsertCommentAction } from "../actions";

// One step's answer: three buttons that save on the click, and a box
// that opens only when the answer is "happens differently" (or when a
// comment already exists). The answer and the comment live on the same
// row, so both are always sent together: a click never wipes the text
// and a saved text never clears the click.
export function StepAnswer({
  nodeId,
  question,
  answers,
  differsPrompt,
  initialAnswer,
  initialBody,
}: {
  nodeId: string;
  question: string;
  answers: { value: string; label: string }[];
  differsPrompt: string;
  initialAnswer: string;
  initialBody: string;
}) {
  const [answer, setAnswer] = useState(initialAnswer);
  const [body, setBody] = useState(initialBody);
  const [savedBody, setSavedBody] = useState(initialBody);
  const [note, setNote] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, start] = useTransition();
  const showBox = answer === "differs" || savedBody.trim() !== "" || body.trim() !== "";
  const dirty = body.trim() !== savedBody.trim();
  const id = `wf-${nodeId}`;

  function choose(value: string) {
    const next = value === answer ? "" : value;
    const prev = answer;
    setAnswer(next);
    setNote(null);
    setError(null);
    start(async () => {
      const res = await upsertCommentAction("workflow_node", nodeId, savedBody.trim(), next);
      if (res.error) {
        setAnswer(prev);
        setError(res.error);
        return;
      }
      setNote(next === "" && res.removed ? "Cleared." : "Saved.");
    });
  }

  function saveBody() {
    setNote(null);
    setError(null);
    start(async () => {
      const res = await upsertCommentAction("workflow_node", nodeId, body.trim(), answer);
      if (res.error) {
        setError(res.error);
        return;
      }
      setSavedBody(body);
      setNote(res.removed ? "Removed." : "Saved.");
    });
  }

  return (
    <div className="mt-4">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">{question}</p>
      <div className="mt-2 flex flex-wrap gap-2" role="group" aria-label={`${question} ${nodeId}`}>
        {answers.map((a) => {
          const on = a.value === answer;
          return (
            <button
              key={a.value}
              type="button"
              onClick={() => choose(a.value)}
              disabled={pending}
              aria-pressed={on}
              className={`inline-flex h-[34px] items-center border px-3 text-sm transition-colors disabled:cursor-wait ${
                on ? "border-accent bg-accent text-white" : "border-line-strong bg-canvas text-ink hover:border-accent"
              }`}
            >
              {a.label}
            </button>
          );
        })}
        {note && !dirty ? <span className="self-center font-mono text-[11px] uppercase tracking-[0.14em] text-success">{note}</span> : null}
        {error ? <span className="self-center text-xs text-signal">{error}</span> : null}
      </div>
      {showBox ? (
        <div className="mt-3">
          <label htmlFor={id} className="block text-sm text-ink">
            {differsPrompt}
          </label>
          <textarea
            id={id}
            value={body}
            onChange={(e) => setBody(e.target.value)}
            rows={2}
            maxLength={4000}
            className="mt-1 block w-full border border-line-strong bg-canvas px-3 py-2 text-sm text-ink outline-none focus:border-accent"
          />
          <div className="mt-2 flex items-center gap-3">
            <button
              type="button"
              onClick={saveBody}
              disabled={pending || !dirty}
              className="inline-flex h-[30px] items-center border border-accent px-3 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
            >
              {pending ? "Saving…" : savedBody.trim() && !body.trim() ? "Remove" : "Save"}
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
