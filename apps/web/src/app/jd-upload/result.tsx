"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { track } from "@/lib/events-client";

type Verdict = {
  id: string;
  text: string;
  category?: string;
  weight?: number;
  verdict: string;
  rationale?: string;
};

type Poll = {
  status?: string;
  matchScore?: number;
  match_score?: number;
  errorMessage?: string;
  error_message?: string;
  resumeMarkdown?: string;
  resume_markdown?: string;
  generatedResumeUrl?: string;
  generated_resume_url?: string;
  verdicts?: Verdict[];
  metCount?: number;
  met_count?: number;
  partialCount?: number;
  partial_count?: number;
  unmetCount?: number;
  unmet_count?: number;
  progressPct?: number;
  progress_pct?: number;
  progressStage?: string;
  progress_stage?: string;
  fitCategory?: string;
  fit_category?: string;
};

const FIT_LABEL: Record<string, string> = {
  very_strong: "very strong fit",
  strong: "strong fit",
  possible: "possible fit",
  weak: "weak fit",
  very_weak: "very weak fit",
};

const FIT_NOTE: Record<string, string> = {
  very_strong:
    "A two-page résumé written for this posting is ready below and in your email.",
  strong:
    "A two-page résumé written for this posting is ready below and in your email.",
  possible: "Roger will review this one himself and get back to you.",
  weak: "Roger will review this one himself and get back to you.",
  very_weak: "No further action is needed; the breakdown below shows why.",
};

const VERDICT_LABEL: Record<string, string> = {
  met: "evidenced",
  partial: "partly evidenced",
  unmet: "not evidenced",
};

