"use client";

import Link from "next/link";
import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import type { ApiClarification } from "@/lib/match-record";

import { requestClarificationAction, type AskState } from "../../questions/actions";

// The follow-up questions of a posting, from the hiring side's page
// (FSD FR-HF-13, FR-HF-22). It says where the questions asked on this
// record stand and links to where they are answered, and it offers the
// candidate's own request: a question for every requirement the record
// left unresolved that nobody has asked about. In Phase 0 the candidate
// and the owner are the same person, which is why the request sits on
// this page.

const label = "font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3";
const button =
  "inline-flex items-center border border-accent px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-50";

function when(iso: string | undefined): string {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : `${d.toISOString().slice(0, 16).replace("T", " ")} UTC`;
}

function plural(n: number, one: string, many: string): string {
  return n === 1 ? one : many;
}

export function FollowUpPanel({
  submissionId,
  clarification,
  askable,
  awaiting,
  superseded,
  busy,
}: {
  submissionId: string;
  clarification?: ApiClarification;
  askable: number;
  awaiting: boolean;
  superseded: boolean;
  busy: boolean;
}) {
  const [state, action] = useActionState<AskState, FormData>(requestClarificationAction, {});
  const c = clarification;
  const open = c?.status === "sent" && !c.closedByThisRun;
  const canAsk = askable > 0 && !superseded && !busy;
  if (!c && !canAsk && !state.requestId) return null;
  const asked = c?.asked ?? 0;
  const pending = c?.pending ?? 0;
  const href = state.requestId ?? c?.requestId;
  return (
    <div className="mt-5 border border-line-strong bg-paper-2 p-5">
      <p className={label}>
        follow-up questions <span className="text-ink-4">·</span> to the candidate
      </p>

      {c && open ? (
        <p className="mt-2 max-w-3xl text-sm leading-relaxed text-ink">
          {asked} {plural(asked, "question is", "questions are")} with the candidate, {pending} still open.{" "}
          {awaiting
            ? `This posting carries no fit until they are closed${c.expiresAt ? `; any still open at ${when(c.expiresAt)} closes unanswered and the fit is issued from what is evidenced` : ""}.`
            : "They were asked at the candidate's request, and the fit this run was given stands meanwhile."}
        </p>
      ) : null}
      {c && !open && !c.closedByThisRun ? (
        <p className="mt-2 max-w-3xl text-sm leading-relaxed text-ink-2">
          {c.status === "answered"
            ? `The candidate submitted the answers ${when(c.answeredAt)}. ${
                c.closingAttempt ? `Run ${c.closingAttempt} judged the answered requirements again.` : "The run that judges the answered requirements again is under way or waiting its turn."
              }`
            : c.closedReason === "superseded"
              ? "The questions asked on this run were closed when a later run replaced the record."
              : `The time for answering passed with ${pending} of ${asked} unanswered. ${
                  c.closingAttempt ? `Run ${c.closingAttempt} issued the fit from what is evidenced.` : "The run that issues the fit is under way or waiting its turn."
                }`}
        </p>
      ) : null}
      {c?.closedByThisRun ? (
        <p className="mt-2 max-w-3xl text-sm leading-relaxed text-ink-2">
          This run closed the {asked} {plural(asked, "question", "questions")} asked on run {c.askedAttempt ?? "?"}:{" "}
          {c.answered ?? 0} answered, {asked - (c.answered ?? 0)} not. The rows say which verdicts rest on an answer.
        </p>
      ) : null}

      {href ? (
        <p className="mt-3 text-sm">
          <Link href={`/admin/questions/${href}`} className="text-accent">
            {open || state.requestId ? "Answer the questions as the candidate" : "Read the questions and answers"}
          </Link>
        </p>
      ) : null}

      {canAsk ? (
        <form action={action} className="mt-4 flex flex-wrap items-center gap-4">
          <input type="hidden" name="submission_id" value={submissionId} />
          <Ask n={askable} />
          <span className="max-w-2xl text-xs leading-relaxed text-ink-3">
            Writes one question for each unresolved requirement that has none. Nothing is judged until the answers are
            submitted; then those requirements are judged again in a new run and the rest are carried forward.
          </span>
          {state.error ? <span className="text-sm text-danger">{state.error}</span> : null}
          {state.added ? (
            <span className="text-sm text-success">
              {state.added} {plural(state.added, "question", "questions")} written.
            </span>
          ) : null}
        </form>
      ) : null}
    </div>
  );
}

function Ask({ n }: { n: number }) {
  const { pending } = useFormStatus();
  return (
    <button type="submit" disabled={pending} className={button}>
      {pending ? "Writing..." : `Ask about the ${n} unresolved ${n === 1 ? "row" : "rows"}`}
    </button>
  );
}
