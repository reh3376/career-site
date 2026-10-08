import Link from "next/link";
import { notFound } from "next/navigation";
import type { Metadata } from "next";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import type { Run } from "../runs";
import { attemptEvidence, attemptLabel } from "../runs";
import { BlockReview, SessionReview } from "../review";
import { RunExport } from "../run-export";

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
  digitsCorrect?: number;
  digitsHeld?: number;
  recallLatencyMs?: number;
  meanLatencyVsBaseline?: number;
};
type Answer = {
  position: number;
  blockNo: number;
  itemCode: string;
  itemFamily: string;
  outcome: string;
  confidence?: number;
  latencyMs?: number;
  prompt?: string;
  chosenText?: string;
  correctText?: string;
  chosenIndex?: number;
  positionInBlock?: number;
  itemVersion?: number;
  isLure?: boolean;
  confidentlyWrong?: boolean;
  confidentlyLured?: boolean;
  latencyVsBaseline?: number;
  brier?: number;
};
type DrawnItem = {
  position?: number;
  blockNo?: number;
  itemCode?: string;
  family?: string;
  answered?: boolean;
  fixture?: boolean;
};
type ReviewEvent = {
  blockNo?: number;
  status?: string;
  reason?: string;
  note?: string;
  reviewedBy?: string;
  createdAt?: string;
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
    reviewHistory?: ReviewEvent[];
    drawnItems?: DrawnItem[];
  };
  const run = res?.run;
  if (!run) notFound();
  const blocks = res?.blocks ?? [];
  const answers = res?.answers ?? [];
  const history = res?.reviewHistory ?? [];
  const drawn = res?.drawnItems ?? [];

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
          run.keyVersion,
          run.wantsResults ? "wants results" : null,
          run.finishedAt ? `ended ${run.finishedAt.slice(0, 16).replace("T", " ")}` : null,
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
          run.baselineRtMs ? `tap offset ${run.baselineRtMs}ms` : null,
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
              <th className="py-3 pr-4">Pace</th>
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
                {/* Per block, which is where it means something: the
                    load effect is the research question and this is the
                    scale on which two participants can be compared. */}
                <td className="py-3 pr-4 tabular-nums text-ink-2">
                  {b.meanLatencyVsBaseline
                    ? Math.round(b.meanLatencyVsBaseline)
                    : ","}
                </td>
                <td className="py-3 pr-4 tabular-nums text-ink-2">
                  {(b.memoryFailurePct ?? -1) < 0
                    ? "not scored"
                    : `${b.memoryFailurePct}%`}
                  {/* The counts the percentage came from. A reviewer
                      judging a block wants the reading and the digits
                      it was computed from, not one standing in for the
                      other. */}
                  {/* held is scored against whichever number the
                      response is closer to, so it is the retention
                      reading rather than the length; the length comes
                      from the number that was actually shown. */}
                  {b.presentedDigits ? (
                    <span className="block font-mono text-[10px] text-ink-3">
                      held {b.digitsHeld ?? 0}/{b.presentedDigits.length}
                      {", "}
                      {b.digitsCorrect ?? 0} correct
                      {b.recallLatencyMs
                        ? `, ${Math.round(b.recallLatencyMs / 100) / 10}s`
                        : ""}
                    </span>
                  ) : null}
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

      {/* Said once, plainly, because the unit is not self-explanatory
          and a reader who guesses will guess wrong. */}
      <section className="mt-12 rounded-md border border-line bg-paper-2 px-6 py-5">
        <h2 className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
          Reading the pace figures
        </h2>
        <p className="mt-4 max-w-2xl text-sm leading-relaxed text-ink-2">
          Pace is answer time divided by this participant&rsquo;s tap-check
          offset, {run.baselineRtMs || "about 90 to 300"}ms here. That offset
          is how far their presses sat from the metronome, not a reaction
          time, so{" "}
          <strong className="text-ink">
            a pace of 174 does not mean 174 times slower than normal
          </strong>
          . It means that answer took 174 offsets.
        </p>
        <p className="mt-3 max-w-2xl text-sm leading-relaxed text-ink-2">
          Within one run the denominator never changes, so pace ranks
          answers in exactly the order plain time does and adds nothing.
          It is worth having across runs and across blocks, where it puts
          a naturally quick and a naturally slow participant on one scale:
          block 4 against block 1, person against person.
        </p>
      </section>

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
              <th className="py-3 pr-4">Chose</th>
              <th className="py-3 pr-4">Confidence</th>
              <th className="py-3 pr-4">Time</th>
              <th className="py-3">Brier</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {answers.map((a) => (
              <tr key={a.position}>
                <td className="py-2 pr-4 tabular-nums text-ink-3">{a.position}</td>
                <td className="py-2 pr-4 font-mono text-[11px] text-ink">{a.itemCode}</td>
                <td className="py-2 pr-4 text-ink-3">{a.itemFamily}</td>
                <td className="py-2 pr-4 text-ink">
                  {a.outcome}
                  {/* The research question, per answer: wrong and sure
                      about it. Flagged here so a reviewer can see where
                      in the run it started rather than inferring it
                      from a block average. */}
                  {a.confidentlyWrong ? (
                    <span className="ml-2 font-mono text-[10px] tracking-[0.1em] text-accent uppercase">
                      sure
                    </span>
                  ) : null}
                </td>
                {/* Which option, by index. The option TEXT is withheld
                    here and kept in the collapsed section below, so the
                    answer key is not sitting in a screenshot of this
                    table. The index alone is the curation signal that
                    matters: a column of identical indices is somebody
                    clicking through, and some of those clicks are
                    correct by chance, so the outcome column cannot show
                    it. */}
                <td className="py-2 pr-4 font-mono text-[11px] tabular-nums text-ink-2">
                  {/* Absent means zero, not missing.
                      chosen_index is a plain int32 and proto3 implicit
                      presence drops a zero from the JSON, so the single
                      commonest answer, option one, arrived as undefined
                      and rendered blank. The contract cannot change
                      (additive-only), and it does not need to: an
                      expired answer is already identifiable from the
                      outcome, so undefined can safely mean 0 here. */}
                  {a.outcome === "expired" || (a.chosenIndex ?? 0) < 0
                    ? ","
                    : (a.chosenIndex ?? 0)}
                  {a.isLure ? (
                    <span className="ml-2 tracking-[0.1em] text-ink-3 uppercase">lure</span>
                  ) : null}
                </td>
                <td className="py-2 pr-4 tabular-nums text-ink-2">
                  {a.outcome === "expired" ? "" : `${a.confidence ?? 0}%`}
                </td>
                <td className="py-2 pr-4 tabular-nums text-ink-2">
                  {a.outcome === "expired"
                    ? ""
                    : `${Math.round((a.latencyMs ?? 0) / 100) / 10}s`}
                  {/* Labelled by the unit, not as "x base". The
                      denominator is the tap-check offset and calling it
                      a baseline invites reading 174 as "174 times
                      slower", which is the misreading this wording
                      exists to prevent. The section above states the
                      unit in full. */}
                  {a.latencyVsBaseline ? (
                    <span className="block font-mono text-[10px] text-ink-3">
                      {Math.round(a.latencyVsBaseline)} offsets
                    </span>
                  ) : null}
                </td>
                <td className="py-2 tabular-nums text-ink-2">
                  {(a.brier ?? -1) < 0 ? "," : a.brier?.toFixed(2)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <RunExport sessionKey={key} />

      {/* The questions themselves, and the key, behind a deliberate
          click.

          This page said "item codes, not questions. The bank is not
          shown here, because an answer key that escapes contaminates a
          standardized instrument permanently and it only has to escape
          once." That reasoning still holds and the default is still
          closed. But a reviewer cannot always judge a run without
          seeing what was actually asked and what was actually picked:
          an answer that looks like carelessness and an answer that
          looks like a misread item are the same row until you read it.

          So it is here, one click away, rather than at /admin/db. The
          click is the point: nothing on this page reveals the key until
          somebody asks for it, so a screenshot of the run, the blocks
          or the answer table never carries it. */}
      <details className="mt-10 rounded-md border border-line bg-paper-2 px-6 py-5">
        <summary className="cursor-pointer font-mono text-[11px] tracking-[0.14em] text-ink-2 uppercase">
          Show the questions and the key for this run
        </summary>
        <p className="mt-4 max-w-xl text-sm text-ink-3">
          Closed by default on purpose. An answer key that escapes
          contaminates a standardized instrument permanently, and it only
          has to escape once. Do not screenshot this open.
        </p>
        <ol className="mt-6 space-y-5">
          {answers.map((a) => (
            <li key={a.position} className="border-b border-line pb-4 last:border-0">
              <p className="font-mono text-[10px] tracking-[0.14em] text-ink-3 uppercase">
                {a.position}. {a.itemCode}
                {a.itemVersion ? ` v${a.itemVersion}` : ""} · block {a.blockNo}
                {a.positionInBlock ? `, q${a.positionInBlock}` : ""}
              </p>
              <p className="mt-2 text-sm leading-relaxed text-ink">{a.prompt}</p>
              <p className="mt-2 text-sm text-ink-2">
                Chose{" "}
                <strong className={a.outcome === "correct" ? "text-ink" : "text-accent"}>
                  {a.chosenText || "nothing, it expired"}
                </strong>
                {a.correctText && a.outcome !== "correct" ? (
                  <>
                    {" "}
                    · correct was <strong className="text-ink">{a.correctText}</strong>
                  </>
                ) : null}
              </p>
            </li>
          ))}
        </ol>
      </details>

      {/* The questions this run was given, including the ones it never
          reached.

          Separate from the answer table on purpose. An abandoned run
          has six answers and says nothing about the twenty-four
          questions that had already been chosen for it, and the
          difference between "stopped at question six" and "was asked
          six questions" matters when judging whether a short run is
          usable. Since 2026-10-07 every run draws its own thirty from
          the category pools, so which thirty is now a property of the
          run rather than of the instrument. */}
      {drawn.length ? (
        <>
          <h2 className="font-display mt-14 text-2xl text-ink">
            The questions this run drew
          </h2>
          <p className="mt-3 max-w-xl text-sm text-ink-3">
            {drawn.length} drawn, {drawn.filter((d) => d.answered).length}{" "}
            reached. Codes and categories only; the questions themselves are
            in the section above.
            {drawn.some((d) => d.fixture) ? (
              <strong className="block mt-2 text-signal">
                This run was served placeholder items, not the instrument.
              </strong>
            ) : null}
          </p>
          <div className="mt-6 overflow-x-auto">
            <table className="w-full min-w-[28rem] text-sm">
              <thead className="text-left font-mono text-[10px] tracking-[0.14em] text-ink-3 uppercase">
                <tr className="border-b border-line">
                  <th className="py-3 pr-4">#</th>
                  <th className="py-3 pr-4">Block</th>
                  <th className="py-3 pr-4">Item</th>
                  <th className="py-3 pr-4">Category</th>
                  <th className="py-3">Reached</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {drawn.map((d) => (
                  <tr key={d.position} className={d.answered ? "" : "opacity-50"}>
                    <td className="py-2 pr-4 tabular-nums text-ink-3">{d.position}</td>
                    <td className="py-2 pr-4 tabular-nums text-ink-3">{d.blockNo}</td>
                    <td className="py-2 pr-4 font-mono text-[11px] text-ink">
                      {d.itemCode}
                      {d.fixture ? (
                        <span className="ml-2 text-signal">fixture</span>
                      ) : null}
                    </td>
                    <td className="py-2 pr-4 text-ink-3">{d.family}</td>
                    <td className="py-2 text-ink-2">{d.answered ? "yes" : "no"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      ) : null}

      {/* The audit trail. Written on every judgement since the curation
          layer shipped and readable only through the SQL console until
          now, which made "an exclusion can always be explained" true of
          the database and false of anybody trying to explain one. */}
      <h2 className="font-display mt-14 text-2xl text-ink">Curation history</h2>
      {history.length ? (
        <ol className="mt-6 space-y-3">
          {history.map((e, i) => (
            <li
              key={i}
              className="flex flex-wrap gap-x-3 gap-y-1 border-b border-line pb-3 text-sm text-ink-2"
            >
              <span className="font-mono text-[11px] tracking-[0.1em] text-ink-3">
                {e.createdAt?.slice(0, 16).replace("T", " ")}
              </span>
              <span className="text-ink">
                {e.blockNo ? `block ${e.blockNo}` : "whole run"}
                {": "}
                {e.status || "cleared"}
              </span>
              {e.reason ? <span className="text-ink-3">{e.reason}</span> : null}
              {e.reviewedBy ? <span className="text-ink-3">by {e.reviewedBy}</span> : null}
              {e.note ? <span className="w-full text-ink-2">{e.note}</span> : null}
            </li>
          ))}
        </ol>
      ) : (
        <p className="mt-4 text-sm text-ink-3">
          Nothing recorded yet. Every judgement on this run, including a
          cleared one, is appended here and nothing is ever removed.
        </p>
      )}
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
 * Absent means there was no such link at all. Zero means there was one
 * and it saw no earlier sittings. Writing "no account" rather than
 * "account saw 0" keeps that distinction legible, because it is the
 * distinction the whole three-column design exists to preserve.
 *
 * Read from `priorSittings`, whose fields are `optional`, for exactly
 * that reason. The flat prior_by_* fields were plain int32 and proto3
 * implicit presence dropped every zero from the JSON, so a real "saw
 * nothing" arrived as undefined and this function confidently reported
 * "no account" for an account that existed. Found by reading the
 * deployed page against the database, not by any test.
 *
 * The flat fields could not simply be changed: career.v1 is
 * additive-only and buf breaking refuses a cardinality change on a
 * published field, so they are deprecated and this message replaces
 * them.
 */
function priorsRead(run: Run): string | null {
  const read = (label: string, v?: number) =>
    v === undefined || v === null ? `no ${label}` : `${label} saw ${v}`;
  const p = run.priorSittings;
  const parts = [
    read("account", p?.byAccount),
    read("email", p?.byEmail),
    read("cookie", p?.byCookie),
  ];
  // Nothing to say when no identity existed at all; the label already
  // reads "sequence unknown" and repeating it three ways is noise.
  if (parts.every((p) => p.startsWith("no "))) return null;
  return parts.join(", ");
}
