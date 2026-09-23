import type { Metadata } from "next";
import Link from "next/link";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export const metadata: Metadata = { title: "Admin · Analytics" };
export const dynamic = "force-dynamic";

// Data layer D5. Every number here is selected from a SQL view
// (migration 00028) and none is computed in this page, so the page
// cannot drift from the log, an export, or a query run by hand.
//
// The sections follow the criteria the owner set for "runs very well".
// A criterion with no data says so plainly rather than showing a zero
// that reads like a failure.

type Eval = {
  id?: string | number;
  note?: string;
  gate_correct?: number;
  gateCorrect?: number;
  scored?: number;
  order_violations?: number;
  orderViolations?: number;
  margin?: number;
};

type Outcome = { fit?: string; outcome?: string; submissions?: number };

type Metrics = {
  recent_runs?: number;
  recentRuns?: number;
  recent_completed?: number;
  recentCompleted?: number;
  recent_failed?: number;
  recentFailed?: number;
  recent_stuck?: number;
  recentStuck?: number;
  reviewed?: number;
  agreed?: number;
  agreement_pct?: number;
  agreementPct?: number;
  soft_disagreements?: number;
  softDisagreements?: number;
  hard_disagreements?: number;
  hardDisagreements?: number;
  finished_runs?: number;
  finishedRuns?: number;
  median_minutes?: number;
  medianMinutes?: number;
  p95_minutes?: number;
  p95Minutes?: number;
  median_queued_minutes?: number;
  medianQueuedMinutes?: number;
  landed?: number;
  read_writing?: number;
  readWriting?: number;
  clicked?: number;
  registered?: number;
  verified?: number;
  signed_in?: number;
  signedIn?: number;
  submitted?: number;
  calls?: number;
  call_failures?: number;
  callFailures?: number;
  prompt_tokens?: string | number;
  promptTokens?: string | number;
  completion_tokens?: string | number;
  completionTokens?: string | number;
  latest_eval?: Eval;
  latestEval?: Eval;
  outcomes?: Outcome[];
};

function n(a: number | undefined, b: number | undefined): number {
  return a ?? b ?? 0;
}
function opt(a: number | undefined, b: number | undefined): number | undefined {
  return a ?? b;
}
function fmt(v: number | undefined, unit = "", digits = 1): string {
  if (v === undefined || v === null) return "-";
  return `${Number(v).toFixed(digits)}${unit}`;
}
function big(v: string | number | undefined): string {
  const x = Number(v ?? 0);
  if (x >= 1_000_000) return `${(x / 1_000_000).toFixed(1)}M`;
  if (x >= 1_000) return `${(x / 1_000).toFixed(1)}k`;
  return String(x);
}

