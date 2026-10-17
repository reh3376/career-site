import { describe, expect, it } from "vitest";

import { describeMove, moveText } from "./move";

// The case that was wrong in production, pinned first.
//
// The instrument exists to find accuracy falling while confidence does
// not. That is the common case, and it is the one the old arithmetic
// reported backwards, so it is the one the test leads with.

describe("describeMove", () => {
  it("calls a drop under load a fall", () => {
    // The expected finding: 60% correct early, 35% in the hardest block.
    expect(describeMove(60, 35)).toEqual({ verb: "fell", points: 25 });
    expect(moveText(describeMove(60, 35))).toBe("fell 25 points");
  });

  it("calls a rise a rise", () => {
    // Run f9753e2f: 8% correct early, 17% in the hardest block, with
    // confidence rising from 54% to 70%. The page called both of these
    // falls.
    expect(moveText(describeMove(8, 17))).toBe("rose 9 points");
    expect(moveText(describeMove(54, 70))).toBe("rose 16 points");
  });

  it("does not invent a direction when nothing moved", () => {
    expect(describeMove(42, 42)).toEqual({ verb: "held", points: 0 });
    expect(moveText(describeMove(42, 42))).toBe("held steady");
  });

  it("reports magnitude without a sign, so the verb carries direction", () => {
    expect(describeMove(10, 90).points).toBeGreaterThan(0);
    expect(describeMove(90, 10).points).toBeGreaterThan(0);
    expect(describeMove(90, 10).verb).toBe("fell");
    expect(describeMove(10, 90).verb).toBe("rose");
  });

  it("is antisymmetric: swapping the ends swaps the verb, not the size", () => {
    for (const [a, b] of [
      [0, 100],
      [37, 12],
      [55, 56],
    ]) {
      const f = describeMove(a, b);
      const r = describeMove(b, a);
      expect(f.points).toBe(r.points);
      expect(f.verb).not.toBe(r.verb);
    }
  });
});
