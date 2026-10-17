// How a figure moved from the early blocks to the hardest one.
//
// This sentence is the instrument's headline, and it was inverted.
//
// The page shows "Early blocks" and then "Hardest block", so a reader
// takes "accuracy moved N points" as the change across that span,
// early to hardest. It printed `early - hardest`. For the result the
// test is built to find, accuracy falling as load rises, that is a
// positive number: a participant whose accuracy fell from 60% to 35%
// was reported as having "moved 25 points", which reads as improvement.
// Every run with the expected pattern was described backwards, and a
// run like f9753e2f, where accuracy genuinely rose, was described as a
// fall.
//
// So the direction is computed in the obvious order and named rather
// than signed. "Fell 25 points" cannot be misread; "-25" beside the
// word "moved" can, and was.

export type Move = { verb: "fell" | "rose" | "held"; points: number };

export function describeMove(early: number, hardest: number): Move {
  const d = hardest - early;
  if (d === 0) return { verb: "held", points: 0 };
  return { verb: d > 0 ? "rose" : "fell", points: Math.abs(d) };
}

export function moveText(m: Move): string {
  return m.verb === "held" ? "held steady" : `${m.verb} ${m.points} points`;
}