export default async function AdminAnalyticsPage() {
  const cookie = await getSessionCookie();
  let m: Metrics = {};
  if (cookie) {
    const resp = await callApi({
      path: "/api/career.v1.AdminService/GetMetrics",
      body: {},
      cookie,
    });
    if (resp.ok) m = (await resp.json()) as Metrics;
  }

  const runs = n(m.recent_runs, m.recentRuns);
  const completed = n(m.recent_completed, m.recentCompleted);
  const failed = n(m.recent_failed, m.recentFailed);
  const stuck = n(m.recent_stuck, m.recentStuck);
  const reviewed = m.reviewed ?? 0;
  const agreement = opt(m.agreement_pct, m.agreementPct);
  const median = opt(m.median_minutes, m.medianMinutes);
  const p95 = opt(m.p95_minutes, m.p95Minutes);
  const evalRun = m.latest_eval ?? m.latestEval;
  const outcomes = m.outcomes ?? [];

  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        analytics
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Does it run well?
      </h1>
      <p className="mt-5 max-w-2xl text-base leading-relaxed text-ink-2">
        Every number here is read from a SQL view, not computed on this page, so
        it cannot drift from what a query would tell you. Where there is no data
        yet, it says so instead of showing a zero.
      </p>

      <Section
        title="Reliability"
        target="Twenty consecutive runs with no intervention."
        empty={runs === 0 ? "No runs recorded yet." : ""}
      >
        <Stat label="last runs" value={String(runs)} />
        <Stat
          label="finished cleanly"
          value={`${completed} of ${runs}`}
          tone={runs > 0 && completed === runs ? "text-signal" : "text-ink"}
        />
        <Stat
          label="failed"
          value={String(failed)}
          tone={failed ? "text-danger" : "text-ink-3"}
        />
        <Stat
          label="stuck"
          value={String(stuck)}
          tone={stuck ? "text-danger" : "text-ink-3"}
        />
      </Section>

      <Section
        title="Agreement"
        target="At least 85 percent of reviewed verdicts, with disagreements landing on partial rather than flipping met and unmet."
        empty={
          reviewed === 0
            ? "Nothing reviewed yet. Label some verdicts in decision review and this fills in."
            : ""
        }
      >
        <Stat
          label="agreement"
          value={fmt(agreement, "%")}
          tone={
            agreement !== undefined && agreement >= 85
              ? "text-signal"
              : "text-ink"
          }
        />
        <Stat label="reviewed" value={String(reviewed)} />
        <Stat
          label="unsure (partial)"
          value={String(n(m.soft_disagreements, m.softDisagreements))}
          tone="text-ink-3"
        />
        <Stat
          label="wrong (met vs unmet)"
          value={String(n(m.hard_disagreements, m.hardDisagreements))}
          tone={
            n(m.hard_disagreements, m.hardDisagreements)
              ? "text-danger"
              : "text-ink-3"
          }
        />
      </Section>

      <Section
        title="Time to a result"
        target="Median under 20 minutes, 95th percentile under 45. Queue time counted separately."
        empty={
          n(m.finished_runs, m.finishedRuns) === 0
            ? "No finished runs yet."
            : ""
        }
      >
        <Stat
          label="median"
          value={fmt(median, " min")}
          tone={
            median !== undefined && median <= 20 ? "text-signal" : "text-ink"
          }
        />
        <Stat
          label="95th percentile"
          value={fmt(p95, " min")}
          tone={p95 !== undefined && p95 <= 45 ? "text-signal" : "text-ink"}
        />
        <Stat
          label="median queued"
          value={fmt(
            opt(m.median_queued_minutes, m.medianQueuedMinutes),
            " min",
          )}
        />
        <Stat
          label="runs measured"
          value={String(n(m.finished_runs, m.finishedRuns))}
        />
      </Section>

      <Section
        title="Calibration"
        target="Every strong posting above the gate and every weak one below, holding across a change."
        empty={
          !evalRun
            ? "No evaluation yet. Add postings on the evaluations page and run one to set the baseline."
            : ""
        }
      >
        {evalRun ? (
          <>
            <Stat
              label="on the expected side"
              value={`${evalRun.gate_correct ?? evalRun.gateCorrect ?? 0} of ${evalRun.scored ?? 0}`}
              tone="text-ink"
            />
            <Stat
              label="inversions"
              value={String(
                evalRun.order_violations ?? evalRun.orderViolations ?? 0,
              )}
              tone={
                (evalRun.order_violations ?? evalRun.orderViolations ?? 0) > 0
                  ? "text-danger"
                  : "text-signal"
              }
            />
            <Stat label="margin" value={fmt(evalRun.margin, "", 3)} />
            <Stat
              label="evaluation"
              value={
                <Link
                  href={`/admin/evals/${evalRun.id}`}
                  className="text-accent no-underline hover:text-accent-strong"
                >
                  #{String(evalRun.id)}
                </Link>
              }
            />
          </>
        ) : null}
      </Section>

      <Section
        title="Does the score predict anything?"
        target="The question the whole data layer exists to answer. It needs recorded outcomes."
        empty={
          outcomes.length === 0
            ? "No outcomes recorded yet. Record what happened on a submission and the bands start earning their keep."
            : ""
        }
      >
        {outcomes.length ? (
          <div className="col-span-full">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-line text-left font-mono text-[10px] uppercase tracking-[0.14em] text-ink-4">
                  <th className="py-2">fit</th>
                  <th className="py-2">outcome</th>
                  <th className="py-2 text-right">submissions</th>
                </tr>
              </thead>
              <tbody>
                {outcomes.map((o, i) => (
                  <tr
                    key={`${o.fit}-${o.outcome}-${i}`}
                    className="border-b border-line"
                  >
                    <td className="py-2 text-ink">{o.fit || "-"}</td>
                    <td className="py-2 text-ink-2">{o.outcome}</td>
                    <td className="py-2 text-right font-mono text-ink">
                      {o.submissions}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </Section>

      <Section
        title="Reach, last 30 days"
        target="Anonymous visitors through to a submitted posting."
        empty={
          n(m.landed, undefined) === 0
            ? "No visitors recorded in the window."
            : ""
        }
      >
        <Stat label="landed" value={String(m.landed ?? 0)} />
        <Stat
          label="read an article"
          value={String(n(m.read_writing, m.readWriting))}
        />
        <Stat label="requested access" value={String(m.registered ?? 0)} />
        <Stat label="submitted a posting" value={String(m.submitted ?? 0)} />
      </Section>

      <Section
        title="Model load, last 30 days"
        target="What the box actually did."
      >
        <Stat label="calls" value={String(m.calls ?? 0)} />
        <Stat
          label="failed"
          value={String(n(m.call_failures, m.callFailures))}
          tone={
            n(m.call_failures, m.callFailures) ? "text-danger" : "text-ink-3"
          }
        />
        <Stat
          label="prompt tokens"
          value={big(m.prompt_tokens ?? m.promptTokens)}
        />
        <Stat
          label="completion tokens"
          value={big(m.completion_tokens ?? m.completionTokens)}
        />
      </Section>
    </>
  );
}

function Section({
  title,
  target,
  empty,
  children,
}: {
  title: string;
  target: string;
  empty?: string;
  children?: React.ReactNode;
}) {
  return (
    <section className="mt-10">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        {title}
      </p>
      <p className="mt-1 max-w-2xl text-sm leading-relaxed text-ink-3">
        {target}
      </p>
      {empty ? (
        <p className="mt-3 text-sm text-ink-2">{empty}</p>
      ) : (
        <dl className="mt-4 grid gap-4 border-t border-line pt-4 sm:grid-cols-4">
          {children}
        </dl>
      )}
    </section>
  );
}

function Stat({
  label,
  value,
  tone,
}: {
  label: string;
  value: React.ReactNode;
  tone?: string;
}) {
  return (
    <div>
      <dt className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-4">
        {label}
      </dt>
      <dd className={`mt-1 font-mono text-lg ${tone ?? "text-ink"}`}>
        {value}
      </dd>
    </div>
  );
}
