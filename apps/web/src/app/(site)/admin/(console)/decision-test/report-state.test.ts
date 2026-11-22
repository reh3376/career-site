import { describe, expect, it } from "vitest";

import { noRanking, ordinal, readReport, when } from "./report-state";

describe("readReport", () => {
  it("says there is none before one is built", () => {
    for (const r of [undefined, {}, { state: "none" }]) {
      const got = readReport(r);
      expect(got.ready).toBe(false);
      expect(got.headline).toBe("No report yet");
      expect(got.canSend).toBe(false);
    }
  });

  it("states a ranking only for a report that carries one", () => {
    const ranked = readReport({
      state: "ready",
      version: 2,
      generatedAt: "2026-11-20T14:03:09Z",
      compared: true,
      others: 15,
      score: 12,
      total: 30,
      rank: 5,
      groupSize: 16,
      percentile: 71.875,
      participantPages: 7,
      ownerPages: 10,
      canSend: true,
    });
    expect(ranked.headline).toBe("Version 2, built 2026-11-20 14:03 UTC");
    expect(ranked.lines[0]).toBe(
      "12 of 30 correct. Ranked 5th of 16 first-time participants, 72nd percentile.",
    );
    expect(ranked.lines).toContain("Not emailed to the participant.");
    expect(ranked.canSend).toBe(true);

    const alone = readReport({
      state: "ready",
      compared: false,
      noRankingReason: "small_group",
      others: 4,
      score: 12,
      total: 30,
      // Figures a careless server might still send. They must not be read.
      rank: 0,
      groupSize: 0,
    });
    expect(alone.lines[0]).toContain("No ranking");
    expect(alone.lines[0]).toContain("there are 4.");
    expect(alone.lines[0]).not.toContain("Ranked");
    expect(alone.lines[0]).not.toContain("percentile");
  });

  it("names who sent it and refuses a send nobody asked for", () => {
    const auto = readReport({ state: "ready", emailedAt: "2026-11-19T03:09:00Z", emailedBy: "system" });
    expect(auto.lines.join(" ")).toContain(
      "Last emailed to the participant 2026-11-19 03:09 UTC, automatically when the run finished.",
    );
    const manual = readReport({ state: "ready", emailedAt: "2026-11-21T10:00:00Z", emailedBy: "Roger" });
    expect(manual.lines.join(" ")).toContain("by Roger.");

    const unasked = readReport({ state: "ready", canSend: false });
    expect(unasked.canSend).toBe(false);
    expect(unasked.sendNote).toContain("did not ask");
  });

  it("shows what went wrong when a build failed", () => {
    const got = readReport({ state: "failed", error: "render: unavailable" });
    expect(got.ready).toBe(false);
    expect(got.lines[0]).toBe("Last error: render: unavailable");
    // A rebuild that failed over a standing report keeps the report.
    const kept = readReport({ state: "ready", version: 1, error: "render: unavailable" });
    expect(kept.ready).toBe(true);
    expect(kept.lines.join(" ")).toContain("failed: render: unavailable");
  });

  it("uses no em dash anywhere it can print", () => {
    const all = [
      readReport(undefined),
      readReport({ state: "failed" }),
      readReport({ state: "ready", compared: true, rank: 1, groupSize: 11, percentile: 95 }),
      ...["small_group", "repeat", "unplaced", "synthetic"].map((reason) =>
        readReport({ state: "ready", noRankingReason: reason, others: 1 }),
      ),
    ];
    for (const r of all) {
      expect([r.headline, r.sendNote, ...r.lines].join(" ")).not.toMatch(/[—–]/);
    }
  });
});

describe("helpers", () => {
  it("counts the way a reader does", () => {
    expect([1, 2, 3, 4, 11, 12, 13, 21, 72, 113].map(ordinal)).toEqual([
      "1st", "2nd", "3rd", "4th", "11th", "12th", "13th", "21st", "72nd", "113th",
    ]);
  });
  it("prints a time in UTC, or nothing", () => {
    expect(when("2026-11-20T14:03:09Z")).toBe("2026-11-20 14:03 UTC");
    expect(when("2026-11-20T09:03:09-05:00")).toBe("2026-11-20 14:03 UTC");
    expect(when("")).toBe("");
    expect(when("not a date")).toBe("");
  });
  it("agrees a verb with one other participant", () => {
    expect(noRanking("small_group", 1)).toContain("there is 1.");
    expect(noRanking(undefined, 0)).toContain("there are 0.");
  });
});
