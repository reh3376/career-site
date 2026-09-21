"use client";

import { useState, useTransition } from "react";

import { reindexCorpusAction, type ReindexResult } from "./actions";

const KINDS = [{ key: "article", label: "Reindex articles" }] as const;

// Walks a filesystem subdirectory (mounted into the api container
// at /corpus) and runs each markdown file through the corpus
// ingester. Idempotent, so re-running after a re-deploy only touches
// files whose content actually changed.
export function ReindexPanel() {
  const [busyKind, setBusyKind] = useState<string | null>(null);
  const [result, setResult] = useState<{
    kind: string;
    res: ReindexResult;
  } | null>(null);
  const [isPending, startTransition] = useTransition();

  function run(kind: string): void {
    setResult(null);
    setBusyKind(kind);
    startTransition(async () => {
      const res = await reindexCorpusAction(kind);
      setResult({ kind, res });
      setBusyKind(null);
    });
  }

  return (
    <div className="grid gap-4 border border-line-strong bg-canvas p-5">
      <div className="flex flex-wrap gap-3">
        {KINDS.map((k) => {
          const busy = busyKind === k.key && isPending;
          return (
            <button
              key={k.key}
              type="button"
              disabled={isPending}
              onClick={() => run(k.key)}
              className="inline-flex h-9 items-center border border-accent bg-accent px-4 font-mono text-[11px] uppercase tracking-[0.14em] text-white transition-colors hover:bg-accent-strong disabled:cursor-not-allowed disabled:opacity-50"
            >
              {busy ? "Walking..." : k.label}
            </button>
          );
        })}
      </div>
      <p className="text-[11px] text-ink-3">
        Walks the mounted content directory for that source_kind and
        re-runs the ingester on every markdown file. Files whose
        content_hash matches the stored row are skipped.
      </p>

      {result ? (
        result.res.ok ? (
          <ResultPanel kind={result.kind} r={result.res} />
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
  kind,
  r,
}: {
  kind: string;
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
        walked · {kind} · {r.root || "(root unknown)"}
      </p>
      <p className="mt-1 font-mono text-xs">
        files {r.files_scanned}
        <span className="text-ink-4"> · </span>
        ingested {r.docs_ingested}
        {r.docs_skipped > 0 ? (
          <> (skipped {r.docs_skipped})</>
        ) : null}
        <span className="text-ink-4"> · </span>
        chunks {r.chunks_inserted}
        <span className="text-ink-4"> · </span>
        embedded{" "}
        <span className={embedComplete ? "text-success" : "text-signal"}>
          {r.chunks_embedded}
        </span>
      </p>
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
