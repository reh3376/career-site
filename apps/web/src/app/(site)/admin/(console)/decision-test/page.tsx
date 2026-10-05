import type { Metadata } from "next";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import { setDecisionTestTimingsAction } from "./actions";
import { RunTable, type Run } from "./runs";

export const metadata: Metadata = { title: "Admin · Decision test" };

type Settings = {
  memoriseMs?: number;
  questionMs?: number;
  recallMs?: number;
  instrumentVersion?: string;
  sessionsOnThisVersion?: number;
};

export default async function DecisionTestPage() {
  const cookie = await getSessionCookie();
  let s: Settings = {};
  let runs: Run[] = [];
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
      body: { includeSynthetic: false },
      cookie,
    });
    if (rr.ok) runs = (((await rr.json()) as { runs?: Run[] }).runs ?? []);
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

      <RunTable runs={runs} />

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
