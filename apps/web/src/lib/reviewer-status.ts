import "server-only";

import { callApi } from "@/lib/api-fetch";

// How the reviewer is currently measuring, for the public page.
//
// Read from the same views /admin/gate reads, so the page cannot quote
// a number the owner is not also looking at. A failure returns
// undefined and the page simply says less; a claim about honesty that
// falls back to a stale hard-coded figure would be self-defeating.

export type ReviewerStatus = {
  graded: number;
  agreed: number;
  agreementPct: number;
  hardDisagreements: number;
  tooHarsh: number;
  tooGenerous: number;
  postings: number;
  postingsRandom: number;
  scored: number;
  gateCorrect: number;
  inversions: number;
  margin?: number;
  evaluatedAt?: string;
  model: string;
  comparison: ComparisonRow[];
};

// One posting scored in the two most recent completed evaluations.
export type ComparisonRow = {
  label: string;
  score: number;
  previous?: number;
  unchanged: boolean;
};

type Raw = Partial<Record<keyof ReviewerStatus, unknown>>;

type RawRow = {
  label?: unknown;
  score?: unknown;
  previous?: unknown;
  unchanged?: unknown;
};

// proto3 JSON omits zero values, so a score of exactly 0 arrives as an
// absent field and a row that did not move arrives without `unchanged`
// only when it is false. Both default correctly here.
function readComparison(raw: unknown): ComparisonRow[] {
  if (!Array.isArray(raw)) return [];
  const out: ComparisonRow[] = [];
  for (const r of raw as RawRow[]) {
    if (typeof r?.label !== "string") continue;
    out.push({
      label: r.label,
      score: typeof r.score === "number" ? r.score : 0,
      previous: typeof r.previous === "number" ? r.previous : undefined,
      unchanged: r.unchanged === true,
    });
  }
  return out;
}

function num(v: unknown): number {
  return typeof v === "number" ? v : 0;
}

export async function getReviewerStatus(): Promise<ReviewerStatus | undefined> {
  try {
    const resp = await callApi({
      path: "/api/career.v1.SystemService/GetReviewerStatus",
      body: {},
    });
    if (!resp.ok) return undefined;
    const j = (await resp.json()) as Raw;
    const status: ReviewerStatus = {
      graded: num(j.graded),
      agreed: num(j.agreed),
      agreementPct: num(j.agreementPct),
      hardDisagreements: num(j.hardDisagreements),
      tooHarsh: num(j.tooHarsh),
      tooGenerous: num(j.tooGenerous),
      postings: num(j.postings),
      postingsRandom: num(j.postingsRandom),
      scored: num(j.scored),
      gateCorrect: num(j.gateCorrect),
      inversions: num(j.inversions),
      margin: typeof j.margin === "number" ? j.margin : undefined,
      evaluatedAt: typeof j.evaluatedAt === "string" ? j.evaluatedAt : undefined,
      model: typeof j.model === "string" ? j.model : "",
      comparison: readComparison(j.comparison),
    };
    // Nothing graded and nothing scored means the page has nothing
    // true to say yet, which is different from the call failing.
    if (status.graded === 0 && status.scored === 0) return undefined;
    return status;
  } catch {
    return undefined;
  }
}
