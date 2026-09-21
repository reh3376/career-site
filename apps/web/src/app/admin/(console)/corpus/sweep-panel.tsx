"use client";

import { useState, useTransition } from "react";

import { sweepCorpusEmbeddingsAction, type SweepResult } from "./actions";

// Re-embeds stale chunks (missing vector, or produced by a different
// embedder than the one the sidecar is serving now). Bounded per
// click; keep clicking until remaining reads 0.
export function SweepPanel({
  embedderCounts,
}: {
  embedderCounts: { model: string; count: number }[];
}) {
  const [result, setResult] = useState<SweepResult | null>(null);
  const [isPending, startTransition] = useTransition();

  function run(): void {
    setResult(null);
    startTransition(async () => {
      setResult(await sweepCorpusEmbeddingsAction());
    });
  }

  return (
    <div className="grid gap-4 border border-line-strong bg-canvas p-5">
      <div className="flex flex-wrap items-start gap-6">
        <div>
          <button
            type="button"
            disabled={isPending}
            onClick={run}
            className="inline-flex h-9 items-center border border-accent bg-accent px-4 font-mono text-[11px] uppercase tracking-[0.14em] text-white transition-colors hover:bg-accent-strong disabled:cursor-not-allowed disabled:opacity-50"
          >
            {isPending ? "Embedding..." : "Embed sweep"}
          </button>
          <p className="mt-2 max-w-md text-[11px] leading-relaxed text-ink-3">
            Re-embeds up to 512 chunks whose vector is missing or came
            from a different embedder than the sidecar serves now. Run
            after flipping SIDECAR_EMBED_PROVIDER, until remaining is 0.
          </p>
        </div>
        {embedderCounts.length > 0 ? (
          <dl className="font-mono text-[11px] text-ink-2">
            <dt className="uppercase tracking-[0.14em] text-ink-3">
              chunks by embedder
            </dt>
            {embedderCounts.map((c) => (
              <dd key={c.model || "none"} className="m-0 mt-1">
                <span className={c.model ? "text-ink" : "text-signal"}>
                  {c.model || "not embedded"}
                </span>
                <span className="text-ink-4"> · </span>
                {c.count}
              </dd>
            ))}
          </dl>
        ) : null}
      </div>

      {result ? (
        result.ok ? (
          <div
            className={
              "border-l-2 px-4 py-3 text-sm text-ink " +
              (result.failed > 0
                ? "border-signal bg-signal-soft/50"
                : "border-success bg-success-soft/50")
            }
          >
            <p className="font-mono text-[11px] uppercase tracking-[0.14em]">
              swept · {result.model}
            </p>
            <p className="mt-1 font-mono text-xs">
              considered {result.considered}
              <span className="text-ink-4"> · </span>
              embedded {result.embedded}
              {result.failed > 0 ? (
                <>
                  <span className="text-ink-4"> · </span>
                  <span className="text-signal">failed {result.failed}</span>
                </>
              ) : null}
              <span className="text-ink-4"> · </span>
              remaining{" "}
              <span
                className={result.remaining === 0 ? "text-success" : "text-signal"}
              >
                {result.remaining}
              </span>
            </p>
            {result.remaining > 0 ? (
              <p className="mt-2 text-xs text-ink-3">
                Not converged yet. Run the sweep again.
              </p>
            ) : null}
          </div>
        ) : (
          <div className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
            {result.error}
          </div>
        )
      ) : null}
    </div>
  );
}
