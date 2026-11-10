import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { markPartAction } from "../../actions";
import { CommentBox } from "../../comment-box";
import { requireSignedIn, unwrap } from "../../gate";
import { getPilotWorkflow, type PilotComment } from "../../lib";
import { FlowStrip } from "../flow-strip";
import { MissingSteps } from "../missing-steps";
import { PhaseStrip } from "../phase-strip";
import { StepAnswer } from "../step-answer";
import { DEFAULT_ANSWERS, parseWorkflow, progressByPhase } from "../types";

export const metadata: Metadata = { title: "Pilot · How a company hires" };
export const dynamic = "force-dynamic";

// One stage of the company hiring workflow (FSD FR-PLT-09, revision 3):
// a small diagram of the stage, its steps in plain language with a
// three-way answer each, the open question at each decision, the stage
// questions and the missing-step boxes, then the next stage. The last
// stage carries the two closing questions and the button that marks
// the part done.
export default async function PilotWorkflowPhasePage({ params }: { params: Promise<{ phase: string }> }) {
  const { phase } = await params;
  await requireSignedIn(`/pilot/workflow/${phase}`);
  const data = unwrap(await getPilotWorkflow());
  const wf = parseWorkflow(data.workflowJson);
  const phases = wf?.phases ?? [];
  const idx = phases.findIndex((p) => p.id === phase);
  if (!wf || idx < 0) notFound();
  const ph = phases[idx];
  const nextPh = phases[idx + 1];
  const last = idx === phases.length - 1;

  const comments = new Map<string, PilotComment>();
  for (const c of data.comments ?? []) comments.set(`${c.targetKind}:${c.targetId ?? ""}`, c);
  const all = wf.nodes ?? [];
  const nodes = all.filter((n) => n.phase === ph.id);
  const byId = new Map(all.map((n) => [n.id, n]));
  const progress = progressByPhase(phases, all, comments);
  const unanswered = progress[ph.id].total - progress[ph.id].answered;
  const answers = wf.step_answers?.length ? wf.step_answers : DEFAULT_ANSWERS;
  const stepQuestion = wf.step_question ?? "Where you work, this step:";
  const differsPrompt = wf.differs_prompt ?? "How does it run where you work, and who does it?";
  const notesPrompt = wf.notes_prompt ?? "Notes (optional)";
  const returns = nodes
    .filter((n) => n.returns_to)
    .map((n) => {
      const to = byId.get(n.returns_to ?? "");
      return `${n.id} can send the work back to ${n.returns_to}${to?.name ? ` (${to.name.toLowerCase()})` : ""}${n.returns_when ? ` ${n.returns_when}` : ""}.`;
    });
  const phaseQs = (wf.phase_questions ?? []).filter((q) => !q.phases || q.phases.includes(ph.id));
  const missing = (data.comments ?? [])
    .filter((c) => c.targetKind === "workflow_missing" && (c.targetId ?? "").startsWith(`${ph.id}-`))
    .map((c) => ({ id: c.targetId ?? "", body: c.body }))
    .sort((a, b) => (Number(a.id.split("-")[1]) || 0) - (Number(b.id.split("-")[1]) || 0));
  const markWorkflowDone = markPartAction.bind(null, "workflow");

  return (
    <div className="mx-auto max-w-4xl px-6 py-16 sm:px-10 sm:py-24">
      <div className="flex flex-wrap items-baseline justify-between gap-3">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
          the pilot · part 3 of 3 · stage {idx + 1} of {phases.length}
        </p>
        <Link href="/pilot/workflow" className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3 no-underline hover:text-accent">
          ← all five stages
        </Link>
      </div>
      <PhaseStrip phases={phases} current={ph.id} progress={progress} />
      <h1
        className="font-display mt-8 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        {ph.title}.
      </h1>
      {ph.purpose ? <p className="mt-4 max-w-2xl text-base leading-relaxed text-ink-2">{ph.purpose}</p> : null}

      <section className="mt-8">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">this stage, as we picture it</p>
        <FlowStrip
          label={`Stage ${ph.id} steps`}
          items={nodes.map((n) => ({ id: n.id, label: n.name ?? n.id, decision: n.decision }))}
          returns={returns}
          enters={ph.enters_with ? `${idx > 0 ? `from stage ${phases[idx - 1].id}: ` : "starts with "}${ph.enters_with}` : undefined}
          leaves={ph.leaves_with ? `${idx < phases.length - 1 ? `to stage ${phases[idx + 1].id}: ` : "ends with "}${ph.leaves_with}` : undefined}
        />
      </section>

      <section className="mt-12">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">the steps</p>
        <p className="mt-2 max-w-2xl text-sm text-ink-2">
          Under each step, one click, and a notes box for anything you want to say about it. Where it happens differently, say how.
        </p>
        <ol className="mt-4 divide-y divide-line border-y border-line">
          {nodes.map((n) => {
            const c = comments.get(`workflow_node:${n.id}`);
            return (
              <li key={n.id} className="py-7">
                <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                  <span className="text-ink">{n.id}</span>
                  {n.decision ? <span className="text-accent">decision</span> : null}
                  {n.open ? <span className="text-signal">we are not sure this step exists; you can tell us</span> : null}
                </div>
                <p className="mt-2 text-lg leading-snug text-ink">{n.name}</p>
                {n.owner ? (
                  <p className="mt-2 text-sm text-ink-2">
                    <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">who</span> · {n.owner}
                  </p>
                ) : null}
                {n.summary ? (
                  <p className="mt-2 max-w-2xl text-sm leading-relaxed text-ink-2">
                    <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">what happens</span> · {n.summary}
                  </p>
                ) : null}
                <StepAnswer
                  nodeId={n.id}
                  question={stepQuestion}
                  answers={answers}
                  differsPrompt={differsPrompt}
                  notesPrompt={notesPrompt}
                  initialAnswer={c?.answer ?? ""}
                  initialBody={c?.body ?? ""}
                />
                {n.decision ? (
                  <div className="mt-5 border-l-2 border-accent pl-4">
                    <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">at this decision</p>
                    <p className="mt-1 max-w-2xl text-sm leading-relaxed text-ink">{n.decision_question ?? wf.decision_question}</p>
                    <CommentBox
                      targetKind="workflow_outcomes"
                      targetId={n.id}
                      initialBody={comments.get(`workflow_outcomes:${n.id}`)?.body ?? ""}
                      label="your answer"
                      placeholder="One outcome per line"
                    />
                    {n.our_outcomes?.length ? (
                      <details className="mt-3">
                        <summary className="cursor-pointer text-sm text-ink-3 hover:text-accent">{wf.compare_label ?? "Compare with our picture"}</summary>
                        <ul className="mt-2 list-disc space-y-1 pl-5 text-sm text-ink-2">
                          {n.our_outcomes.map((o) => (
                            <li key={o}>{o}</li>
                          ))}
                        </ul>
                      </details>
                    ) : null}
                  </div>
                ) : null}
              </li>
            );
          })}
        </ol>
      </section>

      <section className="mt-12 border-t border-line pt-8">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">about this stage</p>
        {phaseQs.map((q) => (
          <div key={q.id} className="mt-5 max-w-2xl">
            <p className="text-base leading-relaxed text-ink">{q.text}</p>
            <CommentBox
              targetKind="workflow_phase"
              targetId={`${ph.id}-${q.id}`}
              initialBody={comments.get(`workflow_phase:${ph.id}-${q.id}`)?.body ?? ""}
              label="your answer"
            />
          </div>
        ))}
        <div className="mt-6 max-w-2xl">
          <p className="text-base leading-relaxed text-ink">
            {wf.missing_question ?? "Is there a step in this stage that happens where you work and is not here? One per box."}
          </p>
          <MissingSteps phase={ph.id} initial={missing} />
        </div>
      </section>

      {last ? (
        <>
          {wf.closing?.length ? (
            <section className="mt-12 border-t border-line pt-8">
              <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">two last questions, about the whole picture</p>
              {wf.closing.map((q) => (
                <div key={q.id} className="mt-5 max-w-2xl">
                  <p className="text-base leading-relaxed text-ink">{q.text}</p>
                  <CommentBox
                    targetKind="workflow_general"
                    targetId={q.id}
                    initialBody={comments.get(`workflow_general:${q.id}`)?.body ?? ""}
                    label="your answer"
                  />
                </div>
              ))}
            </section>
          ) : null}
          <form action={markWorkflowDone} className="mt-12 flex flex-wrap items-center gap-4 border-t border-line pt-8">
            <button
              type="submit"
              className="inline-flex items-center justify-center rounded-md bg-accent px-6 py-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-accent-hover"
            >
              I have read the workflow
            </button>
            <p className="text-xs text-ink-3">
              Marks this part done and takes you back to the pilot home to finish. You can still come back to any stage and
              change your answers.
            </p>
          </form>
        </>
      ) : (
        <div className="mt-12 flex flex-wrap items-center gap-4 border-t border-line pt-8">
          <Link
            href={`/pilot/workflow/${nextPh.id}`}
            className="inline-flex items-center justify-center rounded-md bg-accent px-6 py-3 text-sm font-medium text-white no-underline shadow-sm transition-colors hover:bg-accent-hover"
          >
            Continue to stage {nextPh.id}: {nextPh.title}
          </Link>
          <p className="text-xs text-ink-3">
            {unanswered > 0
              ? `${unanswered} ${unanswered === 1 ? "step" : "steps"} on this stage ${unanswered === 1 ? "has" : "have"} no answer yet. Every answer saves on the click, and you can come back.`
              : "Every answer on this stage is saved."}
          </p>
        </div>
      )}
    </div>
  );
}
