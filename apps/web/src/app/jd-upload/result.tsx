"use client";

import { useEffect, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

type Poll = {
  status?: string;
  matchScore?: number;
  match_score?: number;
  errorMessage?: string;
  error_message?: string;
  resumeMarkdown?: string;
  resume_markdown?: string;
};

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
}: {
  submissionId: string;
  resultToken: string;
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
        const withinBudget = Date.now() - started < 20 * 60 * 1000;
        if (live && withinBudget) timer = setTimeout(tick, 5000);
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
  const err = poll.errorMessage ?? poll.error_message ?? "";
  const live = LIVE.has(status);

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
          posting and can take a few minutes. You can leave this page;
          the reference number brings you back.
        </p>
      ) : null}

      {status === "JD_STATUS_BELOW_THRESHOLD" ? (
        <p className="text-sm leading-relaxed text-ink-2">
          The requirement-by-requirement check came in under the 0.65
          gate, so no tailored résumé was generated. Roger still sees
          every submission and will reply personally if the role looks
          worth a conversation.
        </p>
      ) : null}

      {status === "JD_STATUS_FAILED" ? (
        <p className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          The pipeline hit an error{err ? `: ${err}` : ""}. Roger has
          the submission and will follow up.
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
