import { describe, expect, it } from "vitest";

import { recallLabel, recallState } from "./recall-state";

// The case that made this necessary, pinned first.
//
// Run 37fdd9aa: the participant closed the tab after the first scored
// question, never saw a recall screen, and the page reported
// "933 EXPIRED, held 0/3, 0 correct". The server logs showed no
// SubmitRecall in the session at all.

describe("recallState", () => {
  it("calls an untouched default what it is, not a failure", () => {
    // Written by DTRecordBlockDigits when the block was served: outcome
    // 'expired', no latency, because nobody submitted anything.
    expect(recallState("expired", -1)).toEqual({ kind: "not-reached" });
    expect(recallState("expired", undefined)).toEqual({ kind: "not-reached" });
    expect(recallLabel(recallState("expired", -1))).toBe("not reached");
  });

  it("keeps a real expiry a real expiry", () => {
    // Somebody sat on the recall screen and ran out of clock. That has
    // a latency, and it is a finding rather than an absence.
    const s = recallState("expired", 20000);
    expect(s).toEqual({ kind: "attempted", outcome: "expired" });
    expect(recallLabel(s)).toBe("expired");
  });

  it("treats any non-expired outcome as attempted, whatever the latency", () => {
    // Only 'expired' can be the default, so nothing else needs the
    // latency to prove it was submitted.
    for (const o of ["exact", "partial", "wrong_digits", "untransformed"]) {
      expect(recallState(o, -1).kind).toBe("attempted");
      expect(recallState(o, 4200).kind).toBe("attempted");
    }
  });

  it("keeps untransformed legible, since it is the one worth spotting", () => {
    // Storage held and the executive step failed.
    expect(recallLabel(recallState("untransformed", 5000))).toBe(
      "untransformed",
    );
  });

  it("says nothing when there is no recall row at all", () => {
    expect(recallState(undefined, undefined)).toEqual({ kind: "none" });
    expect(recallState("", 0)).toEqual({ kind: "none" });
    expect(recallLabel({ kind: "none" })).toBe("");
  });

  it("does not mistake a zero latency for an absent one", () => {
    // Zero is implausible but it is a submission, and the sentinel for
    // absent is -1. Reading 0 as "never reached" would be the same
    // mistake one layer along.
    expect(recallState("expired", 0).kind).toBe("attempted");
  });
});
