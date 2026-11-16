"use client";

import Link from "next/link";
import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import { submitAnswersAction, type SubmitState } from "../actions";
import { plural } from "../lib";

// Submitting is the one step that starts anything (FSD FR-HF-14,
// FR-HF-22): it closes the questions and begins the run that judges the
// answered requirements again and gives the posting its fit.
export function SubmitPanel({
  requestId,
  submissionId,
  open,
  pending,
  status,
  closed,
}: {
  requestId: string;
  submissionId: string;
  open: boolean;
  pending: number;
  status: string;
  // The run that judges the answers has finished with them.
  closed: boolean;
}) {
  const [state, action] = useActionState<SubmitState, FormData>(submitAnswersAction, {});
  const posting = (
    <Link href={`/admin/jd/${submissionId}#record`} className="text-accent">
      the posting&rsquo;s record
    </Link>
  );
  if (state.submitted || (!open && status === "answered")) {
    return (
      <div className="mt-10 border border-line-strong bg-paper-2 p-5">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">submitted</p>
        <p className="mt-2 max-w-2xl text-sm leading-relaxed text-ink-2">
          {state.submitted && !state.runStarted
            ? "The answers are saved. No reviewer is running here, so nothing was judged again."
            : closed
              ? "The answers were submitted. The requirements they speak to were judged again in a new run, every other verdict was carried forward, and the fit followed from that run."
              : "The answers are in. The requirements they speak to are judged again in a new run, every other verdict is carried forward, and the fit follows from that run."}{" "}
          {closed ? "The verdicts as they now stand are on" : "Follow it on"} {posting}.
        </p>
      </div>
    );
  }
  if (!open) {
    return (
      <p className="mt-10 max-w-2xl text-sm leading-relaxed text-ink-3">
        This questionnaire is closed and takes no further answers. The verdicts as they now stand are on {posting}.
      </p>
    );
  }
  return (
    <form action={action} className="mt-10 border border-line-strong bg-paper-2 p-5">
      <input type="hidden" name="request_id" value={requestId} />
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">submit the answers</p>
      <p className="mt-2 max-w-2xl text-sm leading-relaxed text-ink-2">
        {pending > 0
          ? `${pending} ${plural(pending, "question still needs", "questions still need")} an answer, "I have not done this" or "prefer not to answer". Each one is answered or explicitly left; none is skipped.`
          : "Every question has an answer. Submitting closes the questions and starts the run that judges the answered requirements again. It waits its turn for the model like any review, and you get the usual email when it finishes."}
      </p>
      <div className="mt-4 flex flex-wrap items-center gap-4">
        <Submit disabled={pending > 0} />
        {state.error ? <span className="text-sm text-danger">{state.error}</span> : null}
      </div>
    </form>
  );
}

function Submit({ disabled }: { disabled: boolean }) {
  const { pending } = useFormStatus();
  return (
    <button
      type="submit"
      disabled={pending || disabled}
      className="inline-flex items-center border border-accent px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
    >
      {pending ? "Submitting..." : "Submit the answers"}
    </button>
  );
}
