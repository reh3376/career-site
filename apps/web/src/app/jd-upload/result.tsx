"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

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
function VerdictBreakdown({ verdicts, met, partial, unmet }: {
  verdicts: Verdict[];
  met: number;
  partial: number;
  unmet: number;
}) {
  const unmetTexts = verdicts.filter((v) => v.verdict === "unmet").map((v) => v.text);
  return (
    <section className="border-t border-line pt-5">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        requirement by requirement
      </p>
      <p className="mt-2 text-sm leading-relaxed text-ink">
        {met} of {verdicts.length} requirements evidenced in Roger&rsquo;s records
        {partial ? `, ${partial} partly` : ""}
        {unmet ? `, ${unmet} not evidenced` : ""}.
        {unmetTexts.length ? (
          <span className="text-ink-2"> Not evidenced: {unmetTexts.join("; ")}.</span>
        ) : null}
      </p>
      <ul className="mt-4 space-y-3">
        {verdicts.map((v) => (
          <li key={v.id} className="grid gap-x-4 gap-y-1 text-sm sm:grid-cols-[9rem_1fr]">
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
              {v.rationale ? <span className="block text-xs text-ink-3">{v.rationale}</span> : null}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

const LIVE = new Set(["JD_STATUS_RECEIVED", "JD_STATUS_SCORING", "JD_STATUS_GENERATING"]);

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
}: {
  submissionId: string;
  resultToken: string;
  threshold: string;
}) {
  const [poll, setPoll] = useState<Poll | null>(null);
  const [failed, setFailed] = useState<string | null>(null);

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

  if (failed) {
    return (
      <p className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
        Could not check progress ({failed}). Keep your reference number
        and try again later.
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

  return (
    <div className="space-y-5">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        status{" "}
        <span className={live ? "text-signal" : status === "JD_STATUS_FAILED" ? "text-danger" : "text-ink"}>
          {STEP_LABEL[status] ?? status.toLowerCase()}
        </span>
        {typeof score === "number" ? (
          <>
            <span className="text-ink-4"> · </span>
            match score <span className="text-ink">{score.toFixed(2)}</span>
          </>
        ) : null}
      </p>

      {live ? (
        <p className="text-sm leading-relaxed text-ink-2">
          This runs a language model over each requirement in the
          posting, one at a time, and usually takes 15 to 30 minutes. You
          can close this page: the review stays at{" "}
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
          The weighted score came in under the {threshold} gate, so no
          tailored résumé was generated automatically. The breakdown
          below shows what was and was not evidenced; Roger sees every
          submission and will reply personally if the role looks worth
          a conversation.
        </p>
      ) : null}

      {!live && verdicts.length > 0 ? (
        <VerdictBreakdown verdicts={verdicts} met={met} partial={partial} unmet={unmet} />
      ) : null}

      {status === "JD_STATUS_FAILED" ? (
        <p className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          The pipeline hit an error{err ? `: ${err}` : ""}. Roger has
          the submission and will follow up.
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
