"use client";

import { useState, useTransition } from "react";

import { upsertPilotNoteAction } from "../actions";

// One of the owner's notes on one target: the session, an answer, a
// record row, a workflow step (FSD FR-PLT-16). The call's Parts A to C
// land here, beside what they discuss, instead of in a file. Saves on
// the button; an emptied box removes the note.
export function NoteBox({
  sessionId,
  targetKind,
  targetId,
  initialBody,
  label,
  compact,
}: {
  sessionId: string;
  targetKind: string;
  targetId: string;
  initialBody: string;
  label: string;
  compact?: boolean;
}) {
  const [body, setBody] = useState(initialBody);
  const [saved, setSaved] = useState(initialBody);
  const [note, setNote] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(!compact || initialBody !== "");
  const [pending, start] = useTransition();
  const dirty = body.trim() !== saved.trim();
  const id = `n-${targetKind}-${targetId || "session"}`;

  function save() {
    setError(null);
    setNote(null);
    start(async () => {
      const res = await upsertPilotNoteAction(sessionId, targetKind, targetId, body.trim());
      if (res.error) {
        setError(res.error);
        return;
      }
      setSaved(body);
      setNote(res.removed ? "Removed." : "Saved.");
    });
  }

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="mt-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3 hover:text-accent"
      >
        + note
      </button>
    );
  }

  return (
    <div className="mt-2">
      <label htmlFor={id} className="block font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        {label}
      </label>
      <textarea
        id={id}
        value={body}
        onChange={(e) => setBody(e.target.value)}
        rows={compact ? 2 : 4}
        maxLength={8000}
        placeholder="Your note"
        className="mt-1 block w-full border border-line-strong bg-canvas px-3 py-2 text-sm text-ink outline-none focus:border-accent"
      />
      <div className="mt-2 flex items-center gap-3">
        <button
          type="button"
          onClick={save}
          disabled={pending || !dirty}
          className="inline-flex h-[30px] items-center border border-accent px-3 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
        >
          {pending ? "Saving…" : saved.trim() && !body.trim() ? "Remove note" : "Save note"}
        </button>
        {note && !dirty ? <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-success">{note}</span> : null}
        {error ? <span className="text-xs text-signal">{error}</span> : null}
      </div>
    </div>
  );
}
