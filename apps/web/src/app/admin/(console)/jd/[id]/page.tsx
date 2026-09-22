import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import { RescoreButton } from "../rescore-button";

import { RunHistory, type Run } from "./run-history";

export const metadata: Metadata = { title: "Admin · JD submission" };
export const dynamic = "force-dynamic";

type Row = {
  id: string;
  status: string;
  match_score?: number;
  matchScore?: number;
  retrieval_score?: number;
  retrievalScore?: number;
  role_hint?: string;
  roleHint?: string;
  employer_hint?: string;
  employerHint?: string;
  contact_email?: string;
  contactEmail?: string;
  apply_url?: string;
  applyUrl?: string;
  error_message?: string;
  errorMessage?: string;
  created_at?: string;
  createdAt?: string;
};

type Detail = {
  row?: Row;
  jd_text?: string;
  jdText?: string;
  assessment_json?: string;
  assessmentJson?: string;
  resume_markdown?: string;
  resumeMarkdown?: string;
  llm_model?: string;
  llmModel?: string;
  prompt_id?: string;
  promptId?: string;
  prompt_version?: number;
  promptVersion?: number;
  download_url?: string;
  downloadUrl?: string;
  runs?: Run[];
};

type Requirement = {
  id: string;
  text: string;
  category: string;
  weight: number;
};
type Judgment = {
  requirement_id: string;
  verdict: string;
  evidence_ids?: number[];
  rationale?: string;
};
type Assessment = {
  requirements?: Requirement[];
  judgments?: Judgment[];
  evidence_ids?: Record<string, number[]>;
  score?: number;
  weight_total?: number;
  model?: string;
  prompts?: Record<string, number>;
  error?: string;
};

const STATUS_LABEL: Record<string, string> = {
  JD_STATUS_RECEIVED: "received",
  JD_STATUS_SCORING: "scoring",
  JD_STATUS_BELOW_THRESHOLD: "below threshold",
  JD_STATUS_GENERATING: "generating",
  JD_STATUS_READY: "ready",
  JD_STATUS_FAILED: "failed",
};

const VERDICT_TONE: Record<string, string> = {
  met: "text-success",
  partial: "text-signal",
  unmet: "text-ink-3",
};

async function fetchDetail(id: string): Promise<Detail | null> {
  const cookie = await getSessionCookie();
  if (!cookie) return null;
  const resp = await callApi({
    path: "/api/career.v1.AdminService/GetJdSubmission",
    body: { submissionId: id },
    cookie,
  });
  if (!resp.ok) return null;
  return (await resp.json()) as Detail;
}

