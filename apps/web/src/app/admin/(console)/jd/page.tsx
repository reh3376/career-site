import type { Metadata } from "next";
import Link from "next/link";

import { callApi } from "@/lib/api-fetch";

import { getJdLimit } from "./actions";
import { FitBandsForm } from "./fit-bands-form";
import { SubmissionLimitForm } from "./submission-limit-form";
import { getJdBands } from "@/lib/jd-bands";
import { getSessionCookie } from "@/lib/session";

export const metadata: Metadata = { title: "Admin · JD submissions" };
export const dynamic = "force-dynamic";

type JdRow = {
  id: string;
  status: string;
  match_score?: number | null;
  matchScore?: number | null;
  text_head?: string;
  textHead?: string;
  role_hint?: string;
  roleHint?: string;
  employer_hint?: string;
  employerHint?: string;
  contact_email?: string;
  contactEmail?: string;
  apply_url?: string;
  applyUrl?: string;
  source?: string;
  error_message?: string;
  errorMessage?: string;
  generated_resume_url?: string;
  generatedResumeUrl?: string;
  created_at?: string;
  createdAt?: string;
  completed_at?: string;
  completedAt?: string;
};

type ListResp = {
  submissions?: JdRow[];
  ready_count?: number;
  readyCount?: number;
  below_threshold_count?: number;
  belowThresholdCount?: number;
  failed_count?: number;
  failedCount?: number;
  in_flight_count?: number;
  inFlightCount?: number;
};

type FetchResult =
  | { ok: true; data: ListResp }
  | { ok: false; error: string };

async function fetchList(): Promise<FetchResult> {
  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, error: "Not signed in" };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/ListJdSubmissions",
    body: {},
    cookie,
  });
  if (!resp.ok) return { ok: false, error: `HTTP ${resp.status}` };
  return { ok: true, data: (await resp.json()) as ListResp };
}

const STATUS_LABEL: Record<string, string> = {
  JD_STATUS_RECEIVED: "received",
  JD_STATUS_SCORING: "scoring",
  JD_STATUS_BELOW_THRESHOLD: "below threshold",
  JD_STATUS_GENERATING: "generating",
  JD_STATUS_READY: "ready",
  JD_STATUS_FAILED: "failed",
  JD_STATUS_NOT_A_POSTING: "not a posting",
};

const STATUS_TONE: Record<string, string> = {
  JD_STATUS_READY: "text-success",
  JD_STATUS_GENERATING: "text-signal",
  JD_STATUS_SCORING: "text-signal",
  JD_STATUS_RECEIVED: "text-ink-3",
  JD_STATUS_BELOW_THRESHOLD: "text-ink-3",
  JD_STATUS_FAILED: "text-danger",
  // Not an error: the reviewer declined to score something that was not
  // a posting, which is the system working.
  JD_STATUS_NOT_A_POSTING: "text-ink-2",
};

export default async function AdminJdPage() {
  const result = await fetchList();
  const nowMs = getNowMs();
  const bands = await getJdBands(await getSessionCookie());
  const limit = await getJdLimit();

  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        admin surface
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        JD submissions.
      </h1>
      <p className="mt-6 max-w-2xl text-sm leading-relaxed text-ink-3">
        Every JD submitted at /jd-upload. The match score is computed
        in code from per-requirement verdicts (open a row for the
        derivation). The gate is {bands.strong.toFixed(2)}; above it, the résumé pipeline
        runs. Below, Roger triages manually.
      </p>

      <section aria-labelledby="fit-bands-heading" className="mt-10">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          fit bands
        </p>
        <h2
          id="fit-bands-heading"
          className="font-display mt-3 text-2xl leading-tight text-ink"
          style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
        >
          What a score means.
        </h2>
        <p className="mt-3 max-w-2xl text-sm leading-relaxed text-ink-3">
          Very strong and strong: the submitter gets the tailored résumé attached.
          Possible and weak: they are told you will review and get back to them.
          Very weak: they are told no further action is needed.
        </p>
        <div className="mt-4">
          <FitBandsForm initial={bands} />
        </div>

        <h2
          className="font-display mt-10 text-2xl leading-tight text-ink"
          style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
        >
          How many a member may send.
        </h2>
        <p className="mt-3 max-w-2xl text-sm leading-relaxed text-ink-3">
          The reviewer works through one posting at a time and each one takes
          close to an hour, so this is the number that decides how long anyone
          can be made to wait. Members see what they have left before they
          spend it.
        </p>
        <div className="mt-4">
          {limit ? (
            <SubmissionLimitForm
              initial={limit.limit}
              windowHours={limit.windowHours}
            />
          ) : (
            <p className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
              Could not read the limit in force, so it is not shown rather
              than guessed at. The API refused the read; the cap itself is
              still enforced.
            </p>
          )}
        </div>
      </section>

      {!result.ok ? (
        <p className="mt-8 max-w-lg border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          Couldn&rsquo;t load submissions, {result.error}.
        </p>
      ) : (
        <>
          <dl className="mt-8 grid gap-3 border-y border-line py-4 font-mono text-sm text-ink-2 sm:grid-cols-4">
            <Chip
              label="in flight"
              n={result.data.in_flight_count ?? result.data.inFlightCount ?? 0}
              tone="text-signal"
            />
            <Chip
              label="ready"
              n={result.data.ready_count ?? result.data.readyCount ?? 0}
              tone="text-success"
            />
            <Chip
              label="below"
              n={
                result.data.below_threshold_count ??
                result.data.belowThresholdCount ??
                0
              }
              tone="text-ink-3"
            />
            <Chip
              label="failed"
              n={result.data.failed_count ?? result.data.failedCount ?? 0}
              tone="text-danger"
            />
          </dl>

          {(result.data.submissions ?? []).length === 0 ? (
            <p className="mt-10 text-sm text-ink-3">
              No submissions yet. When someone pastes a JD at
              /jd-upload, it&rsquo;ll land here with a score.
            </p>
          ) : (
            <ul className="mt-8 divide-y divide-line border-y border-line">
              {(result.data.submissions ?? []).map((r) => (
                <Row key={r.id} r={r} nowMs={nowMs} />
              ))}
            </ul>
          )}
        </>
      )}
    </>
  );
}

