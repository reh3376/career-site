"use client";

import { useEffect, useRef, useState } from "react";

import { getJobAction, runJobAction, type JobState } from "../corpus/actions";
import { findRunningEvalAction } from "./actions";

// Starts an evaluation and polls it. The job runner allows one per
// kind, so the button is simply disabled while one is in flight; there
// is no queue to explain.
export function EvalRunner() {
  const [job, setJob] = useState<JobState | null>(null);
  const [error, setError] = useState<string>("");
  const [note, setNote] = useState<string>("");
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const poll = (jobId: string) => {
    timer.current = setTimeout(async () => {
      const r = await getJobAction(jobId);
      if (!r.ok) {
        setError(r.error);
        return;
      }
      setJob(r.job);
      if (
        r.job.status === "JOB_STATUS_RUNNING" ||
        r.job.status === "JOB_STATUS_QUEUED"
      ) {
        poll(jobId);
      }
    }, 5000);
  };

  // Reattach to an evaluation already in flight.
  //
  // The progress lived only in state set when the button was pressed,
  // so navigating away and back lost the job id and the page looked
  // idle while four hours of work carried on behind it. The job was
  // never affected; the page had forgotten it was watching. At four and
  // a half hours a run, leaving the page is the normal case.
  //
  // Declared after poll so it is not referenced before it exists.
  useEffect(() => {
    let stop = false;
    async function resume(): Promise<void> {
      const live = await findRunningEvalAction();
      if (stop || !live) return;
      setJob({
        jobId: live.jobId,
        kind: "JOB_KIND_EVAL_QUICK",
        status: "JOB_STATUS_RUNNING",
        progressPct: live.progressPct,
        summary: live.summary,
      });
      poll(live.jobId);
    }
    void resume();
    return () => {
      stop = true;
      if (timer.current) clearTimeout(timer.current);
    };
    // poll closes over nothing that changes, so listing it would
    // re-run this on every render and start a second poller.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const start = async () => {
    setError("");
    const r = await runJobAction("JOB_KIND_EVAL_QUICK", note);
    if (!r.ok) {
      setError(r.error);
      return;
    }
    setJob({
      jobId: r.jobId,
      kind: "JOB_KIND_EVAL_QUICK",
      status: "JOB_STATUS_RUNNING",
      progressPct: 0,
      summary: "starting",
    });
    poll(r.jobId);
  };

  const running =
    job?.status === "JOB_STATUS_RUNNING" || job?.status === "JOB_STATUS_QUEUED";

  return (
    <div className="border border-line-strong bg-paper-2 p-5">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        run an evaluation
      </p>
      <p className="mt-2 max-w-2xl text-sm leading-relaxed text-ink-2">
        Scores every active posting through the real pipeline, one at a time,
        and records what produced each result. On the production box that is
        closer to fifty minutes per posting: an eight-posting run took six
        hours and a quarter. Start it and come back tomorrow. Run one before a
        prompt or model change and one after: the pair is the evidence.
      </p>

      <div className="mt-4 flex flex-wrap items-end gap-3">
        <label className="block flex-1">
          <span className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-4">
            what are you testing (optional)
          </span>
          <input
            type="text"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="baseline before the judge prompt change"
            className="mt-1 block w-full border border-line bg-canvas px-3 py-2 text-sm text-ink"
          />
        </label>
        <button
          type="button"
          onClick={start}
          disabled={running}
          className="inline-flex items-center border border-accent px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
        >
          {running ? "Running..." : "Run evaluation"}
        </button>
      </div>

      {job ? (
        <div className="mt-4">
          <div className="h-2 w-full bg-line" aria-hidden="true">
            <div
              className="h-2 bg-accent transition-all duration-700"
              style={{
                width: `${Math.max(0, Math.min(100, job.progressPct))}%`,
              }}
            />
          </div>
          <p className="mt-2 flex items-baseline justify-between font-mono text-xs text-ink-2">
            <span>{job.summary || job.status}</span>
            <span className="text-ink">{job.progressPct}%</span>
          </p>
          {job.status === "JOB_STATUS_SUCCEEDED" ? (
            <p className="mt-2 text-sm text-signal">
              Finished. Reload to see it in the list below.
            </p>
          ) : null}
          {job.status === "JOB_STATUS_FAILED" ? (
            <p className="mt-2 text-sm text-danger">{job.summary}</p>
          ) : null}
        </div>
      ) : null}

      {error ? <p className="mt-3 text-sm text-danger">{error}</p> : null}
    </div>
  );
}
