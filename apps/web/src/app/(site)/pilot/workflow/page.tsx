import { readFile } from "node:fs/promises";
import path from "node:path";

import type { Metadata } from "next";
import Link from "next/link";

import { markPartAction } from "../actions";
import { CommentBox } from "../comment-box";
import { requireSignedIn, unwrap } from "../gate";
import { getPilotWorkflow, type PilotComment } from "../lib";

import { ChartFrame } from "./chart-frame";
import { MissingSteps } from "./missing-steps";

export const metadata: Metadata = { title: "Pilot · How a company hires" };
export const dynamic = "force-dynamic";

// The company hiring workflow as a diagram and as a node list with a
// comment box per node and a repeatable "a step that is missing" field
// (FSD FR-PLT-09). The diagram is an SVG committed to the repository,
// inlined here because it carries foreignObject labels that an <img>
// would drop; no third-party script is loaded. The candidate
// perspective is not shown.
async function readSvg(): Promise<string | null> {
  try {
    const p = path.join(process.cwd(), "public", "pilot", "company-workflow.svg");
    const s = await readFile(p, "utf8");
    // Drop any XML prologue; the element goes straight into the DOM.
    return s.replace(/^<\?xml[^>]*>\s*/, "");
  } catch {
    return null;
  }
}

export default async function PilotWorkflowPage() {
  await requireSignedIn("/pilot/workflow");
  const [data, svg] = await Promise.all([getPilotWorkflow().then(unwrap), readSvg()]);
  const comments = new Map<string, PilotComment>();
  for (const c of data.comments ?? []) comments.set(`${c.targetKind}:${c.targetId ?? ""}`, c);
  const missing = (data.comments ?? [])
    .filter((c) => c.targetKind === "workflow_missing")
    .map((c) => ({ id: c.targetId ?? "", body: c.body }))
    .filter((c) => c.id !== "")
    .sort((a, b) => Number(a.id) - Number(b.id));
  const phases = data.phases ?? [];
  const nodes = data.nodes ?? [];
  const markWorkflowDone = markPartAction.bind(null, "workflow");

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
      {data.intro ? <p className="mt-6 max-w-2xl text-base leading-relaxed text-ink-2">{data.intro}</p> : null}

      {data.prompts?.length ? (
        <section className="mt-8 max-w-2xl border-l-2 border-accent pl-5">
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">what helps most</p>
          <ol className="mt-2 list-decimal space-y-1 pl-5 text-sm leading-relaxed text-ink-2">
            {data.prompts.map((p) => (
              <li key={p}>{p}</li>
            ))}
          </ol>
        </section>
      ) : null}

      <section className="mt-12">
        <div className="flex flex-wrap items-baseline justify-between gap-3">
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">the chart · C-01 to C-26</p>
          <a
            href="/pilot/company-workflow.svg"
            target="_blank"
            rel="noopener noreferrer"
            className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent no-underline hover:underline"
          >
            open full size in a new tab ↗
          </a>
        </div>
        {svg ? (
          <ChartFrame svg={svg} />
        ) : (
          <p className="mt-3 border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
            The chart file is missing from this build. The node list below is the same content.
          </p>
        )}
      </section>

      <section className="mt-14">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">the steps, one by one</p>
        <p className="mt-2 max-w-2xl text-sm text-ink-2">
          Under each step, say whether it matches how a hire runs where you work. Leave the box empty where it does.
        </p>
        {phases.map((ph) => {
          const own = nodes.filter((n) => n.phase === ph.id);
          if (own.length === 0) return null;
          return (
            <div key={ph.id} className="mt-10">
              <h2
                className="font-display text-2xl leading-tight text-ink"
                style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
              >
                <span className="mr-3 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">phase {ph.id}</span>
                {ph.title}
              </h2>
              <ol className="mt-4 divide-y divide-line border-y border-line">
                {own.map((n) => (
                  <li key={n.id} className="py-6">
                    <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                      <span className="text-ink">{n.id}</span>
                      {n.decision ? <span className="text-accent">decision</span> : null}
                      {n.open ? <span className="text-signal">open · whether this step exists is one of the pilot&rsquo;s questions</span> : null}
                    </div>
                    <p className="mt-2 text-base leading-relaxed text-ink">{n.name}</p>
                    {n.owner ? (
                      <p className="mt-1 text-xs text-ink-3">
                        <span className="font-mono uppercase tracking-[0.14em]">owner</span> · {n.owner}
                      </p>
                    ) : null}
                    {n.summary ? <p className="mt-2 max-w-2xl text-sm leading-relaxed text-ink-2">{n.summary}</p> : null}
                    <CommentBox
                      targetKind="workflow_node"
                      targetId={n.id}
                      initialBody={comments.get(`workflow_node:${n.id}`)?.body ?? ""}
                      label={`your comment on ${n.id}`}
                      placeholder="Does this match? Who owns it where you work? What is different?"
                    />
                  </li>
                ))}
              </ol>
            </div>
          );
        })}
      </section>

      <section className="mt-14 border-t border-line pt-8">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">a step that is missing</p>
        <p className="mt-2 max-w-2xl text-sm text-ink-2">
          Something that happens where you work and is not on the chart. One per box; another box appears when you save.
        </p>
        <MissingSteps initial={missing} />
      </section>

      <section className="mt-14 border-t border-line pt-8">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">the chart as a whole</p>
        <p className="mt-3 max-w-2xl text-base leading-relaxed text-ink">
          Which two steps are really one? Is there anything here you would not want a candidate to see?
        </p>
        <CommentBox
          targetKind="workflow_general"
          targetId=""
          initialBody={comments.get("workflow_general:")?.body ?? ""}
          label="your general comment"
        />
      </section>

      <form action={markWorkflowDone} className="mt-12 flex flex-wrap items-center gap-4 border-t border-line pt-8">
        <button
          type="submit"
          className="inline-flex items-center justify-center rounded-md bg-accent px-6 py-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-accent-hover"
        >
          I have read the workflow
        </button>
        <p className="text-xs text-ink-3">Marks this part done. You can still come back and change your comments.</p>
      </form>
    </div>
  );
}
