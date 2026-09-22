"use client";

import { useEffect, useRef, useState } from "react";

import { getJobAction, runJobAction, type JobState } from "./actions";

const JOBS = [
  {
    kind: "JOB_KIND_CORPUS_REINDEX_PUBLIC",
    label: "Reindex public content",
    hint: "apps/web/content, ships with the deploy. Documents land as public.",
  },
  {
    kind: "JOB_KIND_CORPUS_REINDEX_PRIVATE",
    label: "Reindex private corpus",
    hint: "the make sync-corpus mount. Documents land as corpus-only and are never quoted on the site.",
  },
  {
    kind: "JOB_KIND_EMBED_SWEEP",
    label: "Embed sweep",
    hint: "Re-embeds every chunk whose vector is missing or came from a different embedder, in batches, until none remain.",
  },
] as const;

// Long corpus operations run as api-side jobs: the click returns a job
// id at once and this panel polls it. On the production box a private
// reindex or a full sweep runs for minutes, longer than a synchronous
// response can wait behind the proxy.
export function JobPanel({
  embedderCounts,
}: {
  embedderCounts: { model: string; count: number }[];
}) {
  const [job, setJob] = useState<JobState | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [starting, setStarting] = useState<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (timer.current) clearTimeout(timer.current);
    };
  }, []);

  const live = job?.status === "JOB_STATUS_QUEUED" || job?.status === "JOB_STATUS_RUNNING";

  function poll(jobId: string): void {
    timer.current = setTimeout(async () => {
      const next = await getJobAction(jobId);
      if (!next.ok) {
        setError(next.error);
        return;
      }
      setJob(next.job);
      if (next.job.status === "JOB_STATUS_QUEUED" || next.job.status === "JOB_STATUS_RUNNING") {
        poll(jobId);
      }
    }, 2000);
  }

  async function start(kind: string): Promise<void> {
    setError(null);
    setStarting(kind);
    const res = await runJobAction(kind);
    setStarting(null);
    if (!res.ok) {
      setError(res.error);
      return;
    }
    setJob({ jobId: res.jobId, kind, status: "JOB_STATUS_QUEUED", progressPct: 0, summary: "" });
    poll(res.jobId);
  }

  return (
    <div className="grid gap-4 border border-line-strong bg-canvas p-5">
      <div className="grid gap-4 sm:grid-cols-3">
        {JOBS.map((j) => (
          <div key={j.kind}>
            <button
              type="button"
              disabled={live || starting !== null}
              onClick={() => void start(j.kind)}
              className="inline-flex h-9 items-center border border-accent bg-accent px-4 font-mono text-[11px] uppercase tracking-[0.14em] text-white transition-colors hover:bg-accent-strong disabled:cursor-not-allowed disabled:opacity-50"
            >
              {starting === j.kind ? "Starting..." : j.label}
            </button>
            <p className="mt-2 text-[11px] leading-relaxed text-ink-3">{j.hint}</p>
          </div>
        ))}
      </div>

      {embedderCounts.length > 0 ? (
        <dl className="font-mono text-[11px] text-ink-2">
          <dt className="uppercase tracking-[0.14em] text-ink-3">chunks by embedder</dt>
          {embedderCounts.map((c) => (
            <dd key={c.model || "none"} className="m-0 mt-1">
              <span className={c.model ? "text-ink" : "text-signal"}>{c.model || "not embedded"}</span>
              <span className="text-ink-4"> · </span>
              {c.count}
            </dd>
          ))}
        </dl>
      ) : null}

      {error ? (
        <div className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">{error}</div>
      ) : null}

      {job ? (
        <div
          className={
            "border-l-2 px-4 py-3 text-sm text-ink " +
            (job.status === "JOB_STATUS_FAILED"
              ? "border-danger bg-signal-soft/50"
              : job.status === "JOB_STATUS_SUCCEEDED"
                ? "border-success bg-success-soft/50"
                : "border-accent bg-accent/10")
          }
        >
          <p className="font-mono text-[11px] uppercase tracking-[0.14em]">
            {job.kind.replace("JOB_KIND_", "").replace(/_/g, " ").toLowerCase()}
            <span className="text-ink-4"> · </span>
            {job.status.replace("JOB_STATUS_", "").toLowerCase()}
            {live ? (
              <>
                <span className="text-ink-4"> · </span>
                {job.progressPct}%
              </>
            ) : null}
          </p>
          {live ? (
            <div className="mt-2 h-1 w-full bg-line">
              <div className="h-1 bg-accent transition-all" style={{ width: `${job.progressPct}%` }} />
            </div>
          ) : null}
          {job.summary ? <p className="mt-2 font-mono text-xs text-ink-2">{job.summary}</p> : null}
          {live ? (
            <p className="mt-2 text-xs text-ink-3">
              Running on the box. You can leave this page; the job keeps going and the numbers above refresh when you return.
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
