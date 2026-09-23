import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export const metadata: Metadata = { title: "Admin · Evaluation" };
export const dynamic = "force-dynamic";

type Item = {
  golden_id?: string | number;
  goldenId?: string | number;
  golden_name?: string;
  goldenName?: string;
  expected_gate?: string;
  expectedGate?: string;
  submission_id?: string | number;
  submissionId?: string | number;
  match_score?: number;
  matchScore?: number;
  fit?: string;
  gate_side?: string;
  gateSide?: string;
  passed?: boolean;
  error?: string;
};

type Run = {
  id?: string | number;
  status?: string;
  note?: string;
  model?: string;
  num_ctx?: number;
  numCtx?: number;
  host?: string;
  embedder_model?: string;
  embedderModel?: string;
  app_commit?: string;
  appCommit?: string;
  prompts_json?: string;
  promptsJson?: string;
  corpus_fingerprint?: string;
  corpusFingerprint?: string;
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
  items?: Item[];
};

function num(v: number | undefined): string {
  return v === undefined || v === null ? "-" : Number(v).toFixed(3);
}

export default async function EvalDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const cookie = await getSessionCookie();
  if (!cookie) notFound();
  const resp = await callApi({
    path: "/api/career.v1.AdminService/GetEvalRun",
    body: { id },
    cookie,
  });
  if (!resp.ok) notFound();
  const data = (await resp.json()) as { run?: Run };
  const run = data.run;
  if (!run) notFound();

  const items = run.items ?? [];
  const correct = run.gate_correct ?? run.gateCorrect ?? 0;
  const violations = run.order_violations ?? run.orderViolations ?? 0;
  let prompts: [string, string][] = [];
  try {
    prompts = Object.entries(
      JSON.parse(run.prompts_json ?? run.promptsJson ?? "{}") as Record<
        string,
        string
      >,
    );
  } catch {
    prompts = [];
  }

  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        <Link
          href="/admin/evals"
          className="no-underline hover:text-accent-strong"
        >
          evaluations
        </Link>
        <span className="text-ink-4"> / </span>#{String(run.id)}
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        {run.note || "Evaluation"}
      </h1>

      <dl className="mt-8 grid gap-3 border-y border-line py-4 font-mono text-sm text-ink-2 sm:grid-cols-4">
        <Chip
          label="on the expected side"
          value={`${correct} of ${run.scored ?? 0}`}
          tone="text-ink"
        />
        <Chip
          label="inversions"
          value={String(violations)}
          tone={violations ? "text-danger" : "text-ink"}
        />
        <Chip label="margin" value={num(run.margin)} tone="text-ink" />
        <Chip label="gate" value={num(run.threshold)} />
      </dl>

      <p className="mt-4 max-w-2xl text-sm leading-relaxed text-ink-2">
        An inversion is a posting that should be below the gate outscoring one
        that should be above. It is worse than a gate miss, because moving the
        gate cannot fix it. The margin is the smallest gap between the two
        groups: when it shrinks, the next change is the one that breaks
        something.
      </p>

      <p className="mt-4 font-mono text-[11px] leading-relaxed text-ink-3">
        {run.model || "-"}
        {(run.num_ctx ?? run.numCtx)
          ? ` · ${run.num_ctx ?? run.numCtx} ctx`
          : ""}
        {run.host ? ` · ${run.host}` : ""}
        {" · corpus "}
        {run.corpus_fingerprint ?? run.corpusFingerprint ?? "-"}
        {" · embed "}
        {run.embedder_model ?? run.embedderModel ?? "-"}
        {" · build "}
        {run.app_commit ?? run.appCommit ?? "-"}
        {prompts.length
          ? ` · ${prompts.map(([k, v]) => `${k} ${v}`).join("  ·  ")}`
          : ""}
      </p>

      <section className="mt-10">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          per posting
        </p>
        <ul className="mt-4 divide-y divide-line border-y border-line">
          {items.map((it) => {
            const expected = it.expected_gate ?? it.expectedGate ?? "";
            const actual = it.gate_side ?? it.gateSide ?? "";
            const sub = it.submission_id ?? it.submissionId;
            return (
              <li
                key={String(it.golden_id ?? it.goldenId)}
                className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 py-3"
              >
                <div className="min-w-0">
                  <p className="text-sm text-ink">
                    {it.golden_name ?? it.goldenName}
                    <span className="ml-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                      expect {expected}
                      {actual && actual !== expected ? ` · got ${actual}` : ""}
                    </span>
                  </p>
                  {it.error ? (
                    <p className="mt-0.5 text-sm text-danger">{it.error}</p>
                  ) : null}
                </div>
                <div className="flex items-center gap-4 font-mono text-sm">
                  <span className="text-ink">
                    {num(it.match_score ?? it.matchScore)}
                    {it.fit ? (
                      <span className="text-ink-3"> · {it.fit}</span>
                    ) : null}
                  </span>
                  <span className={it.passed ? "text-signal" : "text-danger"}>
                    {it.passed ? "ok" : "miss"}
                  </span>
                  {sub ? (
                    <Link
                      href={`/admin/jd/${sub}`}
                      className="text-[11px] uppercase tracking-[0.14em] text-accent no-underline hover:text-accent-strong"
                    >
                      derivation
                    </Link>
                  ) : null}
                </div>
              </li>
            );
          })}
        </ul>
      </section>
    </>
  );
}

function Chip({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone?: string;
}) {
  return (
    <div>
      <dt className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-4">
        {label}
      </dt>
      <dd className={`mt-0.5 ${tone ?? "text-ink-2"}`}>{value}</dd>
    </div>
  );
}
