import type { Metadata } from "next";
import Link from "next/link";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import { ReviewForm } from "./review-form";

export const metadata: Metadata = { title: "Admin · Decision review" };
export const dynamic = "force-dynamic";

type Row = {
  id: string;
  kind: string;
  ref_kind?: string;
  refKind?: string;
  ref_id?: string;
  refId?: string;
  key?: string;
  model?: string;
  prompt_id?: string;
  promptId?: string;
  prompt_version?: number;
  promptVersion?: number;
  num_ctx?: number;
  numCtx?: number;
  input_json?: string;
  inputJson?: string;
  output_json?: string;
  outputJson?: string;
  prompt_text?: string;
  promptText?: string;
  response_text?: string;
  responseText?: string;
  prompt_tokens?: number;
  promptTokens?: number;
  latency_ms?: number | string;
  latencyMs?: number | string;
  created_at?: string;
  createdAt?: string;
  human_verdict?: string;
  humanVerdict?: string;
  human_note?: string;
  humanNote?: string;
  reviewed_at?: string;
  reviewedAt?: string;
};

type ListResp = {
  decisions?: Row[];
  total_count?: number | string;
  totalCount?: number | string;
  reviewed_count?: number | string;
  reviewedCount?: number | string;
};

type Evidence = {
  chunk_id: number;
  source_kind: string;
  access: string;
  title?: string;
  similarity: number;
  text: string;
};
type VerdictInput = {
  requirement?: { id: string; text: string; category: string; weight: number };
  evidence?: Evidence[];
};
type VerdictOutput = {
  verdict?: string;
  evidence_ids?: number[];
  rationale?: string;
  raw_verdict?: string;
};
type GateInput = {
  score?: number;
  retrieval_score?: number;
  threshold?: number;
  assessor_ran?: boolean;
  requirements?: number;
  weight_total?: number;
  verdicts?: Record<string, number>;
};
type GateOutput = { outcome?: string };

const VERDICT_OPTIONS: Record<string, string[]> = {
  jd_requirement_verdict: ["met", "partial", "unmet"],
  jd_gate: ["above_threshold", "below_threshold"],
};

function parse<T>(s: string | undefined): T | null {
  if (!s) return null;
  try {
    return JSON.parse(s) as T;
  } catch {
    return null;
  }
}

type Search = { kind?: string; ref?: string; all?: string };