export default async function AdminJdDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const d = await fetchDetail(id);
  if (!d?.row) notFound();
  const r = d.row;

  const score = r.match_score ?? r.matchScore;
  const retrieval = r.retrieval_score ?? r.retrievalScore;
  const runs: Run[] = d.runs ?? [];
  const jdText = d.jd_text ?? d.jdText ?? "";
  const resume = d.resume_markdown ?? d.resumeMarkdown ?? "";
  const model = d.llm_model ?? d.llmModel ?? "";
  const errorMsg = r.error_message ?? r.errorMessage ?? "";
  const role = r.role_hint ?? r.roleHint ?? "";
  const employer = r.employer_hint ?? r.employerHint ?? "";
  const email = r.contact_email ?? r.contactEmail ?? "";
  const applyUrl = r.apply_url ?? r.applyUrl ?? "";

  let assessment: Assessment | null = null;
  const raw = d.assessment_json ?? d.assessmentJson ?? "";
  if (raw) {
    try {
      assessment = JSON.parse(raw) as Assessment;
    } catch {
      assessment = null;
    }
  }
  const judgmentsByReq = new Map<string, Judgment>();
  for (const j of assessment?.judgments ?? [])
    judgmentsByReq.set(j.requirement_id, j);

  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        <Link
          href="/admin/jd"
          className="no-underline hover:text-accent-strong"
        >
          JD submissions
        </Link>
        <span className="text-ink-4"> / </span>#{r.id}
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        {role || "Untitled role"}
        {employer ? <span className="text-ink-3"> at {employer}</span> : null}
      </h1>

      <dl className="mt-8 grid gap-3 border-y border-line py-4 font-mono text-sm text-ink-2 sm:grid-cols-4">
        <Chip label="status" value={STATUS_LABEL[r.status] ?? r.status} />
        <Chip
          label="match"
          value={typeof score === "number" ? score.toFixed(3) : "-"}
          tone="text-ink"
        />
        <Chip
          label="retrieval"
          value={typeof retrieval === "number" ? retrieval.toFixed(3) : "-"}
        />
        <Chip label="model" value={assessment?.model || model || "-"} />
      </dl>

      {email ? (
        <p className="mt-3 font-mono text-xs">
          <a
            href={`mailto:${email}?subject=Re%3A%20your%20JD%20%23${r.id}`}
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            {email}
          </a>
        </p>
      ) : null}
      {applyUrl ? (
        <p className="mt-2 text-sm">
          <a
            href={applyUrl}
            target="_blank"
            rel="noopener noreferrer nofollow"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            Apply for this position
          </a>
          <span className="ml-2 font-mono text-[11px] text-ink-3">
            {applyUrl.replace(/^https?:\/\//, "").slice(0, 60)}
          </span>
        </p>
      ) : null}

      {errorMsg ? (
        <p className="mt-4 border-l-2 border-signal bg-signal-soft/40 px-3 py-2 font-mono text-[11px] text-ink">
          {errorMsg}
        </p>
      ) : null}

      <div className="mt-6 flex flex-wrap items-center gap-4">
        <RescoreButton submissionId={r.id} />
        <p className="text-xs text-ink-3">
          Re-runs retrieval, assessment, and (above threshold) the résumé and
          PDF with the current prompts. Use after a transient failure or a
          prompt change; the previous derivation is replaced.
        </p>
      </div>

      <section className="mt-10">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          assessment
          {assessment?.prompts ? (
            <span className="text-ink-4">
              {" "}
              ·{" "}
              {Object.entries(assessment.prompts)
                .map(([k, v]) => `${k} v${v}`)
                .join(", ")}
            </span>
          ) : null}
        </p>
        {!assessment ? (
          <p className="mt-3 text-sm text-ink-3">
            No assessment stored. Either scoring has not run or the assessor was
            not wired (retrieval score is the gate).
          </p>
        ) : assessment.error && !assessment.requirements?.length ? (
          <p className="mt-3 border-l-2 border-signal bg-signal-soft/40 px-3 py-2 text-sm text-ink">
            Assessment failed, gate fell back to the retrieval score:{" "}
            <span className="font-mono text-[11px]">{assessment.error}</span>
          </p>
        ) : (
          <ul className="mt-4 divide-y divide-line border-y border-line">
            {(assessment.requirements ?? []).map((req) => {
              const j = judgmentsByReq.get(req.id);
              const verdict = j?.verdict ?? "unmet";
              return (
                <li key={req.id} className="py-3">
                  <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                    <span className="text-ink">{req.id}</span>
                    <span>{req.category}</span>
                    <span>w{req.weight}</span>
                    <span className={VERDICT_TONE[verdict] ?? "text-ink-3"}>
                      {verdict}
                    </span>
                    {j?.evidence_ids?.length ? (
                      <span>chunks {j.evidence_ids.join(", ")}</span>
                    ) : null}
                  </div>
                  <p className="mt-1 text-sm text-ink">{req.text}</p>
                  {j?.rationale ? (
                    <p className="mt-1 text-xs text-ink-3">{j.rationale}</p>
                  ) : null}
                </li>
              );
            })}
          </ul>
        )}
        {assessment && typeof assessment.score === "number" ? (
          <p className="mt-3 font-mono text-xs text-ink-3">
            weighted score {assessment.score.toFixed(3)} over weight total{" "}
            {assessment.weight_total ?? "-"} (met 1.0, partial 0.5, unmet 0)
          </p>
        ) : null}
      </section>

      {resume ? (
        <section className="mt-10">
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            generated résumé
            {(d.download_url ?? d.downloadUrl) ? (
              <>
                <span className="text-ink-4"> · </span>
                <a
                  href={d.download_url ?? d.downloadUrl}
                  className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
                >
                  download PDF
                </a>
              </>
            ) : null}
          </p>
          <div className="prose-article mt-4 border border-line-strong bg-canvas p-6 text-ink-2">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{resume}</ReactMarkdown>
          </div>
        </section>
      ) : null}

      <RunHistory runs={runs} />

      <section className="mt-10">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          job description as submitted
        </p>
        <pre className="mt-4 whitespace-pre-wrap border border-line bg-paper-2 p-4 font-mono text-[12px] leading-snug text-ink-2">
          {jdText}
        </pre>
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
    <div className="flex items-baseline gap-3">
      <dt className="text-ink-3">{label}</dt>
      <dd className={"m-0 " + (tone ?? "text-ink-2")}>{value}</dd>
    </div>
  );
}
