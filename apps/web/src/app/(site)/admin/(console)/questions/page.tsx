import type { Metadata } from "next";
import Link from "next/link";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import { plural, requestState, when, type ApiRequest } from "./lib";

export const metadata: Metadata = { title: "Admin · Follow-up questions" };
export const dynamic = "force-dynamic";

// The questions reviews have put to the candidate (FSD FR-HF-13, 22).
// In Phase 0 the candidate is the owner, so the candidate's side of the
// loop lives in the console. What a questionnaire shows is what a
// candidate is shown: the posting's own words and the question.
async function fetchRequests(): Promise<ApiRequest[] | null> {
  const cookie = await getSessionCookie();
  if (!cookie) return null;
  const resp = await callApi({
    path: "/api/career.v1.HiringService/ListClarificationRequests",
    body: {},
    cookie,
  });
  if (!resp.ok) return null;
  return ((await resp.json()) as { requests?: ApiRequest[] }).requests ?? [];
}

export default async function QuestionsPage() {
  const requests = await fetchRequests();
  const open = (requests ?? []).filter((q) => q.status === "sent");
  const closed = (requests ?? []).filter((q) => q.status !== "sent");

  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">admin surface</p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Follow-up questions.
      </h1>
      <p className="mt-6 max-w-2xl text-sm leading-relaxed text-ink-3">
        When a review cannot settle a requirement from your documents, it asks you. Each question shows the
        posting&rsquo;s own words and takes an answer, &ldquo;I have not done this&rdquo; or &ldquo;prefer not to
        answer&rdquo;. Your answer becomes evidence in your own words, the requirement is judged again with it, and
        the record says which verdicts rest on it. You answer here as the candidate; the hiring side&rsquo;s view of
        the same posting is under JD submissions.
      </p>

      {requests === null ? (
        <p className="mt-8 max-w-lg border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          Could not load the questions.
        </p>
      ) : (
        <>
          <section aria-labelledby="open-heading" className="mt-10">
            <p id="open-heading" className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              waiting on you <span className="text-ink-4">·</span> {open.length}
            </p>
            {open.length === 0 ? (
              <p className="mt-3 text-sm text-ink-3">Nothing is waiting on an answer.</p>
            ) : (
              <RequestList requests={open} />
            )}
          </section>

          {closed.length > 0 ? (
            <section aria-labelledby="closed-heading" className="mt-12">
              <p id="closed-heading" className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                closed <span className="text-ink-4">·</span> {closed.length}
              </p>
              <RequestList requests={closed} />
            </section>
          ) : null}
        </>
      )}
    </>
  );
}

function RequestList({ requests }: { requests: ApiRequest[] }) {
  return (
    <ul className="mt-4 divide-y divide-line border-y border-line">
      {requests.map((q) => {
        const state = requestState(q);
        const asked = q.itemCount ?? 0;
        return (
          <li key={q.id} className="py-4">
            <Link href={`/admin/questions/${q.id}`} className="grid gap-x-6 gap-y-1 no-underline sm:grid-cols-[1fr_14rem]">
              <span className="text-sm text-ink">
                {q.roleTitle || "Untitled role"}
                {q.employer ? <span className="text-ink-3"> at {q.employer}</span> : null}
                <span className="mt-1 block font-mono text-[11px] text-ink-3">
                  posting #{q.submissionId} · run {q.attempt ?? "?"} · {asked} {plural(asked, "question", "questions")} ·
                  asked {when(q.generatedAt)}
                  {q.askedBy === "candidate" ? " at your request" : ""}
                </span>
              </span>
              <span className={`font-mono text-[11px] uppercase tracking-[0.14em] ${state.tone}`}>
                {state.label}
                {q.status === "sent" && q.expiresAt ? (
                  <span className="mt-1 block normal-case tracking-normal text-ink-3">closes {when(q.expiresAt)}</span>
                ) : null}
              </span>
            </Link>
          </li>
        );
      })}
    </ul>
  );
}
