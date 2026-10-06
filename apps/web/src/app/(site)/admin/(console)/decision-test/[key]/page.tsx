import Link from "next/link";
import { notFound } from "next/navigation";
import type { Metadata } from "next";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import type { Run } from "../runs";
import { attemptEvidence, attemptLabel } from "../runs";
import { BlockReview, SessionReview } from "../review";

export const metadata: Metadata = { title: "Admin · Decision test run" };

type Block = {
  blockNo: number;
  load: string;
  correct?: number;
  total?: number;
  lure?: number;
  expired?: number;
  meanConfidence?: number;
  meanLatencyMs?: number;
  presentedDigits?: string;
  expectedDigits?: string;
  responseDigits?: string;
  recallOutcome?: string;
  memoryFailurePct?: number;
  reviewStatus?: string;
  reviewNote?: string;
};
type Answer = {
  position: number;
  blockNo: number;
  itemCode: string;
  itemFamily: string;
  outcome: string;
  confidence?: number;
  latencyMs?: number;
};

const LOAD_LABEL: Record<string, string> = {
  d3: "3 digits",
  d4: "4 digits",
  d4_plus1: "4 digits, +1",
  d4_plus3: "4 digits, +3",
  d3_control: "3 digits, control",
};

export default async function RunPage({ params }: { params: Promise<{ key: string }> }) {
  const { key } = await params;
  const cookie = await getSessionCookie();
  if (!cookie) notFound();
  const resp = await callApi({
    path: "/api/career.v1.AdminService/GetDecisionTestRun",
    body: { sessionKey: key },
    cookie,
  });
  if (!resp.ok) notFound();
  const res = (await resp.json()) as {
    run?: Run;
    blocks?: Block[];
    answers?: Answer[];
  };
  const run = res?.run;
  if (!run) notFound();
  const blocks = res?.blocks ?? [];
  const answers = res?.answers ?? [];

  const pct = (c?: number, t?: number) => (t ? Math.round((100 * (c ?? 0)) / t) : 0);
  const early = blocks.filter((b) => b.load === "d3" || b.load === "d4");
  const hard = blocks.find((b) => b.load === "d4_plus3");
  const b1 = blocks.find((b) => b.load === "d3");
  const b5 = blocks.find((b) => b.load === "d3_control");
  const earlyAcc = pct(
    early.reduce((n, b) => n + (b.correct ?? 0), 0),
    early.reduce((n, b) => n + (b.total ?? 0), 0),
  );
  const earlyConf = early.length
    ? Math.round(early.reduce((n, b) => n + (b.meanConfidence ?? 0), 0) / early.length)
    : 0;

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
        {run.displayName || "Anonymous"}.
      </h1>
      <p className="mt-4 font-mono text-[11px] tracking-[0.1em] text-ink-3">
        {[
          run.startedAt?.slice(0, 16).replace("T", " "),
          run.status,
          run.ageRange,
          run.education,
          run.occupation,
          run.deviceClass,
          run.audioMode,
          run.gaveEmail ? "email given" : "no email",
          run.instrumentVersion,
          run.itemSetVersion,
        ]
          .filter(Boolean)
          .join("  ·  ")}
      </p>

      {/* The conditions this run happened under. Recorded on every
          session since the first one and shown nowhere until now, which
          made a reviewer guess at exactly the things that decide whether
          a run is sound. recall_strategy is the sharpest of them: it is
          the debrief answer about whether the participant converted the
          number at encoding or carried it and transformed at recall, and
          the two produce different loads during the questions. */}
      <p className="mt-2 font-mono text-[11px] tracking-[0.1em] text-ink-3">
        {[
          run.tapCheckPassed === false ? "tap check FAILED" : "tap check passed",
          run.baselineRtMs ? `baseline ${run.baselineRtMs}ms` : null,
          run.baselineRtSdMs ? `sd ${run.baselineRtSdMs}ms` : null,
          run.recallStrategy ? `held the number: ${run.recallStrategy}` : "strategy not given",
          run.isRepeat ? `repeat, matched by ${run.repeatMatchedBy || "unknown"}` : null,
        ]
          .filter(Boolean)
          .join("  ·  ")}
      </p>

      {/* Which sitting this was, and what that claim rests on.
          The owner's requirement was that a first run be labelled as
          one and every later run as what it is. The number is derived
          from three point-in-time observations rather than decided when
          the run started, so the observations are shown beside it: a
          sequence resting on a cookie is weaker evidence than one
          resting on an account, and that is the reviewer's call to
          make, not the schema's. */}
      <p className="mt-2 font-mono text-[11px] tracking-[0.1em] text-ink-3">
        {[
          attemptLabel(run),
          attemptEvidence(run),
          priorsRead(run),
        ]
          .filter(Boolean)
          .join("  ·  ")}
      </p>

      <SessionReview
        sessionKey={key}
        status={run.reviewStatus}
        reason={run.reviewReason}
        note={run.reviewNote}
        reviewedAt={run.reviewedAt}
        blocksExcluded={run.blocksExcluded}
      />

      {/* The headline. Accuracy against confidence is the whole research
          question: being wrong is ordinary, being wrong and sure is the
          finding. */}
      <section className="mt-12 rounded-md border border-line bg-paper-2 px-6 py-6">
        <h2 className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
          Accuracy against confidence
        </h2>
        <dl className="mt-5 grid gap-4 sm:grid-cols-2">
          <Stat label="Early blocks" value={`${earlyAcc}% correct`} sub={`${earlyConf}% confident`} />
          <Stat
            label="Hardest block"
            value={`${pct(hard?.correct, hard?.total)}% correct`}
            sub={`${hard?.meanConfidence ?? 0}% confident`}
          />
        </dl>
        <p className="mt-5 text-sm leading-relaxed text-ink-2">
          Accuracy moved{" "}
          <strong className="text-ink">
            {earlyAcc - pct(hard?.correct, hard?.total)} points
          </strong>
          , confidence moved{" "}
          <strong className="text-ink">
            {earlyConf - (hard?.meanConfidence ?? 0)} points
          </strong>
          . The gap is the result.
        </p>
      </section>

      {/* Block 5 against block 1 is the only thing that separates load
          from fatigue. Same difficulty, twelve minutes apart. */}
      <section className="mt-6 rounded-md border border-line px-6 py-5">
        <h2 className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
          The fatigue control
        </h2>
        <p className="mt-3 text-sm leading-relaxed text-ink-2">
          Block 1 <strong className="text-ink">{pct(b1?.correct, b1?.total)}%</strong>,
          block 5 <strong className="text-ink">{pct(b5?.correct, b5?.total)}%</strong>.
          Same difficulty, twelve minutes apart. Close together means the
          decline in between was load; a drop at block 5 means the fifteen
          minutes did some of the work.
        </p>
      </section>

      <h2 className="font-display mt-14 text-2xl text-ink">By block</h2>
      <div className="mt-6 overflow-x-auto">
        <table className="w-full min-w-[46rem] text-sm">
          <thead className="text-left font-mono text-[10px] tracking-[0.14em] text-ink-3 uppercase">
            <tr className="border-b border-line">
              <th className="py-3 pr-4">Block</th>
              <th className="py-3 pr-4">Correct</th>
              <th className="py-3 pr-4">Lure</th>
              <th className="py-3 pr-4">Expired</th>
              <th className="py-3 pr-4">Confidence</th>
              <th className="py-3 pr-4">Mean time</th>
              <th className="py-3 pr-4">Number</th>
              <th className="py-3 pr-4">Recall</th>
              <th className="py-3 pr-4">Memory lost</th>
              <th className="py-3">Use</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {blocks.map((b) => (
              <tr
                key={b.blockNo}
                className={b.reviewStatus === "do_not_use" ? "opacity-50" : undefined}
              >
                <td className="py-3 pr-4 text-ink">{LOAD_LABEL[b.load] ?? b.load}</td>
                <td className="py-3 pr-4 tabular-nums text-ink">
                  {b.correct ?? 0}/{b.total ?? 0}
                </td>
                <td className="py-3 pr-4 tabular-nums text-ink-2">{b.lure ?? 0}</td>
                <td className="py-3 pr-4 tabular-nums text-ink-2">{b.expired ?? 0}</td>
                <td className="py-3 pr-4 tabular-nums text-ink-2">{b.meanConfidence ?? 0}%</td>
                <td className="py-3 pr-4 tabular-nums text-ink-2">
                  {Math.round((b.meanLatencyMs ?? 0) / 100) / 10}s
                </td>
                <td className="py-3 pr-4 font-mono text-[11px] text-ink-2 tabular-nums">
                  {b.presentedDigits}
                  {b.expectedDigits && b.expectedDigits !== b.presentedDigits
                    ? ` → ${b.expectedDigits}`
                    : ""}
                  {b.responseDigits ? `, gave ${b.responseDigits}` : ""}
                </td>
                <td className="py-3 pr-4">
                  <RecallTag outcome={b.recallOutcome} />
                </td>
                {/* Severity rather than cause. A typo, a transposition
                    and a genuine lapse are indistinguishable at the
                    moment somebody types four digits, so what is scored
                    is how much of the number survived. Measured against
                    whichever of the two numbers the answer is closer to,
                    so an untransformed answer reads as 0% lost: the
                    number was held, only the operation failed. */}
                <td className="py-3 pr-4 tabular-nums text-ink-2">
                  {(b.memoryFailurePct ?? -1) < 0
                    ? "not scored"
                    : `${b.memoryFailurePct}%`}
                </td>
                {/* Per block, so a phone ringing during block three
                    costs six answers rather than thirty. The run's own
                    status is separate and above. */}
                <td className="py-3">
                  <BlockReview
                    sessionKey={key}
                    blockNo={b.blockNo}
                    status={b.reviewStatus}
                    note={b.reviewNote}
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <h2 className="font-display mt-14 text-2xl text-ink">Every answer</h2>
      <p className="mt-3 max-w-xl text-sm text-ink-3">
        Item codes, not questions. The bank is not shown here, because an
        answer key that escapes contaminates a standardized instrument
        permanently and it only has to escape once.
      </p>
      <div className="mt-6 overflow-x-auto">
        <table className="w-full min-w-[34rem] text-sm">
          <thead className="text-left font-mono text-[10px] tracking-[0.14em] text-ink-3 uppercase">
            <tr className="border-b border-line">
              <th className="py-3 pr-4">#</th>
              <th className="py-3 pr-4">Item</th>
              <th className="py-3 pr-4">Family</th>
              <th className="py-3 pr-4">Outcome</th>
              <th className="py-3 pr-4">Confidence</th>
              <th className="py-3">Time</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {answers.map((a) => (
              <tr key={a.position}>
                <td className="py-2 pr-4 tabular-nums text-ink-3">{a.position}</td>
                <td className="py-2 pr-4 font-mono text-[11px] text-ink">{a.itemCode}</td>
                <td className="py-2 pr-4 text-ink-3">{a.itemFamily}</td>
                <td className="py-2 pr-4 text-ink">{a.outcome}</td>
                <td className="py-2 pr-4 tabular-nums text-ink-2">
                  {a.outcome === "expired" ? "" : `${a.confidence ?? 0}%`}
                </td>
                <td className="py-2 tabular-nums text-ink-2">
                  {a.outcome === "expired"
                    ? ""
                    : `${Math.round((a.latencyMs ?? 0) / 100) / 10}s`}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

function Stat({ label, value, sub }: { label: string; value: string; sub: string }) {
  return (
    <div>
      <dt className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">{label}</dt>
      <dd className="font-display mt-2 text-3xl text-ink tabular-nums">{value}</dd>
      <dd className="mt-1 text-sm text-ink-2">{sub}</dd>
    </div>
  );
}

// untransformed is the one worth spotting: the number survived and the
// operation did not, which is storage holding while the executive fails.
function RecallTag({ outcome }: { outcome?: string }) {
  if (!outcome) return <span className="text-ink-4">,</span>;
  const strong = outcome === "untransformed";
  return (
    <span
      className={`rounded-sm px-2 py-0.5 font-mono text-[10px] tracking-[0.14em] uppercase ${
        strong ? "bg-accent-soft/60 text-accent" : "bg-paper-3 text-ink-3"
      }`}
    >
      {outcome.replace("_", " ")}
    </span>
  );
}

/**
 * The three observations, spelled out.
 *
 * -1 is the wire's way of saying there was no such link at all, which
 * is not the same as a link that existed and saw nothing. Writing "no
 * account" rather than "account: 0" keeps that distinction legible,
 * because it is the distinction the whole three-column design exists
 * to preserve.
 */
function priorsRead(run: Run): string | null {
  const read = (label: string, v?: number) =>
    v === undefined || v < 0 ? `no ${label}` : `${label} saw ${v}`;
  const parts = [
    read("account", run.priorByAccount),
    read("email", run.priorByEmail),
    read("cookie", run.priorByCookie),
  ];
  // Nothing to say when no identity existed at all; the label already
  // reads "sequence unknown" and repeating it three ways is noise.
  if (parts.every((p) => p.startsWith("no "))) return null;
  return parts.join(", ");
}
