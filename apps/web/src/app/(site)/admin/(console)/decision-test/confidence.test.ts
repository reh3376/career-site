import { describe, expect, it } from "vitest";

import { isRated, unratedCount, unratedNote } from "./confidence";

// Zero is a rating. Absent is not. Everything here defends that line.

describe("isRated", () => {
  it("treats a rating of zero as a rating", () => {
    expect(isRated({ outcome: "correct", confidence: 0 })).toBe(true);
  });

  it("treats a missing rating as absent", () => {
    expect(isRated({ outcome: "correct" })).toBe(false);
    expect(isRated({ outcome: "correct", confidence: -1 })).toBe(false);
  });

  it("treats an expired answer as unrated, since it was never asked", () => {
    expect(isRated({ outcome: "expired" })).toBe(false);
    expect(isRated({ outcome: "expired", confidence: 70 })).toBe(false);
  });

  it("accepts the ordinary case", () => {
    expect(isRated({ outcome: "lure", confidence: 90 })).toBe(true);
  });
});

describe("unratedCount", () => {
  it("counts only answers that were given but not rated", () => {
    // The shape of run c7cbd814: one correct answer at the clock limit
    // with no rating, three expired, the rest rated.
    const answers = [
      { outcome: "correct", confidence: 90 },
      { outcome: "expired" },
      { outcome: "other", confidence: 0 },
      { outcome: "correct", confidence: -1 },
      { outcome: "expired" },
      { outcome: "lure", confidence: 70 },
      { outcome: "expired" },
    ];
    expect(unratedCount(answers)).toBe(1);
  });

  it("does not count expired answers, which were never rated by design", () => {
    expect(unratedCount([{ outcome: "expired" }, { outcome: "expired" }])).toBe(0);
  });

  it("is zero for a fully rated run", () => {
    expect(
      unratedCount([
        { outcome: "correct", confidence: 50 },
        { outcome: "other", confidence: 0 },
      ]),
    ).toBe(0);
  });
});

describe("unratedNote", () => {
  it("says nothing when every answer was rated", () => {
    expect(unratedNote(0, 23)).toBeNull();
  });

  it("agrees with itself in number", () => {
    expect(unratedNote(1, 27)).toContain("1 of 27 answer carries");
    expect(unratedNote(4, 23)).toContain("4 of 23 answers carry");
  });

  it("explains which figures the excluded rows do and do not affect", () => {
    const n = unratedNote(2, 20)!;
    expect(n).toContain("count towards accuracy");
    expect(n).toContain("left out of every confidence figure");
  });
});