export default async function DecisionsPage({
  searchParams,
}: {
  searchParams: Promise<Search>;
}) {
  const sp = await searchParams;
  const kind = sp.kind ?? "";
  const ref = sp.ref ?? "";
  const unreviewedOnly = sp.all !== "1";

  const cookie = await getSessionCookie();
  let rows: Row[] = [];
  let total = 0;
  let reviewed = 0;
  let error = "";
  if (!cookie) {
    error = "Not signed in";
  } else {
    const resp = await callApi({
      path: "/api/career.v1.AdminService/ListDecisionLog",
      body: { kind, refId: ref, unreviewedOnly, limit: 200 },
      cookie,
    });
    if (!resp.ok) {
      error = `HTTP ${resp.status}`;
    } else {
      const data = (await resp.json()) as ListResp;
      rows = data.decisions ?? [];
      total = Number(data.total_count ?? data.totalCount ?? 0);
      reviewed = Number(data.reviewed_count ?? data.reviewedCount ?? 0);
    }
  }

  // Group by submission so the reviewer reads one JD's verdicts together.
  const groups = new Map<string, Row[]>();
  for (const r of rows) {
    const g = r.ref_id ?? r.refId ?? "?";
    if (!groups.has(g)) groups.set(g, []);
    groups.get(g)!.push(r);
  }

  const filterHref = (next: Partial<Search>) => {
    const q = new URLSearchParams();
    const merged = { kind, ref, all: unreviewedOnly ? "" : "1", ...next };
    if (merged.kind) q.set("kind", merged.kind);
    if (merged.ref) q.set("ref", merged.ref);
    if (merged.all) q.set("all", merged.all);
    const s = q.toString();
    return s ? `/admin/decisions?${s}` : "/admin/decisions";
  };

  return (
    <div>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        decision review
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-tight text-ink"
        style={{ fontVariationSettings: '"opsz" 96, "SOFT" 40' }}
      >
        What the reviewer decided, and whether you agree.
      </h1>
      <p className="mt-6 max-w-2xl text-sm leading-relaxed text-ink-3">
        Every verdict the JD reviewer reaches is logged with the requirement,
        the exact evidence the model saw, its verdict and its reasoning. Record
        your own verdict on each one. Reviewed rows are the training and
        evaluation set for the Ask Roger adapter; the model&rsquo;s output is
        never edited, so agreement can be measured.
      </p>

      <div className="mt-8 flex flex-wrap items-center gap-x-6 gap-y-2 text-sm text-ink-2">
        <span>
          <span className="font-mono text-ink">{reviewed}</span> reviewed of{" "}
          <span className="font-mono text-ink">{total}</span> logged
        </span>
        <span className="text-ink-3">·</span>
        <Link href={filterHref({ all: unreviewedOnly ? "1" : "" })} className="text-accent">
          {unreviewedOnly ? "show reviewed too" : "unreviewed only"}
        </Link>
        <span className="text-ink-3">·</span>
        <Link href={filterHref({ kind: kind === "jd_gate" ? "" : "jd_gate" })} className="text-accent">
          {kind === "jd_gate" ? "all kinds" : "gate decisions only"}
        </Link>
        {ref ? (
          <>
            <span className="text-ink-3">·</span>
            <Link href={filterHref({ ref: "" })} className="text-accent">
              clear submission filter
            </Link>
          </>
        ) : null}
        <span className="text-ink-3">·</span>
        <a href="/admin/decisions/export?reviewed=1" className="text-accent">
          export reviewed (JSONL)
        </a>
        <a href="/admin/decisions/export" className="text-accent">
          export all
        </a>
      </div>

      {error ? (
        <p className="mt-10 text-sm text-danger">Could not load the log: {error}</p>
      ) : null}

      {!error && rows.length === 0 ? (
        <p className="mt-10 text-sm text-ink-3">
          Nothing to review. New rows appear here each time a JD is scored.
        </p>
      ) : null}

      <div className="mt-10 space-y-12">
        {[...groups.entries()].map(([sub, items]) => (
          <section key={sub}>
            <h2 className="flex flex-wrap items-baseline gap-x-4 border-b border-line pb-2">
              <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                submission
              </span>
              <Link href={`/admin/jd/${sub}`} className="font-mono text-sm text-accent">
                #{sub}
              </Link>
              <Link href={filterHref({ ref: sub })} className="text-xs text-ink-3">
                only this submission
              </Link>
            </h2>
            <ul className="mt-4 space-y-8">
              {items.map((r) => (
                <li key={r.id} className="border border-line p-5">
                  <DecisionCard row={r} />
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </div>
  );
}

function DecisionCard({ row }: { row: Row }) {
  const human = row.human_verdict ?? row.humanVerdict ?? "";
  const note = row.human_note ?? row.humanNote ?? "";
  const reviewedAt = row.reviewed_at ?? row.reviewedAt;
  const options = VERDICT_OPTIONS[row.kind] ?? ["agree", "disagree"];
  const model = row.model ?? "";
  const promptId = row.prompt_id ?? row.promptId ?? "";
  const promptVersion = row.prompt_version ?? row.promptVersion ?? 0;
  const numCtx = row.num_ctx ?? row.numCtx ?? 0;
  const latency = Number(row.latency_ms ?? row.latencyMs ?? 0);
  const created = row.created_at ?? row.createdAt ?? "";

  const meta = (
    <p className="mt-1 font-mono text-[11px] text-ink-3">
      #{row.id} · {model}
      {promptId ? ` · ${promptId} v${promptVersion}` : ""}
      {numCtx ? ` · ctx ${numCtx}` : ""}
      {latency ? ` · ${(latency / 1000).toFixed(1)} s` : ""}
      {created ? ` · ${created.slice(0, 16).replace("T", " ")}` : ""}
      {reviewedAt ? " · reviewed" : ""}
    </p>
  );

  if (row.kind === "jd_gate") {
    const input = parse<GateInput>(row.input_json ?? row.inputJson);
    const output = parse<GateOutput>(row.output_json ?? row.outputJson);
    const v = input?.verdicts ?? {};
    return (
      <div>
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">gate</p>
        <p className="mt-2 text-sm text-ink">
          Match score{" "}
          <span className="font-mono">{(input?.score ?? 0).toFixed(3)}</span> against a gate of{" "}
          <span className="font-mono">{(input?.threshold ?? 0).toFixed(2)}</span>:{" "}
          <span className="font-mono">{(output?.outcome ?? "").replace(/_/g, " ")}</span>.
        </p>
        <p className="mt-1 text-sm text-ink-2">
          {input?.assessor_ran
            ? `${input.requirements ?? 0} requirements, weight ${input.weight_total ?? 0}: ${v.met ?? 0} met, ${v.partial ?? 0} partial, ${v.unmet ?? 0} unmet. Retrieval ${(input.retrieval_score ?? 0).toFixed(3)}.`
            : `Assessor did not run; the retrieval score ${(input?.retrieval_score ?? 0).toFixed(3)} was the gate.`}
        </p>
        {meta}
        <ReviewForm decisionId={row.id} options={options} current={human} note={note} />
      </div>
    );
  }

  const input = parse<VerdictInput>(row.input_json ?? row.inputJson);
  const output = parse<VerdictOutput>(row.output_json ?? row.outputJson);
  const req = input?.requirement;
  const cited = new Set(output?.evidence_ids ?? []);
  const verdict = output?.verdict ?? "";
  const raw = output?.raw_verdict ?? "";

  return (
    <div>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        {req?.id ?? row.key} · {req?.category ?? ""} · weight {req?.weight ?? ""}
      </p>
      <p className="mt-2 text-base leading-snug text-ink">{req?.text ?? "(requirement missing)"}</p>

      <p className="mt-4 text-sm text-ink-2">
        Model verdict:{" "}
        <span
          className={
            verdict === "met"
              ? "font-mono text-signal"
              : verdict === "partial"
                ? "font-mono text-ink"
                : "font-mono text-danger"
          }
        >
          {verdict || "none"}
        </span>
        {raw && raw !== verdict ? (
          <span className="text-ink-3"> (model said {raw}; downgraded because it cited nothing it was given)</span>
        ) : null}
        {output?.rationale ? <span className="text-ink-2">. {output.rationale}</span> : null}
      </p>
      {meta}

      <details className="mt-4">
        <summary className="cursor-pointer font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          evidence shown to the model ({input?.evidence?.length ?? 0} chunks
          {cited.size ? `, ${cited.size} cited` : ""})
        </summary>
        <ul className="mt-3 space-y-3">
          {(input?.evidence ?? []).map((e) => (
            <li
              key={e.chunk_id}
              className={`border-l-2 pl-3 text-sm leading-relaxed ${cited.has(e.chunk_id) ? "border-accent" : "border-line"}`}
            >
              <p className="font-mono text-[11px] text-ink-3">
                chunk {e.chunk_id} · {e.source_kind} · {e.access}
                {e.title ? ` · ${e.title}` : ""} · sim {e.similarity.toFixed(2)}
                {cited.has(e.chunk_id) ? " · cited" : ""}
              </p>
              <p className="mt-1 whitespace-pre-wrap text-ink-2">{e.text}</p>
            </li>
          ))}
        </ul>
      </details>

      <details className="mt-2">
        <summary className="cursor-pointer font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          raw prompt and response
        </summary>
        <pre className="mt-3 max-h-96 overflow-auto whitespace-pre-wrap border border-line p-3 text-xs text-ink-2">
          {row.prompt_text ?? row.promptText ?? ""}
          {"\n\n=== response ===\n"}
          {row.response_text ?? row.responseText ?? ""}
        </pre>
      </details>

      <ReviewForm decisionId={row.id} options={options} current={human} note={note} />
    </div>
  );
}