function Chip({
  label,
  n,
  tone,
}: {
  label: string;
  n: number;
  tone: string;
}) {
  return (
    <div className="flex items-baseline gap-3">
      <dt className="text-ink-3">{label}</dt>
      <dd className={"m-0 tabular text-2xl " + tone}>{n}</dd>
    </div>
  );
}

function Row({ r, nowMs }: { r: JdRow; nowMs: number }) {
  const statusCls = STATUS_TONE[r.status] ?? "text-ink-3";
  const label = STATUS_LABEL[r.status] ?? r.status;
  const score = r.match_score ?? r.matchScore ?? null;
  const scoreText =
    typeof score === "number" ? score.toFixed(3) : "—";
  const textHead = r.text_head ?? r.textHead ?? "";
  const role = r.role_hint ?? r.roleHint ?? "";
  const employer = r.employer_hint ?? r.employerHint ?? "";
  const email = r.contact_email ?? r.contactEmail ?? "";
  const applyUrl = r.apply_url ?? r.applyUrl ?? "";
  const errorMsg = r.error_message ?? r.errorMessage ?? "";
  const resume = r.generated_resume_url ?? r.generatedResumeUrl ?? "";
  const createdIso = r.created_at ?? r.createdAt;
  const created = createdIso ? new Date(createdIso) : null;

  return (
    <li className="py-5">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        <Link
          href={`/admin/jd/${r.id}`}
          className="text-accent no-underline hover:text-accent-strong"
        >
          #{r.id}
        </Link>
        <span className={statusCls}>· {label}</span>
        <span>
          score <span className="text-ink">{scoreText}</span>
        </span>
        {created ? <span>{relative(created, nowMs)}</span> : null}
      </div>
      {role || employer ? (
        <p className="mt-1 text-sm text-ink">
          {role || "(no role)"}
          {employer ? (
            <>
              {" "}
              <span className="text-ink-3">·</span> {employer}
            </>
          ) : null}
        </p>
      ) : null}
      {email ? (
        <p className="mt-1 font-mono text-xs">
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
          <span className="ml-2 font-mono text-[11px] text-ink-3">{applyUrl.replace(/^https?:\/\//, "").slice(0, 60)}</span>
        </p>
      ) : null}
      <p className="mt-3 whitespace-pre-wrap text-sm leading-relaxed text-ink-2">
        {textHead}
      </p>
      {errorMsg ? (
        <p className="mt-2 border-l-2 border-signal bg-signal-soft/40 px-3 py-2 font-mono text-[11px] text-ink">
          {errorMsg}
        </p>
      ) : null}
      {resume ? (
        <p className="mt-3 text-sm">
          <a
            href={resume}
            target="_blank"
            rel="noopener noreferrer"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            Generated résumé →
          </a>
        </p>
      ) : null}
    </li>
  );
}

function getNowMs(): number {
  return Date.now();
}

function relative(d: Date, nowMs: number): string {
  const ms = nowMs - d.getTime();
  const s = Math.max(0, Math.round(ms / 1000));
  if (s < 60) return "just now";
  const m = Math.round(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.round(m / 60);
  if (h < 48) return `${h}h ago`;
  const days = Math.round(h / 24);
  if (days < 30) return `${days}d ago`;
  if (days < 365) return `${Math.round(days / 30)}mo ago`;
  return `${Math.round(days / 365)}y ago`;
}
