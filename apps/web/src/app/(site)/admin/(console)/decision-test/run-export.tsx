"use client";

import { useState } from "react";

import { exportDecisionTestRunAction } from "./actions";

// Downloading one run.
//
// A client component for the same reason the dataset export is one: a
// server action can return the bytes but cannot hand the browser a file
// to save, so the action fetches and this builds the download.
//
// Two files rather than one, because the page shows two tables and they
// are different shapes. A block row and an answer row stacked into one
// CSV would need a type column and a lot of empty cells, and whatever
// opened it would have to know to split them again.
//
// Both carry the full session context on every row. These files leave
// the console and get opened somewhere else, where "block 3, 4 correct"
// with no run key, no version and no curation status is not a
// measurement of anything.

type Result = {
  filenameStem: string;
  blocksCsv: string;
  answersCsv: string;
  blockRows: number;
  answerRows: number;
};

function save(name: string, csv: string) {
  const url = URL.createObjectURL(new Blob([csv], { type: "text/csv;charset=utf-8" }));
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  URL.revokeObjectURL(url);
}

export function RunExport({ sessionKey }: { sessionKey: string }) {
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState<{ blocks: number; answers: number } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [includeKey, setIncludeKey] = useState(false);

  async function download() {
    setBusy(true);
    setError(null);
    setDone(null);
    try {
      const r = (await exportDecisionTestRunAction(sessionKey, includeKey)) as Result | null;
      if (!r) {
        setError("The export did not come back. Check the api logs.");
        return;
      }
      if (r.answerRows === 0 && r.blockRows === 0) {
        setError("This run has no answers yet, so there is nothing to download.");
        return;
      }
      save(`${r.filenameStem}-blocks.csv`, r.blocksCsv);
      save(`${r.filenameStem}-answers.csv`, r.answersCsv);
      setDone({ blocks: r.blockRows, answers: r.answerRows });
    } catch (e) {
      setError(e instanceof Error ? e.message : "The export failed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mt-10 max-w-2xl rounded-md border border-line bg-paper-2 px-6 py-5">
      <h2 className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
        Download this run
      </h2>
      <p className="mt-4 text-sm leading-relaxed text-ink-2">
        Two files: the by-block table and the every-answer table, each
        carrying this run&rsquo;s full context on every row so they can be
        read on their own. Built from exactly what this page was given, so
        the numbers in the file are the numbers above.
      </p>

      {/* The same opt-in as the collapsed section on this page, for the
          same reason. A file is easier to forward than a screen. */}
      <label className="mt-5 flex items-start gap-3 text-sm text-ink-2">
        <input
          type="checkbox"
          checked={includeKey}
          onChange={(e) => setIncludeKey(e.target.checked)}
          className="mt-1"
        />
        <span>
          Include the questions and the correct answers. Off by default:
          an answer key that escapes contaminates a standardized
          instrument permanently, and it only has to escape once.
        </span>
      </label>

      <button
        type="button"
        onClick={download}
        disabled={busy}
        className="mt-6 rounded-md bg-accent px-8 py-3 font-mono text-[11px] tracking-[0.14em] text-canvas uppercase disabled:opacity-50"
      >
        {busy ? "Building" : "Download CSV"}
      </button>

      {done ? (
        <p className="mt-4 text-sm text-ink-2">
          Saved two files: {done.blocks}{" "}
          {done.blocks === 1 ? "block" : "blocks"} and {done.answers}{" "}
          {done.answers === 1 ? "answer" : "answers"}.
        </p>
      ) : null}
      {error ? <p className="mt-4 text-sm text-signal">{error}</p> : null}
    </div>
  );
}
