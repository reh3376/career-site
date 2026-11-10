// The company workflow as embedded in the API (services/api/internal/
// pilot/data/workflow-company-<version>.json), typed loosely: the file
// is versioned data and a later revision may add fields without this
// code having to know. Revision 3 (2026-11-09) added the stage
// purposes, the per-step question and answers, the decision questions
// with our outcomes, the return edges and the stage and closing
// questions, all from the owner's comments after his own run
// (docs/framework-research/15 §14).
export type WorkflowPhase = { id: string; title?: string; purpose?: string };

export type WorkflowNode = {
  id: string;
  phase?: string;
  name?: string;
  owner?: string;
  summary?: string;
  decision?: boolean;
  open?: boolean;
  decision_question?: string;
  our_outcomes?: string[];
  returns_to?: string;
  returns_when?: string;
};

export type WorkflowData = {
  version?: string;
  intro?: string;
  overview_question?: string;
  step_question?: string;
  step_answers?: { value: string; label: string }[];
  differs_prompt?: string;
  decision_question?: string;
  compare_label?: string;
  phase_questions?: { id: string; text: string; phases?: string[] }[];
  missing_question?: string;
  closing?: { id: string; text: string }[];
  phases?: WorkflowPhase[];
  nodes?: WorkflowNode[];
};

export function parseWorkflow(json?: string): WorkflowData | null {
  if (!json) return null;
  try {
    return JSON.parse(json) as WorkflowData;
  } catch {
    return null;
  }
}

export const ANSWER_LABEL: Record<string, string> = {
  matches: "Happens like this",
  differs: "Happens differently",
  absent: "Does not happen here",
};

export const DEFAULT_ANSWERS = [
  { value: "matches", label: ANSWER_LABEL.matches },
  { value: "differs", label: ANSWER_LABEL.differs },
  { value: "absent", label: ANSWER_LABEL.absent },
];

// How many of each stage's steps carry an answer, for the stage strip
// and the overview's "continue" button.
export function progressByPhase(
  phases: WorkflowPhase[],
  nodes: WorkflowNode[],
  comments: Map<string, { answer?: string }>,
): Record<string, { answered: number; total: number }> {
  const out: Record<string, { answered: number; total: number }> = {};
  for (const ph of phases) out[ph.id] = { answered: 0, total: 0 };
  for (const n of nodes) {
    const p = out[n.phase ?? ""];
    if (!p) continue;
    p.total += 1;
    if (comments.get(`workflow_node:${n.id}`)?.answer) p.answered += 1;
  }
  return out;
}
