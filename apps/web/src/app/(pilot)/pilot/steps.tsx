"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

export type StepProgress = {
  started: boolean;
  surveyDone: boolean;
  surveyHref: string;
  recordDone: boolean;
  workflowDone: boolean;
  finished: boolean;
};

type State = "done" | "current" | "next" | "locked";

// The four steps, in order, with one of four states each: done,
// current (the page open now), next (open, not done, not here), or
// locked (not yet reachable). Done and open steps are links; locked
// ones are not, which is how the order is kept without a wall of
// explanation.
export function PilotSteps({ progress: p }: { progress: StepProgress }) {
  const path = usePathname() ?? "";
  const on = (prefix: string) => path === prefix || path.startsWith(prefix + "/");
  const allRead = p.surveyDone && p.recordDone && p.workflowDone;
  const steps: { n: number; label: string; href: string; done: boolean; open: boolean; here: boolean }[] = [
    { n: 1, label: "Survey", href: p.surveyHref, done: p.surveyDone, open: p.started, here: on("/pilot/survey") },
    { n: 2, label: "Match record", href: "/pilot/record", done: p.recordDone, open: p.surveyDone, here: on("/pilot/record") },
    { n: 3, label: "Workflow", href: "/pilot/workflow", done: p.workflowDone, open: p.recordDone, here: on("/pilot/workflow") },
    { n: 4, label: "Finish", href: p.finished ? "/pilot/done" : "/pilot", done: p.finished, open: allRead, here: on("/pilot/done") },
  ];
  // The one open step that is not done is "next" unless we are on it.
  const nextN = steps.find((s) => s.open && !s.done)?.n;
  return (
    <ol className="flex flex-wrap items-center gap-x-2 gap-y-2" aria-label="Pilot steps">
      {steps.map((s, i) => {
        const state: State = s.here ? "current" : s.done ? "done" : s.open && s.n === nextN ? "next" : s.open ? "next" : "locked";
        const tone =
          state === "current"
            ? "border-accent bg-accent text-white"
            : state === "done"
              ? "border-success text-success"
              : state === "next"
                ? "border-accent text-accent"
                : "border-line text-ink-4";
        const inner = (
          <span className={`inline-flex items-center gap-2 border px-3 py-1 font-mono text-[11px] uppercase tracking-[0.14em] ${tone}`}>
            <span>{state === "done" ? "✓" : s.n}</span>
            <span>{s.label}</span>
            {state === "next" && s.n === nextN ? <span className="opacity-70">· next</span> : null}
          </span>
        );
        return (
          <li key={s.n} className="flex items-center gap-2">
            {state === "locked" ? inner : <Link href={s.href} className="no-underline">{inner}</Link>}
            {i < steps.length - 1 ? <span className="text-ink-4">→</span> : null}
          </li>
        );
      })}
    </ol>
  );
}