// The requirement-by-requirement reading behind the score. Shown for
// every finished result, above or below the gate: what a hiring
// manager needs is which asks were and were not evidenced, not a
// single number that hides a strong candidate behind two literal
// misses.
function VerdictBreakdown({
  verdicts,
  met,
  partial,
  unmet,
}: {
  verdicts: Verdict[];
  met: number;
  partial: number;
  unmet: number;
}) {
  const unmetTexts = verdicts
    .filter((v) => v.verdict === "unmet")
    .map((v) => v.text);
  return (
    <section className="border-t border-line pt-5">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        requirement by requirement
      </p>
      <p className="mt-2 text-sm leading-relaxed text-ink">
        {met} of {verdicts.length} requirements evidenced in Roger&rsquo;s
        records
        {partial ? `, ${partial} partly` : ""}
        {unmet ? `, ${unmet} not evidenced` : ""}.
        {unmetTexts.length ? (
          <span className="text-ink-2">
            {" "}
            Not evidenced: {unmetTexts.join("; ")}.
          </span>
        ) : null}
      </p>
      <ul className="mt-4 space-y-3">
        {verdicts.map((v) => (
          <li
            key={v.id}
            className="grid gap-x-4 gap-y-1 text-sm sm:grid-cols-[9rem_1fr]"
          >
            <span
              className={
                v.verdict === "met"
                  ? "font-mono text-[11px] uppercase tracking-[0.14em] text-signal"
                  : v.verdict === "partial"
                    ? "font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2"
                    : "font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3"
              }
            >
              {VERDICT_LABEL[v.verdict] ?? v.verdict}
              {v.category === "nice" ? " · preferred" : ""}
            </span>
            <span className="text-ink-2">
              <span className="text-ink">{v.text}</span>
              {v.rationale ? (
                <span className="block text-xs text-ink-3">{v.rationale}</span>
              ) : null}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

const LIVE = new Set([
  "JD_STATUS_RECEIVED",
  "JD_STATUS_SCORING",
  "JD_STATUS_GENERATING",
]);

const STEP_LABEL: Record<string, string> = {
  JD_STATUS_RECEIVED: "queued",
  JD_STATUS_SCORING: "scoring the posting against the corpus",
  JD_STATUS_GENERATING: "above threshold, preparing the résumé",
  JD_STATUS_BELOW_THRESHOLD: "below threshold",
  JD_STATUS_READY: "ready",
  JD_STATUS_FAILED: "failed",
};

// Polls the public GetJdResult endpoint with the submission's result
// token until the pipeline reaches a terminal state, then renders the
// outcome. The token is what releases the résumé body; the id alone
// only ever yields status and score.
export function JdResult({
  submissionId,
  resultToken,
  threshold,
  watch = false,
}: {
  submissionId: string;
  resultToken: string;
  threshold: string;
  // Open the progress panel as a modal the submitter can keep open or
  // close (they get an email either way). Used right after submit.
  watch?: boolean;
}) {
  const [poll, setPoll] = useState<Poll | null>(null);
  const [failed, setFailed] = useState<string | null>(null);
  const [modalOpen, setModalOpen] = useState(watch);

  useEffect(() => {
    let stop = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const started = Date.now();

    async function tick(): Promise<void> {
      try {
        const resp = await fetch("/api/career.v1.JdService/GetJdResult", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ submissionId, resultToken }),
        });
        if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
        const j = (await resp.json()) as Poll;
        if (stop) return;
        setPoll(j);
        const live = LIVE.has(j.status ?? "");
        // The pipeline on the production box takes 15 to 30 minutes and
        // may queue behind another submission; keep polling for an hour,
        // quickly at first, then every 20 s.
        const elapsed = Date.now() - started;
        const withinBudget = elapsed < 60 * 60 * 1000;
        const interval = elapsed < 3 * 60 * 1000 ? 5000 : 20000;
        if (live && withinBudget) timer = setTimeout(tick, interval);
      } catch (e) {
        if (!stop) setFailed(e instanceof Error ? e.message : "poll failed");
      }
    }
    void tick();
    return () => {
      stop = true;
      if (timer) clearTimeout(timer);
    };
  }, [submissionId, resultToken]);

  // One jd.result_viewed per finished review per page load, whether it
  // was reached from the submit flow or reopened from the list.
  const viewedRef = useRef<string>("");
  const pollStatus = poll?.status ?? "";
  useEffect(() => {
    if (
      !pollStatus ||
      LIVE.has(pollStatus) ||
      viewedRef.current === submissionId
    )
      return;
    viewedRef.current = submissionId;
    track("jd.result_viewed", {
      submission_id: submissionId,
      via: resultToken ? "submit" : "reopen",
    });
  }, [pollStatus, submissionId, resultToken]);

  if (failed) {
    return (
      <p className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
        Could not check progress ({failed}). The review keeps running; reopen it
        from your submissions on /jd-upload, or wait for the email.
      </p>
    );
  }
  if (!poll) {
    return <p className="text-sm text-ink-3">Checking progress...</p>;
  }

  const status = poll.status ?? "";
  const score = poll.matchScore ?? poll.match_score;
  const resume = poll.resumeMarkdown ?? poll.resume_markdown ?? "";
  const pdfUrl = poll.generatedResumeUrl ?? poll.generated_resume_url ?? "";
  const err = poll.errorMessage ?? poll.error_message ?? "";
  const live = LIVE.has(status);
  const verdicts = poll.verdicts ?? [];
  const met = Number(poll.metCount ?? poll.met_count ?? 0);
  const partial = Number(poll.partialCount ?? poll.partial_count ?? 0);
  const unmet = Number(poll.unmetCount ?? poll.unmet_count ?? 0);
  const pct = Math.max(
    0,
    Math.min(100, Number(poll.progressPct ?? poll.progress_pct ?? 0)),
  );
  const stage = poll.progressStage ?? poll.progress_stage ?? "";
  const fit = poll.fitCategory ?? poll.fit_category ?? "";

  return (
    <div className="space-y-5">
      {modalOpen ? (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="jd-progress-title"
          className="fixed inset-0 z-50 flex items-end justify-center bg-ink/60 p-4 sm:items-center"
        >
          <div className="w-full max-w-lg border border-line-strong bg-canvas p-6 shadow-xl">
            <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
              jd review <span className="text-ink-4">·</span> #{submissionId}
            </p>
            {live ? (
              <>
                <h2 id="jd-progress-title" className="mt-2 text-lg text-ink">
                  Reviewing the posting
                </h2>
                <div className="mt-4 h-2 w-full bg-line" aria-hidden="true">
                  <div
                    className="h-2 bg-accent transition-all duration-700"
                    style={{ width: `${pct}%` }}
                  />
                </div>
                <p className="mt-2 flex items-baseline justify-between font-mono text-xs text-ink-2">
                  <span>{stage || STEP_LABEL[status] || "working"}</span>
                  <span className="text-ink">{pct}%</span>
                </p>
                <p className="mt-4 text-sm leading-relaxed text-ink-2">
                  Each requirement is judged one at a time; this usually takes
                  15 to 30 minutes. Keep this open to watch, or close it: you
                  get an email when the review is finished, with the résumé
                  attached if the fit is strong.
                </p>
                <div className="mt-5 flex flex-wrap gap-3">
                  <button
                    type="button"
                    onClick={() => setModalOpen(false)}
                    className="inline-flex items-center rounded-md bg-accent px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-accent-hover"
                  >
                    Close, email me when it is done
                  </button>
                  <span className="inline-flex items-center text-sm text-ink-3">
                    or keep watching
                  </span>
                </div>
              </>
            ) : (
              <>
                <h2 id="jd-progress-title" className="mt-2 text-lg text-ink">
                  {status === "JD_STATUS_FAILED"
                    ? "The review hit an error"
                    : fit
                      ? `Finished: ${FIT_LABEL[fit] ?? fit}`
                      : "Finished"}
                </h2>
                {typeof score === "number" ? (
                  <p className="mt-2 font-mono text-sm text-ink-2">
                    match score{" "}
                    <span className="text-ink">{score.toFixed(2)}</span>
                  </p>
                ) : null}
                {fit && FIT_NOTE[fit] ? (
                  <p className="mt-3 text-sm leading-relaxed text-ink-2">
                    {FIT_NOTE[fit]}
                  </p>
                ) : null}
                <div className="mt-5">
                  <button
                    type="button"
                    onClick={() => setModalOpen(false)}
                    className="inline-flex items-center rounded-md bg-accent px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-accent-hover"
                  >
                    View the review
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      ) : null}

      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        status{" "}
        <span
          className={
            live
              ? "text-signal"
              : status === "JD_STATUS_FAILED"
                ? "text-danger"
                : "text-ink"
          }
        >
          {STEP_LABEL[status] ?? status.toLowerCase()}
        </span>
        {typeof score === "number" ? (
          <>
            <span className="text-ink-4"> · </span>
            match score <span className="text-ink">{score.toFixed(2)}</span>
          </>
        ) : null}
        {fit && !live ? (
          <>
            <span className="text-ink-4"> · </span>
            <span
              className={
                fit === "very_strong" || fit === "strong"
                  ? "text-signal"
                  : "text-ink"
              }
            >
              {FIT_LABEL[fit] ?? fit}
            </span>
          </>
        ) : null}
      </p>

      {live ? (
        <div>
          <div className="h-1.5 w-full bg-line" aria-hidden="true">
            <div
              className="h-1.5 bg-accent transition-all duration-700"
              style={{ width: `${pct}%` }}
            />
          </div>
          <p className="mt-1 flex items-baseline justify-between font-mono text-[11px] text-ink-3">
            <span>{stage}</span>
            <span>{pct}%</span>
          </p>
        </div>
      ) : null}

      {fit && !live && FIT_NOTE[fit] ? (
        <p className="text-sm leading-relaxed text-ink-2">{FIT_NOTE[fit]}</p>
      ) : null}

      {live ? (
        <p className="text-sm leading-relaxed text-ink-2">
          This runs a language model over each requirement in the posting, one
          at a time, and usually takes 15 to 30 minutes. You can close this
          page: the review stays at{" "}
          <Link
            href={`/jd-upload/${submissionId}`}
            className="font-mono text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            /jd-upload/{submissionId}
          </Link>{" "}
          and you get an email when it finishes.
        </p>
      ) : null}

      {status === "JD_STATUS_BELOW_THRESHOLD" ? (
        <p className="text-sm leading-relaxed text-ink-2">
          The weighted score came in under the {threshold} gate, so no tailored
          résumé was generated automatically. The breakdown below shows what was
          and was not evidenced; Roger sees every submission and will reply
          personally if the role looks worth a conversation.
        </p>
      ) : null}

      {!live && verdicts.length > 0 ? (
        <VerdictBreakdown
          verdicts={verdicts}
          met={met}
          partial={partial}
          unmet={unmet}
        />
      ) : null}

      {status === "JD_STATUS_FAILED" ? (
        <p className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          The pipeline hit an error{err ? `: ${err}` : ""}. Roger has the
          submission and will follow up.
        </p>
      ) : null}

      {status === "JD_STATUS_READY" && pdfUrl ? (
        <p>
          <a
            href={pdfUrl}
            className="inline-flex items-center justify-center rounded-md bg-accent px-5 py-2.5 text-sm font-medium text-white no-underline shadow-sm transition-colors hover:bg-accent-hover"
          >
            Download the tailored résumé (PDF)
          </a>
          <span className="ml-3 text-xs text-ink-3">
            Opens without a password; editing is locked.
          </span>
        </p>
      ) : null}

      {status === "JD_STATUS_READY" && resume ? (
        <div className="prose-article border-t border-line pt-6 text-ink-2">
          <ReactMarkdown remarkPlugins={[remarkGfm]}>{resume}</ReactMarkdown>
        </div>
      ) : null}
    </div>
  );
}
