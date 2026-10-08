import Link from "next/link";
import { notFound } from "next/navigation";
import type { Metadata } from "next";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export const metadata: Metadata = { title: "Admin · Decision test analysis" };

// The three views that answer the research question.
//
// They have existed since migration 00054 and were reachable only
// through /admin/db, which put the finding this instrument exists to
// produce one hand-written query away from anybody wanting to see it.
//
// Nothing is computed on this page. Every number comes from a view
// (FR-DT-15, docs/metrics.md), because the whole point of defining each
// one once is that a second definition in a React component is how two
// numbers that should agree stop agreeing.

type LoadPoint = {
  itemSetVersion?: string;
  instrumentVersion?: string;
  load?: string;
  loadRank?: number;
  sessions?: number;
  sessionsUnreviewed?: number;
  sessionsFirstAttempt?: number;
  sessionsRepeat?: number;
  answers?: number;
  accuracyPct?: number;
  lurePct?: number;
  expiryPct?: number;
  meanConfidence?: number;
  gapPct?: number;
  confidentlyWrongPct?: number;
  meanBrier?: number;
  families?: string;
};
type CalPoint = {
  instrumentVersion?: string;
  load?: string;
  loadRank?: number;
  confidenceBand?: number;
  answers?: number;
  accuracyPct?: number;
  overclaimPct?: number;
};
type Threshold = {
  sessionKey?: string;
  attemptNo?: number;
  reviewStatus?: string;
  accuracyPct?: number;
  gapPct?: number;
  fatigueDeltaPct?: number;
  luredAtRank?: number;
  luredAtLoad?: string;
  luredInControl?: boolean;
  confidentlyWrong?: number;
};

const LOAD_LABEL: Record<string, string> = {
  d3: "3 digits",
  d4: "4 digits",
  d4_plus1: "4 digits, +1",
  d4_plus3: "4 digits, +3",
  d3_control: "3 digits, control",
};

