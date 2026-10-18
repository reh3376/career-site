// The worked example shown before a transformation block.
//
// Why it is not computed from the block's own digits
// --------------------------------------------------
// It used to be. The memorise screen said "Add 3 to every digit before
// you give it back", showed 7027, and underneath it said "So you will
// type 0350" — the answer, on screen, for the whole six seconds the
// participant was supposed to be holding the number and working it out.
//
// That removed the thing those blocks exist to measure. The ramp calls
// them "the steepest step" because they move the task from storage to
// storage plus executive processing; with the result displayed there
// was no processing left, only a different number to store. It shipped
// on 2026-10-04 and was live for every run in the usable sample.
//
// So the example is now worked on digits the participant will never be
// asked to reproduce, and it is shown *before* the number appears,
// where teaching belongs. The rule is explained once, demonstrated
// once, and then the screen shows only what they have to hold.
//
// Why each example contains a wrap to zero
// ----------------------------------------
// The scoring is digit-wise modulo ten with no carry: 7 plus 3 is 0,
// not 10. The instruction "add three to every digit" does not say that,
// and a participant who meets the ambiguity for the first time while
// holding four digits under a clock has to guess. Two runs show the
// guess being made: one wrapped correctly, one did not, and one
// returned a seven-digit answer to a four-digit question.
//
// Every example here therefore contains exactly that case, so the rule
// is demonstrated on the hard part rather than stated and then met for
// the first time when it counts.

export type RuleExample = { shown: string; typed: string; wrapped: string };

// Candidates per transform. Each contains a digit that lands on zero:
// a 9 for +1, a 7 for +3. More than one so a candidate that collides
// with the block's own number can be passed over.
const CANDIDATES: Record<number, string[]> = {
  1: ["4902", "3791", "6095"],
  3: ["1752", "8274", "5713"],
};

export function applyTransform(digits: string, add: number): string {
  return digits
    .split("")
    .map((d) => String((Number(d) + add) % 10))
    .join("");
}

/**
 * A worked example for `add`, avoiding `avoid` (the digits this block
 * will actually ask for).
 *
 * The collision matters more than it looks: if the example happened to
 * be the block's own number, the modal would print the answer, which is
 * the exact defect this file exists to remove.
 */
export function ruleExample(add: number, avoid = ""): RuleExample | null {
  const pool = CANDIDATES[add];
  if (!pool) return null;
  const shown = pool.find((c) => c !== avoid) ?? pool[0];
  const typed = applyTransform(shown, add);
  // The digit that demonstrates the wrap, named so the modal can point
  // at it rather than make the reader hunt for it.
  const i = shown.split("").findIndex((d) => (Number(d) + add) % 10 === 0);
  const wrapped = i >= 0 ? shown[i] : "";
  return { shown, typed, wrapped };
}
