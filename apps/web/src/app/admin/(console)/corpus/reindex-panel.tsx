"use client";

import { useState, useTransition } from "react";

import { reindexCorpusAction, type ReindexResult } from "./actions";

const SCOPES = [
  {
    key: "public",
    label: "Reindex public content",
    hint: "apps/web/content, ships with the deploy. Documents land as public.",
  },
  {
    key: "private",
    label: "Reindex private corpus",
    hint: "the make sync-corpus mount. Documents land as corpus-only and are never quoted on the site.",
  },
] as const;

// Walks a mounted directory and runs each markdown / text file
// through the corpus ingester. Idempotent, so re-running after a
// re-deploy or a sync only touches files whose content changed.
export function ReindexPanel() {
  const [busyScope, setBusyScope] = useState<string | null>(null);
  const [result, setResult] = useState<{
    scope: string;
    res: ReindexResult;
  } | null>(null);
  const [isPending, startTransition] = useTransition();

  function run(scope: string): void {
    setResult(null);
    setBusyScope(scope);
    startTransition(async () => {
      const res = await reindexCorpusAction(scope);
      setResult({ scope, res });
      setBusyScope(null);
    });
  }

  return (
    <div className="grid gap-4 border border-line-strong bg-canvas p-5">
      <div className="grid gap-4 sm:grid-cols-2">
        {SCOPES.map((s) => {
          const busy = busyScope === s.key && isPending;
          return (
            <div key={s.key}>
              <button
                type="button"
                disabled={isPending}
                onClick={() => run(s.key)}
                className="inline-flex h-9 items-center border border-accent bg-accent px-4 font-mono text-[11px] uppercase tracking-[0.14em] text-white transition-colors hover:bg-accent-strong disabled:cursor-not-allowed disabled:opacity-50"
              >
                {busy ? "Walking..." : s.label}
              </button>
              <p className="mt-2 text-[11px] leading-relaxed text-ink-3">
                {s.hint}
              </p>
            </div>
          );
        })}
      </div>

      {result ? (
        result.res.ok ? (
          <ResultPanel scope={result.scope} r={result.res} />
        ) : (
          <div className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
            {result.res.error}
          </div>
        )
      ) : null}
    </div>
  );
}

function ResultPanel({
  scope,
  r,
}: {
  scope: string;
  r: Extract<ReindexResult, { ok: true }>;
}): React.ReactElement {
  const embedComplete =
    r.chunks_inserted > 0 && r.chunks_embedded === r.chunks_inserted;
  const tone =
    r.errors.length > 0
      ? "border-signal bg-signal-soft/50"
      : "border-success bg-success-soft/50";
  return (
    <div className={"border-l-2 px-4 py-3 text-sm text-ink " + tone}>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em]">
        walked · {scope} · {r.visibility || "?"} · {r.root || "(root unknown)"}
      </p>
      {r.kinds_walked.length > 0 ? (
        <p className="mt-1 font-mono text-xs text-ink-2">
          kinds {r.kinds_walked.join(", ")}
        </p>
      ) : null}
      <p className="mt-1 font-mono text-xs">
        files {r.files_scanned}
        <span className="text-ink-4"> · </span>
        ingested {r.docs_ingested}
        {r.docs_skipped > 0 ? <> (skipped {r.docs_skipped})</> : null}
        <span className="text-ink-4"> · </span>
        chunks {r.chunks_inserted}
        <span className="text-ink-4"> · </span>
        embedded{" "}
        <span className={embedComplete ? "text-success" : "text-signal"}>
          {r.chunks_embedded}
        </span>
      </p>
      {r.files_scanned === 0 && r.errors.length === 0 ? (
        <p className="mt-2 text-xs text-ink-3">
          Nothing to walk. For the private scope, run make sync-corpus
          first; for public, check that apps/web/content is mounted.
        </p>
      ) : null}
      {r.errors.length > 0 ? (
        <details className="mt-3">
          <summary className="cursor-pointer font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
            {r.errors.length} error{r.errors.length === 1 ? "" : "s"}
          </summary>
          <ul className="mt-2 space-y-1 font-mono text-[11px] text-ink-2">
            {r.errors.map((e, i) => (
              <li key={i}>{e}</li>
            ))}
          </ul>
        </details>
      ) : null}
    </div>
  );
}
