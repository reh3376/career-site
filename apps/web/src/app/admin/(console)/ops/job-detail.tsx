"use client";

import { useEffect, useRef, useState } from "react";

import { getJobDetailAction, type JobDetail, type JobEvent } from "./actions";

// A job row opens its own timeline.
//
// The list shows a job's latest summary, and each progress report
// overwrites the last, so for a run measured in hours that line is the
// least useful thing about it: an evaluation reads "posting 4 of 9:
// scoring" when what a reader wants is that the first three took 28, 31
// and 26 minutes. The gaps between reports are the measurement. The
// text is only how the job described what it was doing.

function hhmm(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime())
    ? ""
    : d.toISOString().slice(11, 16) + " UTC";
}

// Gap since the previous report, which is the number worth reading.
function gap(from?: string, to?: string): string {
  if (!from || !to) return "";
  const a = new Date(from).getTime();
  const b = new Date(to).getTime();
  if (Number.isNaN(a) || Number.isNaN(b) || b < a) return "";
  const secs = Math.round((b - a) / 1000);
  if (secs < 90) return `+${secs}s`;
  const mins = Math.round(secs / 60);
  if (mins < 90) return `+${mins}m`;
  return `+${(mins / 60).toFixed(1)}h`;
}

function elapsed(startedAt?: string, finishedAt?: string): string {
  if (!startedAt) return "";
  const end = finishedAt ? new Date(finishedAt).getTime() : Date.now();
  const mins = Math.round((end - new Date(startedAt).getTime()) / 60000);
  if (Number.isNaN(mins) || mins < 0) return "";
  return mins < 90 ? `${mins}m` : `${(mins / 60).toFixed(1)}h`;
}

export function JobDetailDialog({
  jobId,
  label,
  onClose,
}: {
  jobId: string;
  label: string;
  onClose: () => void;
}) {
  const [detail, setDetail] = useState<JobDetail | null>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const returnFocusTo = useRef<Element | null>(null);

  useEffect(() => {
    returnFocusTo.current = document.activeElement;
    let stop = false;
    let timer: ReturnType<typeof setTimeout> | undefined;

    // A running job keeps reporting, so the timeline refreshes while it
    // is open, and stops once the job finishes: nothing more will
    // arrive and polling a finished job is only load. Self-scheduling
    // rather than an interval, so a slow response cannot stack calls.
    async function tick(): Promise<void> {
      const d = await getJobDetailAction(jobId);
      if (stop) return;
      setDetail(d);
      const live = d.ok && !d.job.finishedAt;
      if (live) timer = setTimeout(tick, 20000);
    }
    void tick();

    return () => {
      stop = true;
      if (timer) clearTimeout(timer);
      // Put focus back where it was, or the reader is dropped at the
      // top of the document with no idea what they just closed.
      if (returnFocusTo.current instanceof HTMLElement) {
        returnFocusTo.current.focus();
      }
    };
  }, [jobId]);

  useEffect(() => {
    panelRef.current?.focus();
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  const events: JobEvent[] = detail?.ok ? detail.events : [];

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="job-detail-title"
      className="fixed inset-0 z-50 flex items-end justify-center bg-ink/60 p-4 sm:items-center"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={panelRef}
        tabIndex={-1}
        className="max-h-[85vh] w-full max-w-2xl overflow-y-auto border border-line-strong bg-canvas p-6 shadow-xl outline-none"
      >
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              job <span className="text-ink-4">·</span> {jobId.slice(0, 8)}
            </p>
            <h2 id="job-detail-title" className="mt-1 font-mono text-sm text-ink">
              {label}
            </h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="border border-line-strong px-3 py-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2 hover:text-ink"
          >
            Close
          </button>
        </div>

        {detail === null ? (
          <p className="mt-6 text-sm text-ink-3">Loading...</p>
        ) : !detail.ok ? (
          <p className="mt-6 border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
            {detail.error}
          </p>
        ) : (
          <>
            <dl className="mt-5 grid grid-cols-2 gap-x-6 gap-y-2 font-mono text-xs sm:grid-cols-4">
              <div>
                <dt className="text-ink-4">status</dt>
                <dd className="text-ink">{detail.job.status}</dd>
              </div>
              <div>
                <dt className="text-ink-4">started</dt>
                <dd className="text-ink">{hhmm(detail.job.startedAt)}</dd>
              </div>
              <div>
                <dt className="text-ink-4">elapsed</dt>
                <dd className="text-ink">
                  {elapsed(detail.job.startedAt, detail.job.finishedAt)}
                  {detail.job.finishedAt ? "" : " so far"}
                </dd>
              </div>
              <div>
                <dt className="text-ink-4">reports</dt>
                <dd className="text-ink">{events.length}</dd>
              </div>
            </dl>

            {detail.truncated ? (
              <p className="mt-4 border-l-2 border-ink-3 bg-paper-2 px-4 py-2 text-xs text-ink-2">
                The runner keeps the most recent reports only, so this
                timeline starts mid-run rather than at the beginning.
              </p>
            ) : null}

            <p className="mt-6 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              timeline
            </p>
            {events.length === 0 ? (
              <p className="mt-2 text-sm text-ink-3">
                Nothing reported yet. Jobs that finish quickly, or that do
                not report progress, show only their result here.
              </p>
            ) : (
              <ol className="mt-2 space-y-px">
                {events.map((e, i) => (
                  <li
                    key={`${e.at ?? i}-${i}`}
                    className="bg-paper-2 px-4 py-2"
                  >
                    <div className="flex flex-wrap items-baseline gap-x-3 font-mono text-[11px] text-ink-4">
                      <span>{hhmm(e.at)}</span>
                      {i > 0 ? (
                        <span className="text-accent">
                          {gap(events[i - 1].at, e.at)}
                        </span>
                      ) : null}
                      {e.progress ? <span>{e.progress}%</span> : null}
                    </div>
                    <p className="mt-0.5 text-sm leading-relaxed text-ink-2">
                      {e.summary}
                    </p>
                  </li>
                ))}
              </ol>
            )}
          </>
        )}
      </div>
    </div>
  );
}
