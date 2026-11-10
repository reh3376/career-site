import type { Metadata } from "next";
import Link from "next/link";

import { CommentBox } from "../comment-box";
import { requireSignedIn, unwrap } from "../gate";
import { getPilotWorkflow, type PilotComment } from "../lib";
import { FlowStrip } from "./flow-strip";
import { PhaseStrip } from "./phase-strip";
import { parseWorkflow, progressByPhase } from "./types";

export const metadata: Metadata = { title: "Pilot · How a company hires" };
export const dynamic = "force-dynamic";

// The company hiring workflow, revision 3 (FSD FR-PLT-09, amended
// 2026-11-09 from the owner's comments after his own run: the 26-step
// chart could not be evaluated as a whole, the summaries used our
// vocabulary, and the questions did not say what was wanted). This
// screen is the overview: the five stages as one small diagram and one
// question about them; each stage then has a screen of its own.
export default async function PilotWorkflowPage() {
  await requireSignedIn("/pilot/workflow");
  const data = unwrap(await getPilotWorkflow());
  const wf = parseWorkflow(data.workflowJson);
  const comments = new Map<string, PilotComment>();
  for (const c of data.comments ?? []) comments.set(`${c.targetKind}:${c.targetId ?? ""}`, c);
  const phases = wf?.phases ?? [];
  const nodes = wf?.nodes ?? [];
  const progress = progressByPhase(phases, nodes, comments);
  const next = phases.find((ph) => progress[ph.id].answered < progress[ph.id].total) ?? phases[0];
  const started = phases.some((ph) => progress[ph.id].answered > 0);

  return (
    <div className="mx-auto max-w-4xl px-6 py-16 sm:px-10 sm:py-24">
      <div className="flex flex-wrap items-baseline justify-between gap-3">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">the pilot · part 3 of 3</p>
        <Link href="/pilot" className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3 no-underline hover:text-accent">
          ← pilot home
        </Link>
      </div>
      <h1
        className="font-display mt-6 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        How a company hires.
      </h1>
      {wf?.intro ? <p className="mt-6 max-w-2xl text-base leading-relaxed text-ink-2">{wf.intro}</p> : null}

      {!wf ? (
        <p className="mt-10 border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          The workflow could not be read. Try again in a moment.
        </p>
      ) : (
        <>
          <section className="mt-8 max-w-2xl border-l-2 border-accent pl-5">
            <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">how this part works</p>
            <p className="mt-2 text-sm leading-relaxed text-ink-2">
              Five stages, one screen each. Under every step, one click: it happens like this, it happens differently, or it
              does not happen where you work. A box opens only where something differs. At each decision you are asked
              what the outcomes are where you work. About ten minutes.
            </p>
          </section>

          <section className="mt-12">
            <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">the five stages</p>
            <FlowStrip label="The five stages" items={phases.map((ph) => ({ id: ph.id, label: ph.title ?? ph.id }))} />
            <ol className="mt-5 space-y-2">
              {phases.map((ph) => (
                <li key={ph.id} className="text-sm leading-relaxed">
                  <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">{ph.id}</span>{" "}
                  <span className="font-medium text-ink">{ph.title}.</span> <span className="text-ink-2">{ph.purpose}</span>
                </li>
              ))}
            </ol>
            {wf.overview_question ? (
              <div className="mt-6 max-w-2xl">
                <p className="text-base leading-relaxed text-ink">{wf.overview_question}</p>
                <CommentBox
                  targetKind="workflow_general"
                  targetId="overview"
                  initialBody={comments.get("workflow_general:overview")?.body ?? ""}
                  label="your answer"
                />
              </div>
            ) : null}
          </section>

          <section className="mt-12 border-t border-line pt-8">
            <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">the stages, one screen each</p>
            <PhaseStrip phases={phases} progress={progress} />
            {next ? (
              <div className="mt-6 flex flex-wrap items-center gap-4">
                <Link
                  href={`/pilot/workflow/${next.id}`}
                  className="inline-flex items-center justify-center rounded-md bg-accent px-6 py-3 text-sm font-medium text-white no-underline shadow-sm transition-colors hover:bg-accent-hover"
                >
                  {started ? `Continue with stage ${next.id}` : `Start with stage ${next.id}`}: {next.title}
                </Link>
                <p className="text-xs text-ink-3">Every stage is a link above. The last stage ends this part.</p>
              </div>
            ) : null}
          </section>

          <p className="mt-12 text-xs text-ink-4">
            The same 26 steps as one chart, for anyone who wants the whole picture at once:{" "}
            <a href="/pilot/company-workflow.svg" target="_blank" rel="noopener noreferrer" className="text-ink-3 underline hover:text-accent">
              open the full chart in a new tab
            </a>
            .
          </p>
        </>
      )}
    </div>
  );
}
