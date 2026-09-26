"use client";

import { useState } from "react";

import { JobDetailDialog } from "./job-detail";

export type Job = {
  id?: string;
  kind?: string;
  status?: string;
  progress?: number;
  summary?: string;
  startedAt?: string;
  finishedAt?: string;
};

function when(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime())
    ? ""
    : d.toISOString().slice(11, 16) + " UTC";
}

// The rows are buttons because each one opens its own timeline. The
// summary shown here is only the job's latest report; every earlier one
// is behind the click, and for a run measured in hours those are the
// interesting ones.
export function JobList({ jobs }: { jobs: Job[] }) {
  const [openId, setOpenId] = useState<string | null>(null);
  const open = jobs.find((j) => j.id === openId);

  if (jobs.length === 0) {
    return (
      <p className="text-sm text-ink-3">
        None. The runner forgets finished jobs after a day, and forgets
        everything when the api restarts.
      </p>
    );
  }

  return (
    <>
      <ul className="space-y-px">
        {jobs.map((j) => (
          <li key={j.id}>
            <button
              type="button"
              onClick={() => setOpenId(j.id ?? null)}
              aria-haspopup="dialog"
              className="block w-full bg-paper-2 px-4 py-3 text-left transition-colors hover:bg-paper-3 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            >
              <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
                <p className="font-mono text-sm text-ink">
                  {j.kind}
                  <span className="ml-3 text-ink-3">{j.status}</span>
                  {!j.finishedAt && (j.progress ?? 0) > 0 ? (
                    <span className="ml-3 text-accent">{j.progress}%</span>
                  ) : null}
                </p>
                <p className="font-mono text-[11px] text-ink-4">
                  {when(j.startedAt)}
                  {j.finishedAt ? ` to ${when(j.finishedAt)}` : ""}
                </p>
              </div>
              {j.summary ? (
                <p className="mt-1 text-sm leading-relaxed text-ink-2">
                  {j.summary}
                </p>
              ) : null}
            </button>
          </li>
        ))}
      </ul>
      {open?.id ? (
        <JobDetailDialog
          jobId={open.id}
          label={`${open.kind ?? "job"} · ${open.status ?? ""}`}
          onClose={() => setOpenId(null)}
        />
      ) : null}
    </>
  );
}
