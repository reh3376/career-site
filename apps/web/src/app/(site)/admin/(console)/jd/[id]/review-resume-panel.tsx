"use client";

import { useRouter } from "next/navigation";
import { useActionState, useEffect } from "react";
import { useFormStatus } from "react-dom";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import type { ApiReviewResume } from "@/lib/match-record";

import { writeReviewResumeAction, type ReviewResumeState } from "./record-actions";

// The résumé after review (FSD FR-HF-08; owner's decision, 2026-11-13).
// A run that finished below the gate wrote no résumé. When your verdicts
// lift it to the gate you may have one written from the verdicts as
// they stand. It is yours alone: shown here, never emailed, and not on
// the recruiter's page, which reads a different place entirely.

const label = "font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3";
const button =
  "inline-flex items-center border border-accent px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-50";

function when(iso: string | undefined): string {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : `${d.toISOString().slice(0, 16).replace("T", " ")} UTC`;
}

export function ReviewResumePanel({
  runId,
  resume,
  canWrite,
  note,
}: {
  runId: string;
  resume?: ApiReviewResume;
  canWrite: boolean;
  note: string;
}) {
  const status = resume?.status ?? "";
  if (!resume && !canWrite && !note) return null;
  return (
    <div className="mt-5 border border-line-strong bg-paper-2 p-5">
      <p className={label}>
        résumé after review <span className="text-ink-4">·</span> for you only <span className="text-ink-4">·</span> never
        sent
      </p>

      {status === "writing" ? <Writing requestedAt={resume?.requestedAt} /> : null}

      {status === "ready" ? (
        <>
          <p className="mt-2 text-sm leading-relaxed text-ink-2">
            Written {when(resume?.finishedAt)} from the verdicts as they stood, with {resume?.overrideCount ?? 0} set by a
            person and a score after review of {(resume?.reviewedScore ?? 0).toFixed(2)}. The recruiter has not been sent
            it and cannot see it. Send it yourself if you choose to.
            {resume?.downloadPath ? (
              <>
                {" "}
                <a href={resume.downloadPath} className="text-accent">
                  Download the PDF
                </a>
                .
              </>
            ) : null}
          </p>
          <div className="prose-article mt-4 border border-line bg-canvas p-6 text-ink-2">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{resume?.markdown ?? ""}</ReactMarkdown>
          </div>
        </>
      ) : null}

      {status === "failed" ? (
        <p className="mt-2 border-l-2 border-signal pl-3 text-sm leading-relaxed text-ink">
          The last attempt did not produce a résumé{resume?.error ? `: ${resume.error}` : "."}
        </p>
      ) : null}

      {!resume && canWrite ? (
        <p className="mt-2 max-w-3xl text-sm leading-relaxed text-ink-2">
          Your verdicts lift this posting to the résumé gate. The run itself finished below it, so no résumé was written
          or sent. You can have one written from the verdicts as they now stand. It is shown here, to you, and is not
          sent to the recruiter.
        </p>
      ) : null}

      {canWrite ? (
        <WriteForm
          runId={runId}
          idle={status === "ready" ? "Write it again" : status === "failed" ? "Try again" : "Write the résumé for this posting"}
          hint={
            status === "ready"
              ? "Replaces the one above. Use it after changing a verdict."
              : "It waits its turn for the model like any review, then takes about 10 to 15 minutes on the production box."
          }
        />
      ) : note ? (
        <p className="mt-3 max-w-3xl text-sm leading-relaxed text-ink-3">{note}</p>
      ) : null}
    </div>
  );
}

// While a résumé is being written the page asks again every twenty
// seconds, so it appears without a reload.
function Writing({ requestedAt }: { requestedAt?: string }) {
  const router = useRouter();
  useEffect(() => {
    const t = setInterval(() => router.refresh(), 20000);
    return () => clearInterval(t);
  }, [router]);
  return (
    <p className="mt-2 max-w-3xl text-sm leading-relaxed text-ink">
      Being written since {when(requestedAt)}, from the verdicts as they stand when the model reaches it. On the
      production box this takes about 10 to 15 minutes, longer if a review is ahead of it. This page checks every 20
      seconds.
    </p>
  );
}

function WriteForm({ runId, idle, hint }: { runId: string; idle: string; hint: string }) {
  const [state, action] = useActionState<ReviewResumeState, FormData>(writeReviewResumeAction, {});
  return (
    <form action={action} className="mt-4 flex flex-wrap items-center gap-4">
      <input type="hidden" name="run_id" value={runId} />
      <Submit idle={idle} />
      <span className="max-w-2xl text-xs text-ink-3">{hint}</span>
      {state.error ? <span className="text-sm text-danger">{state.error}</span> : null}
    </form>
  );
}

function Submit({ idle }: { idle: string }) {
  const { pending } = useFormStatus();
  return (
    <button type="submit" disabled={pending} className={button}>
      {pending ? "Starting..." : idle}
    </button>
  );
}
