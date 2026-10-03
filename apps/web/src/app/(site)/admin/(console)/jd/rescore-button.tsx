"use client";

import { useFormStatus } from "react-dom";

import { rescoreJdAction } from "./actions";

export function RescoreButton({ submissionId }: { submissionId: string }) {
  return (
    <form action={rescoreJdAction} className="m-0">
      <input type="hidden" name="submission_id" value={submissionId} />
      <Submit />
    </form>
  );
}

function Submit() {
  const { pending } = useFormStatus();
  return (
    <button
      type="submit"
      disabled={pending}
      className="inline-flex items-center border border-accent px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
    >
      {pending ? "Queuing..." : "Re-score"}
    </button>
  );
}
