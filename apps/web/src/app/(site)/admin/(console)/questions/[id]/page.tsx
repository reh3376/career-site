import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import { plural, requestState, when, type ApiRequest } from "../lib";

import { QuestionCard } from "./question-card";
import { SubmitPanel } from "./submit-panel";

export const metadata: Metadata = { title: "Admin · Follow-up questions" };
export const dynamic = "force-dynamic";

// One questionnaire, as the candidate answers it (FSD FR-HF-13). Each
// question carries the posting's own words for the requirement, the
// question and the inputs it offers. Nothing here is read from the
// judgment: no threshold, no evidence standard, no verdict and no
// score, because a candidate is asked what they did, not shown what
// would score.
async function fetchRequest(id: string): Promise<ApiRequest | null> {
  const cookie = await getSessionCookie();
  if (!cookie) return null;
  const resp = await callApi({
    path: "/api/career.v1.HiringService/GetClarificationRequest",
    body: { clarificationRequestId: id },
    cookie,
  });
  if (!resp.ok) return null;
  return ((await resp.json()) as { request?: ApiRequest }).request ?? null;
}

export default async function QuestionnairePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  if (!/^[0-9a-fA-F-]{36}$/.test(id)) notFound();
  const q = await fetchRequest(id);
  if (!q) notFound();
  const items = q.items ?? [];
  const open = q.status === "sent";
  const pending = items.filter((it) => (it.answer?.status ?? "pending") === "pending").length;
  const state = requestState({ ...q, itemCount: items.length, pendingCount: pending });

  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        <Link href="/admin/questions" className="no-underline hover:text-accent-hover">
          Follow-up questions
        </Link>
        <span className="text-ink-4"> / </span>posting #{q.submissionId}
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        {q.roleTitle || "Untitled role"}
        {q.employer ? <span className="text-ink-3"> at {q.employer}</span> : null}
      </h1>

      <dl className="mt-8 grid gap-3 border-y border-line py-4 font-mono text-sm text-ink-2 sm:grid-cols-3">
        <div className="flex items-baseline gap-3">
          <dt className="text-ink-3">questions</dt>
          <dd className="m-0 text-ink">{items.length}</dd>
        </div>
        <div className="flex items-baseline gap-3">
          <dt className="text-ink-3">status</dt>
          <dd className={`m-0 ${state.tone}`}>{state.label}</dd>
        </div>
        <div className="flex items-baseline gap-3">
          <dt className="text-ink-3">asked</dt>
          <dd className="m-0">{when(q.generatedAt)}</dd>
        </div>
      </dl>

      <p className="mt-6 max-w-2xl text-sm leading-relaxed text-ink-2">
        The review of this posting could not settle {items.length} {plural(items.length, "requirement", "requirements")}{" "}
        from your documents. {state.sentence}
      </p>
      <p className="mt-3 max-w-2xl text-sm leading-relaxed text-ink-3">
        Say what you did, in your own terms. The choices are your own roles and projects and lists that are the same
        for every posting. An answer is kept as your statement, shown to the people reviewing this posting as yours,
        and never rewritten. You can change an answer until you submit, and withdraw one afterwards.
      </p>

      <ol className="mt-10 space-y-6">
        {items.map((it, i) => (
          <QuestionCard
            key={`${it.id}:${it.answer?.status ?? "pending"}:${it.answeredAt ?? ""}`}
            item={it}
            index={i + 1}
            total={items.length}
            open={open}
          />
        ))}
      </ol>

      <SubmitPanel
        requestId={q.id ?? ""}
        submissionId={q.submissionId ?? ""}
        open={open}
        pending={pending}
        status={q.status ?? ""}
        closed={Boolean(q.closedAt)}
      />
    </>
  );
}
