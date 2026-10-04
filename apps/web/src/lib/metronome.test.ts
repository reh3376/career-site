import { describe, expect, it } from "vitest";

import { scoreTapCheck } from "./metronome";

// The tap check is the only thing standing between a muted laptop and
// fifteen minutes of silent data. It has to reject two different
// failures, and a hit rate alone cannot tell them apart.

const ticks = Array.from({ length: 10 }, (_, i) => i * 1.0);

describe("scoreTapCheck", () => {
  it("passes somebody tracking the beat with a steady lag", () => {
    // A real person lags the beat by a consistent 120 ms or so.
    const taps = ticks.map((t) => t + 0.12);
    const r = scoreTapCheck(ticks, taps);
    expect(r.passed).toBe(true);
    expect(r.hits).toBe(10);
    expect(r.sdMs).toBeLessThan(20);
  });

  it("passes somebody who misses a couple", () => {
    const taps = ticks.slice(0, 8).map((t) => t + 0.1);
    expect(scoreTapCheck(ticks, taps).passed).toBe(true);
  });

  it("fails silence, which is the muted device it exists to catch", () => {
    expect(scoreTapCheck(ticks, []).passed).toBe(false);
  });

  it("fails somebody guessing at the wrong rate", () => {
    // Tapping twice a second: plenty land near a tick by luck.
    const taps = Array.from({ length: 20 }, (_, i) => i * 0.5);
    const r = scoreTapCheck(ticks, taps);
    expect(r.passed).toBe(false);
  });

  it("fails mashing even when the hit rate looks fine", () => {
    // Thirty taps over ten seconds: many land inside the window, so a
    // hit-rate-only check would pass this. The tap count is what
    // catches it.
    const taps = Array.from({ length: 30 }, (_, i) => i * 0.33);
    const r = scoreTapCheck(ticks, taps);
    expect(r.hits).toBeGreaterThan(5);
    expect(r.passed).toBe(false);
  });

  it("fails erratic timing even at a plausible rate", () => {
    // Ten taps, each near a tick, but with a lag that wanders. Somebody
    // tracking a metronome does not wander.
    const wobble = [0.02, 0.28, 0.05, 0.26, 0.01, 0.29, 0.04, 0.27, 0.03, 0.25];
    const taps = ticks.map((t, i) => t + wobble[i]);
    const r = scoreTapCheck(ticks, taps);
    expect(r.hits).toBe(10);
    expect(r.sdMs).toBeGreaterThan(100);
    expect(r.passed).toBe(false);
  });

  it("reports the offset and spread for calibration", () => {
    // These numbers are what the pilot will set the thresholds from, so
    // they have to be reported rather than only used internally.
    const taps = ticks.map((t) => t + 0.15);
    const r = scoreTapCheck(ticks, taps);
    expect(r.meanOffsetMs).toBeGreaterThan(140);
    expect(r.meanOffsetMs).toBeLessThan(160);
  });
});
