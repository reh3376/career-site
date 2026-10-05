"use client";

import { useState } from "react";

import { exportDecisionTestAction } from "./actions";

// Downloading the dataset.
//
// A client component rather than a plain form because a server action
// cannot hand the browser a file to save: it can return the bytes, but
// turning them into a download needs a Blob and an object URL, which
// only exist on the client. So the action fetches and this builds the
// file.
//
// The alternative, a route handler streaming text/csv, would be the
// right shape for a dataset large enough to stream. Thirty rows per run
// is not that, and a route handler would mean a second admin
// authorisation path to keep in step with the first.

type Result = { filename: string; csv: string; rows: number; sessions: number };

export function ExportPanel() {
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState<{ rows: number; sessions: number } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [synthetic, setSynthetic] = useState(false);

  async function download() {
    setBusy(true);
    setError(null);
    setDone(null);
    try {
      const r = (await exportDecisionTestAction(synthetic)) as Result | null;
      if (!r) {
        setError("The export did not come back. Check the api logs.");
        return;
      }
      if (r.rows === 0) {
        setError(
          synthetic
            ? "No runs at all yet, so there is nothing to export."
            : "No real runs yet. Tick the box to include agent-driven runs.",
        );
        return;
      }
      // Saved rather than opened: a browser shown text/csv inline is a
      // wall of commas, and the point of this button is the file.
      const url = URL.createObjectURL(new Blob([r.csv], { type: "text/csv;charset=utf-8" }));
      const a = document.createElement("a");
      a.href = url;
      a.download = r.filename;
      a.click();
      URL.revokeObjectURL(url);
      setDone({ rows: r.rows, sessions: r.sessions });
    } catch (e) {
      setError(e instanceof Error ? e.message : "The export failed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mt-8 max-w-xl">
      <label className="flex items-start gap-3 text-sm text-ink-2">
        <input
          type="checkbox"
          checked={synthetic}
          onChange={(e) => setSynthetic(e.target.checked)}
          className="mt-1"
        />
        <span>
          Include agent-driven runs. Off by default: they prove the
          pipeline works, and averaged into a claim about people they
          flatter every number in the direction that looks like success.
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
          {done.rows} rows from {done.sessions}{" "}
          {done.sessions === 1 ? "run" : "runs"}.
        </p>
      ) : null}
      {error ? <p className="mt-4 text-sm text-signal">{error}</p> : null}
    </div>
  );
}
