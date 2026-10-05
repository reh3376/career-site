import Link from "next/link";
import type { Metadata } from "next";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import { setDecisionTestTimingsAction } from "./actions";
import { ExportPanel } from "./export";
import { RunTable, type Run, type ReviewCounts } from "./runs";

export const metadata: Metadata = { title: "Admin · Decision test" };

type Settings = {
  memoriseMs?: number;
  questionMs?: number;
  recallMs?: number;
  instrumentVersion?: string;
  sessionsOnThisVersion?: number;
};

export default async function DecisionTestPage({
  searchParams,
}: {
  searchParams: Promise<{ review?: string }>;
}) {
  const { review } = await searchParams;
  const cookie = await getSessionCookie();
  let s: Settings = {};
  let runs: Run[] = [];
  let counts: ReviewCounts = {};
  if (cookie) {
    // callApi returns the fetch Response, not the parsed body. The first
    // version of this page cast the Response straight to Settings, so
    // every field read as undefined and rendered as 0, and the run list
    // rendered as "no runs yet" while two real runs sat in the database.
    // The `as` cast is what hid it: it silenced the exact type error
    // that would have caught the mistake, at the one boundary where the
    // checker had something useful to say.
    const sr = await callApi({
      path: "/api/career.v1.AdminService/GetDecisionTestSettings",
      body: {},
      cookie,
    });
    if (sr.ok) s = ((await sr.json()) as Settings) ?? {};

    const rr = await callApi({
      path: "/api/career.v1.AdminService/ListDecisionTestRuns",
      // The filter goes to the api rather than being applied here, so
      // the 200-row cap bites on the filtered set: filtering a truncated
      // page would quietly hide runs from the queue.
      body: { includeSynthetic: false, reviewStatus: review ?? "" },
      cookie,
    });
    if (rr.ok) {
      const body = (await rr.json()) as { runs?: Run[]; counts?: ReviewCounts };
      runs = body.runs ?? [];
      counts = body.counts ?? {};
    }
  }
  const secs = (ms?: number) => Math.round((ms ?? 0) / 1000);

  return (
    <>
      <h1
        className="font-display text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Decision test.
      </h1>
      <p className="mt-6 max-w-xl text-base leading-relaxed text-ink-2">
        Every run, and the timings they were taken under. Agent-driven
        runs are excluded: they prove the pipeline works and are not
        data.
      </p>

      {/* The queue. Reviewing is work, so what is left to look at
          belongs at the top rather than being something to count by
          eye. The totals are across every real run, not the filtered
          page, so switching filters does not change what "14 left"
          means. */}
      <nav className="mt-10 flex flex-wrap gap-2">
        <Chip href="/admin/decision-test" active={!review} label="All" n={counts.total} />
        <Chip
          href="/admin/decision-test?review=unreviewed"
          active={review === "unreviewed"}
          label="Unreviewed"
          n={counts.unreviewed}
        />
        <Chip
          href="/admin/decision-test?review=good"
          active={review === "good"}
          label="Good"
          n={counts.good}
        />
        <Chip
          href="/admin/decision-test?review=incomplete"
          active={review === "incomplete"}
          label="Incomplete"
          n={counts.incomplete}
        />
        <Chip
          href="/admin/decision-test?review=hold"
          active={review === "hold"}
          label="Hold"
          n={counts.hold}
        />
        <Chip
          href="/admin/decision-test?review=do_not_use"
          active={review === "do_not_use"}
          label="Do not use"
          n={counts.doNotUse}
        />
      </nav>

      <RunTable runs={runs} />

      <h2 className="font-display mt-20 text-2xl text-ink">The dataset</h2>
      <p className="mt-3 max-w-xl text-base leading-relaxed text-ink-2">
        One row per question presented, which is the grain everything
        else is counted from. Served from the same view the console and
        the metric views read, so an export and a page can never
        disagree about what a number means.
      </p>
      <p className="mt-3 max-w-xl text-base leading-relaxed text-ink-2">
        It carries no name, no email address and no chosen option. The
        option index across enough runs would let somebody rebuild the
        answer key, and the key only has to escape once.
      </p>
      <ExportPanel />

      <h2 className="font-display mt-20 text-2xl text-ink">Timings</h2>
      <p className="mt-3 max-w-xl text-base leading-relaxed text-ink-2">
        How long a participant gets for each phase. These were constants
        until the first two live runs each moved them, and changing a
        number should not need a deploy.
      </p>

      <form action={setDecisionTestTimingsAction} className="mt-8 max-w-md space-y-6">
        <Field
          name="memorise_s"
          label="Seconds to memorise the number"
          value={secs(s.memoriseMs)}
          note="Shown before the questions begin. Two was the original figure and proved unreadable: four digits plus a transformation rule needs longer than that."
        />
        <Field
          name="question_s"
          label="Seconds per question"
          value={secs(s.questionMs)}
          note="Covers reading, deciding and rating confidence. A question that runs out is recorded as expired rather than wrong."
        />
        <Field
          name="recall_s"
          label="Seconds to enter the number"
          value={secs(s.recallMs)}
          note="At the end of each block."
        />
        <button
          type="submit"
          className="rounded-md bg-accent px-8 py-3 font-mono text-[11px] tracking-[0.14em] text-canvas uppercase"
        >
          Save
        </button>
      </form>

      <section className="mt-16 max-w-xl border-t border-line pt-8">
        <h2 className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
          What a change costs
        </h2>
        <p className="mt-4 text-base leading-relaxed text-ink-2">
          The instrument version is derived from these three numbers, so
          saving a different value starts a new one. That is the point:
          a question answered in twenty seconds and the same question
          answered in twenty-five are not the same measurement, and
          pooling them is the kind of mistake that never announces
          itself.
        </p>
        <dl className="mt-6 space-y-3 text-sm">
          <div className="flex justify-between gap-6">
            <dt className="text-ink-3">Current version</dt>
            <dd className="font-mono text-ink">{s.instrumentVersion ?? "unknown"}</dd>
          </div>
          <div className="flex justify-between gap-6">
            <dt className="text-ink-3">Real runs recorded under it</dt>
            <dd className="font-mono text-ink">{s.sessionsOnThisVersion ?? 0}</dd>
          </div>
        </dl>
        <p className="mt-6 text-sm text-ink-3">
          Those runs are not lost by a change. They keep their own
          version and stay queryable; they simply stop being comparable
          with what follows.
        </p>
      </section>
    </>
  );
}

// A filter with its count. The count is what makes it a queue rather
// than a set of tabs.
function Chip({
  href,
  active,
  label,
  n,
}: {
  href: string;
  active: boolean;
  label: string;
  n?: number;
}) {
  return (
    <Link
      href={href}
      className={`rounded-md px-4 py-2 font-mono text-[11px] tracking-[0.12em] uppercase no-underline ${
        active ? "bg-accent text-canvas" : "border border-line text-ink-2 hover:text-accent"
      }`}
    >
      {label} {n ?? 0}
    </Link>
  );
}

function Field({
  name,
  label,
  value,
  note,
}: {
  name: string;
  label: string;
  value: number;
  note: string;
}) {
  return (
    <label className="block">
      <span className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
        {label}
      </span>
      <input
        name={name}
        type="number"
        min={1}
        max={120}
        defaultValue={value}
        className="mt-2 w-32 rounded-md border border-line bg-canvas px-4 py-3 text-base text-ink tabular-nums"
      />
      <span className="mt-2 block text-sm text-ink-3">{note}</span>
    </label>
  );
}
