// Did the participant try the recall, or never get to it?
//
// `dt_recalls` rows are written when a block is *served*, by
// DTRecordBlockDigits, with `outcome = 'expired'` as the starting
// state, so the server can grade a recall later without trusting the
// client to say what it asked somebody to hold. Submitting a recall
// updates that row.
//
// The consequence is that "expired" is the default rather than a
// finding. Every block an abandoned run never reached shows a recall
// marked expired, with a presented number and `held 0/3`, which reads
// exactly like a participant who stared at the box and ran out of time.
//
// Run 37fdd9aa is the case that made this worth fixing. The server logs
// show three requests in the whole session and no SubmitRecall at all:
// the participant closed the tab after the first scored question. The
// page said "933 EXPIRED, held 0/3, 0 correct", and reading it cold
// gives a completely wrong account of what happened.
//
// The data already separates them. A recall somebody actually attempted
// has a latency; one that was never reached has none. That is carried
// as -1, which is the sentinel the sibling columns in the same
// projection already use for absent.

export type RecallState =
  | { kind: "attempted"; outcome: string }
  | { kind: "not-reached" }
  | { kind: "none" };

const ABSENT = -1;

export function recallState(
  outcome: string | undefined,
  latencyMs: number | undefined,
): RecallState {
  if (!outcome) return { kind: "none" };
  // Only an expired recall can be the untouched default. Any other
  // outcome was written by a submission, whatever its latency.
  if (outcome === "expired" && (latencyMs ?? ABSENT) < 0) {
    return { kind: "not-reached" };
  }
  return { kind: "attempted", outcome };
}

export function recallLabel(s: RecallState): string {
  switch (s.kind) {
    case "none":
      return "";
    case "not-reached":
      return "not reached";
    case "attempted":
      return s.outcome.replace("_", " ");
  }
}
