"use client";

import { useState, useTransition } from "react";

import { upsertCommentAction } from "./actions";

// One comment on one target (a record row, a workflow node, or a
// general box). Saves on the button, not on every keystroke, because
// a half-typed comment is not a comment; an emptied box removes it.
export function CommentBox({
  targetKind,
  targetId,
  initialBody,
  label,
  placeholder,
  onSaved,
}: {
  targetKind: string;
  targetId: string;
  initialBody: string;
  label: string;
  placeholder?: string;
  // Called after a successful save or removal with the body as stored.
  onSaved?: (body: string) => void;
}) {
  const [body, setBody] = useState(initialBody);
  const [saved, setSaved] = useState(initialBody);
  const [note, setNote] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, start] = useTransition();
  const dirty = body.trim() !== saved.trim();
  const id = `c-${targetKind}-${targetId || "general"}`;

  function save() {
    setError(null);
    setNote(null);
    start(async () => {
      const res = await upsertCommentAction(targetKind, targetId, body.trim());
      if (res.error) {
        setError(res.error);
        return;
      }
      setSaved(body);
      setNote(res.removed ? "Removed." : "Saved.");
      onSaved?.(res.removed ? "" : body.trim());
    });
  }

  return (
    <div className="mt-3">
      <label htmlFor={id} className="block font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        {label}
      </label>
      <textarea
        id={id}
        value={body}
        onChange={(e) => setBody(e.target.value)}
        rows={2}
        maxLength={4000}
        placeholder={placeholder ?? "Your comment"}
        className="mt-1 block w-full border border-line-strong bg-canvas px-3 py-2 text-sm text-ink outline-none focus:border-accent"
      />
      <div className="mt-2 flex items-center gap-3">
        <button
          type="button"
          onClick={save}
          disabled={pending || !dirty}
          className="inline-flex h-[30px] items-center border border-accent px-3 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
        >
          {pending ? "Saving…" : saved.trim() && !body.trim() ? "Remove" : "Save comment"}
        </button>
        {note && !dirty ? <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-success">{note}</span> : null}
        {error ? <span className="text-xs text-signal">{error}</span> : null}
      </div>
    </div>
  );
}
