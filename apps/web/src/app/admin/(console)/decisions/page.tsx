import type { Metadata } from "next";
import Link from "next/link";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import { AgreementPanel } from "./agreement";
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
  error?: string;
  humanVerdict?: string;
  human_note?: string;
  humanNote?: string;
  // The corrected wording, for a kind whose output is prose. This is
  // the training target; human_note is a remark about the grade.
  human_answer?: string;
  humanAnswer?: string;
  human_dimensions?: Record<string, string>;
  humanDimensions?: Record<string, string>;
  // Milliseconds to the first word, separate from latency_ms because on
  // a CPU-only box the two move for different reasons.
  first_token_ms?: number | string;
  firstTokenMs?: number | string;
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
  // Why the code weakened the verdict, when it did.
  adjusted?: string;
  // Ways the rationale disagrees with its own record. They never change
  // the verdict, so a reviewer can disagree with the check as readily as
  // with the model, which is the point of grading these at all.
  rationale_issues?: { kind?: string; detail?: string }[];
};

// A map rather than a ternary, because a ternary labels every kind it
// has not heard of as the one in its else branch. When the disjunction
// check was added, its issues rendered as "quote not in the
// requirement", which is a confident wrong answer of exactly the sort
// these checks exist to catch. An unknown kind now shows its own name.
const RATIONALE_ISSUE_LABEL: Record<string, string> = {
  span_mismatch: "span not reported",
  quote_unsupported: "quote not in the requirement",
  disjunction_ignored: "an offered alternative was refused",
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

// One of Ask Roger's answers. Shapes pinned in
// services/api/internal/users/chat_decision.go.
type ChatRetrieved = {
  chunk_id: number;
  title?: string;
  similarity: number;
  citable: boolean;
  shown: boolean;
  marker?: number;
};
type ChatInput = {
  question?: string;
  path?: string;
  persona_fingerprint?: string;
  history_turns?: number;
  corpus_scope?: string;
  retrieved?: ChatRetrieved[];
  qa?: {
    matched?: boolean;
    entry_id?: number;
    phrasing?: string;
    threshold?: number;
    best_similarity?: number;
  };
  timings?: {
    embed_ms?: number;
    qa_match_ms?: number;
    retrieve_ms?: number;
    first_token_ms?: number;
    total_ms?: number;
  };
};
type ChatCitation = {
  chunk_id?: number;
  title?: string;
  path?: string;
  rank?: number;
};
type ChatOutput = {
  text?: string;
  citations?: ChatCitation[];
  markers_offered?: number;
  markers_written?: number;
  markers_dropped?: number;
  out_of_scope?: boolean;
  no_support?: boolean;
  degraded?: boolean;
  qa_match?: boolean;
  finish_reason?: string;
  truncated?: boolean;
};

// How the answer was produced. Six different systems working or
// failing, which render almost identically to a reader, so the grader
// is told which one they are looking at before anything else.
const CHAT_PATH_LABEL: Record<string, string> = {
  qa_bank: "from your Q&A bank, no model",
  model: "retrieved and generated",
  degraded: "model unavailable, degraded reply",
  out_of_scope: "refused as out of scope",
  no_support: "nothing retrieved, said so",
  error: "failed before it could answer",
};

// The rubric, mirroring users.ReviewDimensions. A kind with no entry
// here is graded by its verdict alone.
const DIMENSIONS: Record<string, string[]> = {
  chat_answer: ["grounded", "citations", "voice", "scope", "length"],
};

// The owner's vocabulary. The first values mirror the model's, so
// agreement is a direct comparison. `insufficient_evidence` is the
// reviewer saying they could not judge this from what they were shown;
// it is on both kinds because either can be ungradeable, and it is
// excluded from the agreement rate rather than counted as a
// disagreement.
const VERDICT_OPTIONS: Record<string, string[]> = {
  jd_requirement_verdict: ["met", "partial", "unmet", "insufficient_evidence"],
  jd_gate: ["above_threshold", "below_threshold", "insufficient_evidence"],
  jd_posting_check: ["posting", "not_posting", "insufficient_evidence"],
  // "rejected_correctly" exists so a correct refusal is not graded as a
  // failure, and "needs_edit" is kept apart from "wrong" because it is
  // the outcome that produces the most useful correction.
  chat_answer: [
    "good",
    "needs_edit",
    "wrong",
    "rejected_correctly",
    "insufficient_evidence",
  ],
};

function parse<T>(s: string | undefined): T | null {
  if (!s) return null;
  try {
    return JSON.parse(s) as T;
  } catch {
    return null;
  }
}

type Search = {
  kind?: string;
  ref?: string;
  all?: string;
  failed?: string;
};

export default async function DecisionsPage({
  searchParams,
}: {
  searchParams: Promise<Search>;
}) {
  const sp = await searchParams;
  const kind = sp.kind ?? "";
  const ref = sp.ref ?? "";
  const unreviewedOnly = sp.all !== "1";
  // Failures are rare by construction, so they need a way to be found
  // that is not scrolling past two hundred successes.
  const failedOnly = sp.failed === "1";

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
      body: { kind, refId: ref, unreviewedOnly, failedOnly, limit: 200 },
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
    const merged = {
      kind,
      ref,
      all: unreviewedOnly ? "" : "1",
      failed: failedOnly ? "1" : "",
      ...next,
    };
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
        <Link
          href={filterHref({ all: unreviewedOnly ? "1" : "" })}
          className="text-accent"
        >
          {unreviewedOnly ? "show reviewed too" : "unreviewed only"}
        </Link>
        <span className="text-ink-3">·</span>
        <Link
          href={filterHref({ kind: kind === "jd_gate" ? "" : "jd_gate" })}
          className="text-accent"
        >
          {kind === "jd_gate" ? "all kinds" : "gate decisions only"}
        </Link>
        <span className="text-ink-3">·</span>
        <Link
          href={filterHref({ failed: failedOnly ? "" : "1" })}
          className={failedOnly ? "text-danger" : "text-accent"}
        >
          {failedOnly ? "all calls" : "failed calls only"}
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

      <AgreementPanel />

      {error ? (
        <p className="mt-10 text-sm text-danger">
          Could not load the log: {error}
        </p>
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
              <Link
                href={`/admin/jd/${sub}`}
                className="font-mono text-sm text-accent"
              >
                #{sub}
              </Link>
              <Link
                href={filterHref({ ref: sub })}
                className="text-xs text-ink-3"
              >
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

// One of Ask Roger's answers, laid out for grading rather than for
// browsing.
//
// The order is the order the owner needs it in: what was asked, what
// was said, then what it was said from. The correction box sits at the
// bottom seeded with the model's own wording, because the point of
// this page is not to count answers but to produce the corrected ones,
// and a correction that costs a retype does not get written.
function ChatAnswerCard({
  row,
  meta,
  options,
}: {
  row: Row;
  meta: React.ReactNode;
  options: string[];
}) {
  const input = parse<ChatInput>(row.input_json ?? row.inputJson);
  const output = parse<ChatOutput>(row.output_json ?? row.outputJson);
  const human = row.human_verdict ?? row.humanVerdict ?? "";
  const note = row.human_note ?? row.humanNote ?? "";
  const humanAnswer = row.human_answer ?? row.humanAnswer ?? "";
  const humanDims = row.human_dimensions ?? row.humanDimensions ?? {};
  const path = input?.path ?? "";
  const t = input?.timings ?? {};
  const firstToken = Number(
    row.first_token_ms ?? row.firstTokenMs ?? t.first_token_ms ?? 0,
  );
  const retrieved = input?.retrieved ?? [];
  const dropped = output?.markers_dropped ?? 0;

  return (
    <div>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        ask roger · {CHAT_PATH_LABEL[path] ?? path}
      </p>
      {meta}

      <p className="mt-3 text-base leading-snug text-ink">
        {input?.question ?? "(question missing)"}
      </p>

      <div className="mt-3 border-l-2 border-accent bg-paper-2 px-4 py-3 text-sm leading-relaxed whitespace-pre-wrap text-ink">
        {output?.text ?? row.response_text ?? row.responseText ?? ""}
      </div>

      {output?.truncated ? (
        <p className="mt-2 text-sm text-danger">
          Cut off at the token limit, so it stops mid sentence. That is the cap
          doing its job, not the model losing the thread; grade the part that
          is there.
        </p>
      ) : null}

      {(output?.citations ?? []).length === 0 ? null : (
        <ul className="mt-3 space-y-1 text-sm text-ink-2">
          {(output?.citations ?? []).map((c) => (
            <li key={`${c.rank}-${c.chunk_id}`}>
              <span className="font-mono text-ink-3">[{c.rank}]</span>{" "}
              {c.title || "(untitled)"}{" "}
              {c.path ? (
                <span className="font-mono text-[11px] text-ink-3">
                  {c.path}
                </span>
              ) : null}
            </li>
          ))}
        </ul>
      )}

      {dropped > 0 ? (
        <p className="mt-2 text-sm text-danger">
          {dropped} citation {dropped === 1 ? "marker" : "markers"} pointed at
          nothing and{" "}
          {dropped === 1 ? "was removed" : "were removed"} before this was
          shown. The answer claimed support it did not have.
        </p>
      ) : null}

      {/* What it was answering from. Grounding is a property of an
          answer given what it was shown, so a grader cannot mark it
          from the answer alone. */}
      <details className="mt-4">
        <summary className="cursor-pointer font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          what it was shown ({retrieved.filter((r) => r.shown).length} of{" "}
          {retrieved.length} retrieved)
        </summary>
        <ul className="mt-2 space-y-1 text-sm">
          {retrieved.map((r) => (
            <li
              key={r.chunk_id}
              className={r.shown ? "text-ink" : "text-ink-3"}
            >
              <span className="font-mono text-[11px]">
                {r.marker ? `[${r.marker}]` : "   "} {r.similarity.toFixed(3)}
              </span>{" "}
              {r.citable ? (r.title ?? "(untitled)") : "(private)"}{" "}
              {r.shown ? "" : "· not shown"}
            </li>
          ))}
          {retrieved.length === 0 ? (
            <li className="text-ink-3">Nothing was retrieved.</li>
          ) : null}
        </ul>
      </details>

      {/* The bank lookup every question goes through. The near misses
          are what calibrate the threshold, so a miss is worth reading. */}
      <p className="mt-3 font-mono text-[11px] text-ink-3">
        q&amp;a bank:{" "}
        {input?.qa?.matched
          ? `matched entry ${input.qa.entry_id} on "${input.qa.phrasing}" at ${(input.qa.best_similarity ?? 0).toFixed(3)}`
          : `no match, closest ${(input?.qa?.best_similarity ?? 0).toFixed(3)} against ${(input?.qa?.threshold ?? 0).toFixed(2)}`}
        {firstToken
          ? ` · ${(firstToken / 1000).toFixed(1)} s to first word`
          : ""}
        {input?.history_turns
          ? ` · ${input.history_turns} earlier ${input.history_turns === 1 ? "turn" : "turns"} in the prompt`
          : ""}
      </p>

      <details className="mt-2">
        <summary className="cursor-pointer font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          the exact prompt
        </summary>
        <pre className="mt-2 max-h-[28rem] overflow-auto bg-paper-2 p-3 font-mono text-[11px] leading-relaxed whitespace-pre-wrap text-ink-2">
          {row.prompt_text ?? row.promptText ?? "(no model was called)"}
        </pre>
      </details>

      <ReviewForm
        decisionId={row.id}
        options={options}
        current={human}
        note={note}
        dimensions={DIMENSIONS[row.kind] ?? []}
        currentDimensions={humanDims}
        modelAnswer={output?.text ?? ""}
        currentAnswer={humanAnswer}
      />
    </div>
  );
}

function DecisionCard({ row }: { row: Row }) {
  const human = row.human_verdict ?? row.humanVerdict ?? "";
  const note = row.human_note ?? row.humanNote ?? "";
  const reviewedAt = row.reviewed_at ?? row.reviewedAt;
  // No fallback. This used to default to ["agree", "disagree"], which
  // the server rejects for every kind, so a decision kind missing from
  // the map above got a form whose every answer failed. An empty list
  // makes the gap say so instead.
  const options = VERDICT_OPTIONS[row.kind] ?? [];
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

  // A call that produced no usable verdict. There is nothing to grade,
  // so it gets no verdict form; what it has is the prompt and the whole
  // response, which is the only reason the row exists. Before this,
  // decision_log was written after a successful decode only, so the one
  // response worth reading was the one thrown away.
  if (row.error) {
    return (
      <div>
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-danger">
          no verdict · {row.kind}
        </p>
        {meta}
        <p className="mt-3 border-l-2 border-danger bg-paper-2 px-4 py-3 text-sm leading-relaxed text-ink">
          {row.error}
        </p>
        <details className="mt-4">
          <summary className="cursor-pointer font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            what the model returned (
            {(row.response_text ?? row.responseText ?? "").length} characters)
          </summary>
          <pre className="mt-2 max-h-[28rem] overflow-auto bg-paper-2 p-3 font-mono text-[11px] leading-relaxed whitespace-pre-wrap text-ink-2">
            {row.response_text ?? row.responseText ?? "(nothing was returned)"}
          </pre>
        </details>
        <details className="mt-2">
          <summary className="cursor-pointer font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            the prompt it was answering
          </summary>
          <pre className="mt-2 max-h-[28rem] overflow-auto bg-paper-2 p-3 font-mono text-[11px] leading-relaxed whitespace-pre-wrap text-ink-2">
            {row.prompt_text ?? row.promptText ?? ""}
          </pre>
        </details>
      </div>
    );
  }

  if (row.kind === "chat_answer") {
    return <ChatAnswerCard row={row} meta={meta} options={options} />;
  }

  if (row.kind === "jd_gate") {
    const input = parse<GateInput>(row.input_json ?? row.inputJson);
    const output = parse<GateOutput>(row.output_json ?? row.outputJson);
    const v = input?.verdicts ?? {};
    return (
      <div>
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          gate
        </p>
        <p className="mt-2 text-sm text-ink">
          Match score{" "}
          <span className="font-mono">{(input?.score ?? 0).toFixed(3)}</span>{" "}
          against a gate of{" "}
          <span className="font-mono">
            {(input?.threshold ?? 0).toFixed(2)}
          </span>
          :{" "}
          <span className="font-mono">
            {(output?.outcome ?? "").replace(/_/g, " ")}
          </span>
          .
        </p>
        <p className="mt-1 text-sm text-ink-2">
          {input?.assessor_ran
            ? `${input.requirements ?? 0} requirements, weight ${input.weight_total ?? 0}: ${v.met ?? 0} met, ${v.partial ?? 0} partial, ${v.unmet ?? 0} unmet. Retrieval ${(input.retrieval_score ?? 0).toFixed(3)}.`
            : `Assessor did not run; the retrieval score ${(input?.retrieval_score ?? 0).toFixed(3)} was the gate.`}
        </p>
        {meta}
        <ReviewForm
          decisionId={row.id}
          options={options}
          current={human}
          note={note}
        />
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
        {req?.id ?? row.key} · {req?.category ?? ""} · weight{" "}
        {req?.weight ?? ""}
      </p>
      <p className="mt-2 text-base leading-snug text-ink">
        {req?.text ?? "(requirement missing)"}
      </p>

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
          <span className="text-ink-3">
            {" "}
            (model said {raw}; downgraded because it cited nothing it was given)
          </span>
        ) : null}
        {output?.rationale ? (
          <span className="text-ink-2">. {output.rationale}</span>
        ) : null}
      </p>
      {output?.adjusted ? (
        <p className="mt-2 text-sm text-ink-3">
          Code weakened this verdict: {output.adjusted}
        </p>
      ) : null}
      {(output?.rationale_issues ?? []).length > 0 ? (
        <ul className="mt-3 space-y-1">
          {(output?.rationale_issues ?? []).map((ri, i) => (
            <li
              key={i}
              className="border-l-2 border-signal bg-signal-soft/40 px-3 py-2 text-sm text-ink"
            >
              <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                {RATIONALE_ISSUE_LABEL[ri.kind ?? ""] ?? ri.kind ?? "issue"}
              </span>
              <span className="ml-2 text-ink-2">{ri.detail}</span>
            </li>
          ))}
        </ul>
      ) : null}
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

      <ReviewForm
        decisionId={row.id}
        options={options}
        current={human}
        note={note}
      />
    </div>
  );
}
