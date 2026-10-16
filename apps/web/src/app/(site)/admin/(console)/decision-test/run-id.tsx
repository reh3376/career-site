"use client";

import { useState } from "react";

// The run's identifier, made visible and copyable.
//
// Every run has had one since the first migration: dt_sessions.public_id,
// the uuid the detail page is addressed by. It was in the URL and nowhere
// on the screen, so the only way to name a run out loud was "the
// anonymous one from Tuesday", which stops working the moment there are
// two. A run that cannot be named cannot be discussed, disputed, or
// excluded on the record.
//
// Copying the full id rather than the abbreviation, because the short
// form is for reading and the long form is for pasting into a query.

export function CopyRunId({ id, short }: { id: string; short?: string }) {
  const [state, setState] = useState<"idle" | "ok" | "fail">("idle");

  async function copy() {
    try {
      await navigator.clipboard.writeText(id);
      setState("ok");
    } catch {
      // Clipboard access is refused outside a secure context and in
      // some embedded views. Saying so beats a button that silently
      // does nothing, because the reader would paste whatever was in
      // the clipboard already.
      setState("fail");
    }
    setTimeout(() => setState("idle"), 1600);
  }

  return (
    <span className="inline-flex items-center gap-2">
      <code className="bg-paper-2 px-1.5 py-0.5 font-mono text-[11px] tracking-[0.06em] text-ink select-all">
        {short ?? id}
      </code>
      <button
        type="button"
        onClick={copy}
        className="font-mono text-[10px] tracking-[0.14em] text-ink-3 uppercase underline decoration-line-strong underline-offset-4 hover:text-accent"
      >
        {state === "ok" ? "copied" : state === "fail" ? "copy failed" : "copy id"}
      </button>
    </span>
  );
}
