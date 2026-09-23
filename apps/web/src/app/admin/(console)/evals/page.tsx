import type { Metadata } from "next";
import Link from "next/link";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import { setGoldenActiveAction } from "./actions";
import { EvalRunner } from "./eval-runner";
import { GoldenForm } from "./golden-form";

export const metadata: Metadata = { title: "Admin · Evaluations" };
export const dynamic = "force-dynamic";

type Posting = {
  id?: string | number;
  name?: string;
  expected_gate?: string;
  expectedGate?: string;
  expected_band?: string;
  expectedBand?: string;
  note?: string;
  active?: boolean;
  last_score?: number;
  lastScore?: number;
  last_passed?: boolean;
  lastPassed?: boolean;
};

type Run = {
  id?: string | number;
  status?: string;
  note?: string;
  model?: string;
  app_commit?: string;
  appCommit?: string;
  corpus_fingerprint?: string;
  corpusFingerprint?: string;
  prompts_json?: string;
  promptsJson?: string;
  threshold?: number;
  total?: number;
  scored?: number;
  gate_correct?: number;
  gateCorrect?: number;
  order_violations?: number;
  orderViolations?: number;
  margin?: number;
  errors?: number;
  started_at?: string;
  startedAt?: string;
};

async function fetchJson<T>(path: string, body: unknown): Promise<T | null> {
  const cookie = await getSessionCookie();
  if (!cookie) return null;
  const resp = await callApi({ path, body, cookie });
  if (!resp.ok) return null;
  return (await resp.json()) as T;
}

function num(v: number | undefined): string {
  return v === undefined || v === null ? "-" : Number(v).toFixed(3);
}

export default async function AdminEvalsPage() {
  const [set, runs] = await Promise.all([
    fetchJson<{ postings?: Posting[] }>(
      "/api/career.v1.AdminService/ListGoldenPostings",
      { activeOnly: false },
    ),
    fetchJson<{ runs?: Run[] }>("/api/career.v1.AdminService/ListEvalRuns", {
      limit: 20,
    }),
  ]);
  const postings = set?.postings ?? [];
  const evals = runs?.runs ?? [];
  const above = postings.filter(
    (p) => (p.expected_gate ?? p.expectedGate) === "above",
  ).length;

  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        evaluations
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        The golden set.
      </h1>
      <p className="mt-5 max-w-2xl text-base leading-relaxed text-ink-2">
        A fixed group of postings, each with one claim attached: this is a real
        fit, or it is not. Scoring them all under one configuration says whether
        a change to a prompt, a model or the corpus made the reviewer better or
        worse. Without it, a change is just a change.
      </p>

      <div className="mt-8 space-y-6">
        <EvalRunner />
        <GoldenForm />
      </div>

      <section className="mt-10">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          the set <span className="text-ink-4">·</span> {postings.length}{" "}
          postings, {above} expected above the gate
        </p>
        {postings.length === 0 ? (
          <p className="mt-3 max-w-2xl text-sm leading-relaxed text-ink-3">
            Nothing in the set yet. Add the postings you already use to judge
            the reviewer by hand: a couple that are obviously a fit, a couple
            that are obviously not, and any that it has got wrong before. Four
            or five is enough to catch a regression.
          </p>
        ) : (
          <ul className="mt-4 divide-y divide-line border-y border-line">
            {postings.map((p) => {
              const gate = p.expected_gate ?? p.expectedGate ?? "";
              const last = p.last_score ?? p.lastScore;
              const passed = p.last_passed ?? p.lastPassed;
              const active = p.active !== false;
              return (
                <li
                  key={String(p.id)}
                  className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 py-3"
                >
                  <div className="min-w-0">
                    <p className="text-sm text-ink">
                      {p.name}
                      <span className="ml-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                        expect {gate}
                      </span>
                      {!active ? (
                        <span className="ml-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-4">
                          retired
                        </span>
                      ) : null}
                    </p>
                    {p.note ? (
                      <p className="mt-0.5 text-sm text-ink-3">{p.note}</p>
                    ) : null}
                  </div>
                  <div className="flex items-center gap-4">
                    <span className="font-mono text-sm text-ink">
                      {last === undefined ? (
                        <span className="text-ink-4">not scored</span>
                      ) : (
                        <>
                          {num(last)}{" "}
                          <span
                            className={passed ? "text-signal" : "text-danger"}
                          >
                            {passed ? "ok" : "miss"}
                          </span>
                        </>
                      )}
                    </span>
                    <form action={setGoldenActiveAction} className="m-0">
                      <input type="hidden" name="id" value={String(p.id)} />
                      <input
                        type="hidden"
                        name="active"
                        value={active ? "false" : "true"}
                      />
                      <button
                        type="submit"
                        className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3 underline decoration-line underline-offset-4 hover:text-accent"
                      >
                        {active ? "retire" : "restore"}
                      </button>
                    </form>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </section>

      <section className="mt-10">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          evaluations <span className="text-ink-4">·</span> {evals.length}
        </p>
        {evals.length === 0 ? (
          <p className="mt-3 text-sm text-ink-3">
            None yet. Run one to set the baseline everything after it is
            compared against.
          </p>
        ) : (
          <ul className="mt-4 space-y-3">
            {evals.map((r) => {
              const started = r.started_at ?? r.startedAt;
              const correct = r.gate_correct ?? r.gateCorrect ?? 0;
              const violations = r.order_violations ?? r.orderViolations ?? 0;
              return (
                <li
                  key={String(r.id)}
                  className="border border-line bg-paper-2 px-4 py-3"
                >
                  <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
                    <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2">
                      <Link
                        href={`/admin/evals/${r.id}`}
                        className="text-accent no-underline hover:text-accent-strong"
                      >
                        #{String(r.id)}
                      </Link>
                      <span className="text-ink-4"> · </span>
                      {r.status}
                      {r.note ? (
                        <span className="text-ink-3"> · {r.note}</span>
                      ) : null}
                    </p>
                    <p className="font-mono text-[11px] text-ink-3">
                      {started ? new Date(started).toLocaleString() : ""}
                    </p>
                  </div>
                  <p className="mt-2 text-sm text-ink">
                    {correct} of {r.scored ?? 0} on the expected side
                    <span className="text-ink-3">
                      {" "}
                      · {violations} inversions · margin {num(r.margin)} · gate{" "}
                      {num(r.threshold)}
                    </span>
                    {r.errors ? (
                      <span className="text-danger"> · {r.errors} errors</span>
                    ) : null}
                  </p>
                  <p className="mt-1 font-mono text-[11px] text-ink-3">
                    {r.model || "-"}
                    {" · corpus "}
                    {r.corpus_fingerprint ?? r.corpusFingerprint ?? "-"}
                    {" · build "}
                    {r.app_commit ?? r.appCommit ?? "-"}
                  </p>
                </li>
              );
            })}
          </ul>
        )}
      </section>
    </>
  );
}
