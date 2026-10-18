import { describe, expect, it } from "vitest";

import { applyTransform, ruleExample } from "./rule-example";

// The example exists to teach the rule without giving away the answer.
// Both halves of that are defended here, because the version this
// replaces failed the second half and was live for the whole sample.

describe("applyTransform", () => {
  it("adds per digit, modulo ten, with no carry", () => {
    expect(applyTransform("7027", 3)).toBe("0350");
    expect(applyTransform("7113", 1)).toBe("8224");
    expect(applyTransform("6777", 3)).toBe("9000");
    expect(applyTransform("7956", 1)).toBe("8067");
  });

  it("never carries into the digit on the left", () => {
    // 9+3 is 2 in this scheme, not 12, and the 1 does not move left.
    expect(applyTransform("9999", 3)).toBe("2222");
    expect(applyTransform("1999", 1)).toBe("2000");
  });

  it("returns as many digits as it was given", () => {
    for (const d of ["793", "3338", "99999"]) {
      expect(applyTransform(d, 3)).toHaveLength(d.length);
    }
  });
});

describe("ruleExample", () => {
  it("demonstrates the wrap, which is the part the rule never stated", () => {
    for (const add of [1, 3]) {
      const e = ruleExample(add)!;
      expect(e.wrapped).not.toBe("");
      // The named digit really is the one that lands on zero.
      expect((Number(e.wrapped) + add) % 10).toBe(0);
      expect(e.typed).toContain("0");
    }
  });

  it("works its own arithmetic correctly", () => {
    for (const add of [1, 3]) {
      const e = ruleExample(add)!;
      expect(e.typed).toBe(applyTransform(e.shown, add));
    }
  });

  it("never shows the block's own number", () => {
    // The whole defect in one line: if the example were the block's
    // digits, the modal would print the answer the participant is
    // about to be asked for.
    for (const add of [1, 3]) {
      const first = ruleExample(add)!.shown;
      const avoided = ruleExample(add, first)!;
      expect(avoided.shown).not.toBe(first);
      expect(avoided.typed).toBe(applyTransform(avoided.shown, add));
      expect(avoided.wrapped).not.toBe("");
    }
  });

  it("gives a four digit example, matching the blocks that transform", () => {
    for (const add of [1, 3]) {
      expect(ruleExample(add)!.shown).toHaveLength(4);
    }
  });

  it("has nothing to say about a block that does not transform", () => {
    expect(ruleExample(0)).toBeNull();
    expect(ruleExample(5)).toBeNull();
  });
});