export default async function AnalysisPage() {
  const cookie = await getSessionCookie();
  if (!cookie) notFound();
  const resp = await callApi({
    path: "/api/career.v1.AdminService/GetDecisionTestAnalysis",
    body: {},
    cookie,
  });
  if (!resp.ok) notFound();
  const res = (await resp.json()) as {
    loadCurve?: LoadPoint[];
    calibration?: CalPoint[];
    thresholds?: Threshold[];
  };
  const curve = res.loadCurve ?? [];
  const cal = res.calibration ?? [];
  const thresholds = res.thresholds ?? [];

  // Grouped by instrument, because the view groups by it and pooling
  // two instruments into one table would undo that in the rendering.
  const instruments = [...new Set(curve.map((p) => p.instrumentVersion ?? ""))];

  return (
    <>
      <Link
        href="/admin/decision-test"
        className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase no-underline hover:text-accent"
      >
        back to runs
      </Link>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        What the runs say.
      </h1>
      <p className="mt-6 max-w-xl text-base leading-relaxed text-ink-2">
        Accuracy against confidence as load rises, the reliability curve
        behind it, and where each participant first went confidently
        wrong. Agent-driven runs are excluded. Every figure here is
        defined once, as a SQL view, and read rather than recomputed.
      </p>

      {curve.length === 0 ? (
        <p className="mt-10 rounded-md border border-line bg-paper-2 px-5 py-4 text-base text-ink-2">
          Nothing to show yet. These views need completed runs on the
          current instrument, and a curve drawn from one or two people
          says nothing about anybody.
        </p>
      ) : null}

      {instruments.map((iv) => {
        const points = curve
          .filter((p) => p.instrumentVersion === iv)
          .sort((a, b) => (a.loadRank ?? 0) - (b.loadRank ?? 0));
        const sessions = Math.max(...points.map((p) => p.sessions ?? 0), 0);
        const thin = sessions < 10;
        // A point drawing on one family measures the family as much as
        // the load. That is exactly what made the first two real runs
        // unusable, so it is a warning rather than a footnote.
        const familyBlocked = points.some(
          (p) => (p.families ?? "").length > 0 && !(p.families ?? "").includes("+"),
        );
        return (
          <section key={iv} className="mt-14">
            <h2 className="font-display text-2xl text-ink">Load curve</h2>
            <p className="mt-2 font-mono text-[11px] tracking-[0.1em] text-ink-3">
              {iv} · {points[0]?.itemSetVersion} · {sessions}{" "}
              {sessions === 1 ? "run" : "runs"}
            </p>

            {/* Said before the numbers, not after. A curve from two
                people looks exactly like a curve from two hundred. */}
            {thin ? (
              <p className="mt-4 max-w-2xl rounded-md border border-signal bg-paper-2 px-5 py-4 text-sm text-ink-2">
                <strong className="text-ink">
                  {sessions} {sessions === 1 ? "run" : "runs"}: not enough to
                  claim anything.
                </strong>{" "}
                These figures are real and they are not evidence. Read them
                as a check that the instrument is recording what it should,
                not as a finding.
              </p>
            ) : null}
            {familyBlocked ? (
              <p className="mt-4 max-w-2xl rounded-md border border-signal bg-paper-2 px-5 py-4 text-sm text-ink-2">
                At least one load level here draws on a single item family
                ({points.filter((p) => !(p.families ?? "").includes("+")).map((p) => `${p.load}: ${p.families}`).join(", ")}),
                so that point measures the family as much as the load.
              </p>
            ) : null}

            <div className="mt-6 overflow-x-auto">
              <table className="w-full min-w-[42rem] text-sm">
                <thead className="text-left font-mono text-[10px] tracking-[0.14em] text-ink-3 uppercase">
                  <tr className="border-b border-line">
                    <th className="py-3 pr-4">Load</th>
                    <th className="py-3 pr-4">Answers</th>
                    <th className="py-3 pr-4">Correct</th>
                    <th className="py-3 pr-4">Confident</th>
                    <th className="py-3 pr-4">Gap</th>
                    <th className="py-3 pr-4">Lure</th>
                    <th className="py-3 pr-4">Sure and wrong</th>
                    <th className="py-3">Brier</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-line">
                  {points.map((p) => (
                    <tr key={p.load}>
                      <td className="py-3 pr-4 text-ink">
                        {LOAD_LABEL[p.load ?? ""] ?? p.load}
                        {p.load === "d3_control" ? (
                          <span className="ml-2 font-mono text-[10px] text-ink-3">
                            block 5
                          </span>
                        ) : null}
                      </td>
                      <td className="py-3 pr-4 tabular-nums text-ink-3">{p.answers ?? 0}</td>
                      <td className="py-3 pr-4 tabular-nums text-ink">{p.accuracyPct ?? 0}%</td>
                      <td className="py-3 pr-4 tabular-nums text-ink-2">
                        {p.meanConfidence ?? 0}%
                      </td>
                      {/* The research question in one column: positive is
                          more sure than right. */}
                      <td
                        className={`py-3 pr-4 tabular-nums ${
                          (p.gapPct ?? 0) > 0 ? "text-accent" : "text-ink-2"
                        }`}
                      >
                        {(p.gapPct ?? 0) > 0 ? "+" : ""}
                        {p.gapPct ?? 0}
                      </td>
                      <td className="py-3 pr-4 tabular-nums text-ink-2">{p.lurePct ?? 0}%</td>
                      <td className="py-3 pr-4 tabular-nums text-ink-2">
                        {p.confidentlyWrongPct ?? 0}%
                      </td>
                      <td className="py-3 tabular-nums text-ink-2">
                        {(p.meanBrier ?? 0).toFixed(2)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <p className="mt-4 max-w-2xl text-sm leading-relaxed text-ink-3">
              Of {sessions} {sessions === 1 ? "run" : "runs"},{" "}
              {points[0]?.sessionsFirstAttempt ?? 0} are first sittings and{" "}
              {points[0]?.sessionsRepeat ?? 0} are repeats;{" "}
              {points[0]?.sessionsUnreviewed ?? 0} have not been reviewed.
              A repeat is a different measurement from a first sitting and
              is counted separately rather than filtered out, so the choice
              stays yours.
            </p>
          </section>
        );
      })}

      {cal.length ? (
        <section className="mt-16">
          <h2 className="font-display text-2xl text-ink">Calibration</h2>
          <p className="mt-2 max-w-xl text-sm leading-relaxed text-ink-2">
            What each confidence band actually achieved. A well calibrated
            person who says 70% is right about 70% of the time. Overclaim
            is positive where a band is more sure than right.
          </p>
          <div className="mt-6 overflow-x-auto">
            <table className="w-full min-w-[32rem] text-sm">
              <thead className="text-left font-mono text-[10px] tracking-[0.14em] text-ink-3 uppercase">
                <tr className="border-b border-line">
                  <th className="py-3 pr-4">Load</th>
                  <th className="py-3 pr-4">Said</th>
                  <th className="py-3 pr-4">Answers</th>
                  <th className="py-3 pr-4">Actually right</th>
                  <th className="py-3">Overclaim</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {cal.map((c, i) => (
                  <tr key={i}>
                    <td className="py-2 pr-4 text-ink-3">
                      {LOAD_LABEL[c.load ?? ""] ?? c.load}
                    </td>
                    <td className="py-2 pr-4 tabular-nums text-ink">
                      {c.confidenceBand ?? 0}%
                    </td>
                    <td className="py-2 pr-4 tabular-nums text-ink-3">{c.answers ?? 0}</td>
                    <td className="py-2 pr-4 tabular-nums text-ink">{c.accuracyPct ?? 0}%</td>
                    <td
                      className={`py-2 tabular-nums ${
                        (c.overclaimPct ?? 0) > 0 ? "text-accent" : "text-ink-2"
                      }`}
                    >
                      {(c.overclaimPct ?? 0) > 0 ? "+" : ""}
                      {c.overclaimPct ?? 0}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      ) : null}

      {thresholds.length ? (
        <section className="mt-16">
          <h2 className="font-display text-2xl text-ink">
            Where each person turned
          </h2>
          <p className="mt-2 max-w-xl text-sm leading-relaxed text-ink-2">
            The lowest load on the ramp at which this participant took the
            intended wrong answer while still reporting high confidence.
            Blocks 1 to 4 only: block 5 is the fatigue control and is
            flagged separately, because being lured there is a claim about
            tiredness rather than about load.
          </p>
          <div className="mt-6 overflow-x-auto">
            <table className="w-full min-w-[38rem] text-sm">
              <thead className="text-left font-mono text-[10px] tracking-[0.14em] text-ink-3 uppercase">
                <tr className="border-b border-line">
                  <th className="py-3 pr-4">Run</th>
                  <th className="py-3 pr-4">Sitting</th>
                  <th className="py-3 pr-4">Correct</th>
                  <th className="py-3 pr-4">Gap</th>
                  <th className="py-3 pr-4">Turned at</th>
                  <th className="py-3 pr-4">Sure and wrong</th>
                  <th className="py-3">Block 5 vs 1</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {thresholds.map((t) => (
                  <tr key={t.sessionKey}>
                    <td className="py-2 pr-4">
                      <Link
                        href={`/admin/decision-test/${t.sessionKey}`}
                        className="font-mono text-[11px] text-ink no-underline hover:text-accent"
                      >
                        {t.sessionKey?.slice(0, 8)}
                      </Link>
                      {t.reviewStatus ? (
                        <span className="ml-2 font-mono text-[10px] text-ink-3">
                          {t.reviewStatus.replace("_", " ")}
                        </span>
                      ) : null}
                    </td>
                    <td className="py-2 pr-4 tabular-nums text-ink-3">
                      {t.attemptNo ? t.attemptNo : "unknown"}
                    </td>
                    <td className="py-2 pr-4 tabular-nums text-ink">{t.accuracyPct ?? 0}%</td>
                    <td
                      className={`py-2 pr-4 tabular-nums ${
                        (t.gapPct ?? 0) > 0 ? "text-accent" : "text-ink-2"
                      }`}
                    >
                      {(t.gapPct ?? 0) > 0 ? "+" : ""}
                      {t.gapPct ?? 0}
                    </td>
                    <td className="py-2 pr-4 text-ink-2">
                      {t.luredAtRank
                        ? LOAD_LABEL[t.luredAtLoad ?? ""] ?? t.luredAtLoad
                        : "never"}
                      {t.luredInControl ? (
                        <span className="ml-2 font-mono text-[10px] text-accent">
                          control too
                        </span>
                      ) : null}
                    </td>
                    <td className="py-2 pr-4 tabular-nums text-ink-2">
                      {t.confidentlyWrong ?? 0}
                    </td>
                    <td className="py-2 tabular-nums text-ink-2">
                      {(t.fatigueDeltaPct ?? 0) > 0 ? "+" : ""}
                      {t.fatigueDeltaPct ?? 0}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      ) : null}
    </>
  );
}
