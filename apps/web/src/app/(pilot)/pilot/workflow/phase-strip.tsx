import Link from "next/link";

import type { WorkflowPhase } from "./types";

// The five stages as chips, the current one filled, each with how many
// of its steps have an answer. Every stage is a link: the order is a
// suggestion, not a lock, because a participant who knows one stage
// well may want to start there.
export function PhaseStrip({
  phases,
  current,
  progress,
}: {
  phases: WorkflowPhase[];
  current?: string;
  progress: Record<string, { answered: number; total: number }>;
}) {
  return (
    <ol className="mt-6 flex flex-wrap items-center gap-2" aria-label="Stages">
      {phases.map((ph) => {
        const p = progress[ph.id] ?? { answered: 0, total: 0 };
        const done = p.total > 0 && p.answered >= p.total;
        const here = ph.id === current;
        const tone = here ? "border-accent bg-accent text-white" : done ? "border-success text-success" : "border-line text-ink-2";
        return (
          <li key={ph.id}>
            <Link
              href={`/pilot/workflow/${ph.id}`}
              className={`inline-flex items-center gap-2 border px-3 py-1 font-mono text-[11px] uppercase tracking-[0.14em] no-underline ${tone}`}
              aria-current={here ? "page" : undefined}
            >
              <span>{done && !here ? "✓" : ph.id}</span>
              <span>{ph.title}</span>
              {p.total > 0 ? <span className="opacity-70">{p.answered}/{p.total}</span> : null}
            </Link>
          </li>
        );
      })}
    </ol>
  );
}
