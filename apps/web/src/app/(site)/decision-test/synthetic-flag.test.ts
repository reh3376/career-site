import { readFileSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

// The ?synthetic=1 flag, checked by reading the source.
//
// This is a crude test and it is the right one. The flag exists so an
// agent can drive a whole run through the real UI, and its failure mode
// is not an exception or a wrong number: it is the client quietly
// sending a constant, which looks identical to working from every
// angle except the database.
//
// That is not hypothetical. The debrief question had a column, a proto
// field, handler validation and a UI, and the client always sent `""`,
// so the question never rendered and every run recorded no strategy.
// Nothing failed. There was simply a literal where a value belonged.
// `synthetic: false` was a literal in that same call for the same
// reason, and if somebody re-inlines one while refactoring, the test
// still passes, the page still works, and the whole point of the flag
// is gone.
//
// Parsing source cannot tell you the page behaves. It can tell you the
// value is still wired to something, which is the part that broke.
// `apps/web` has no component harness, so the alternative is nothing.

const briefing = readFileSync(
  join(import.meta.dirname, "briefing.tsx"),
  "utf8",
);
const runScreen = readFileSync(
  join(import.meta.dirname, "../../decision-test/run/run-screen.tsx"),
  "utf8",
);

describe("the synthetic flag on the briefing", () => {
  it("is read from the URL", () => {
    expect(briefing).toContain('get("synthetic")');
  });

  it("accepts 1 and true, because an agent will type either", () => {
    const read = briefing.slice(briefing.indexOf('get("synthetic")'));
    expect(read).toContain('"1"');
    expect(read).toContain('"true"');
  });

  it("is sent to startSession as a value, not a literal", () => {
    // The whole fault class: `synthetic: false` compiled, passed every
    // gate, and meant no run could ever be marked.
    expect(briefing).not.toMatch(/synthetic:\s*(true|false)\b/);
    expect(briefing).toMatch(/^\s*synthetic,\s*$/m);
  });

  it("warns the participant it is not a real run", () => {
    // The flag is on a public URL. Somebody can arrive on it by
    // accident, and the cost of not saying so is fifteen minutes of
    // somebody's unbroken attention thrown away without their knowing.
    expect(briefing).toContain("This is a test run, not a real one.");
  });

  it("lets a synthetic run past the tap check", () => {
    // Tapping in rhythm is a motor skill. An agent cannot do it, and
    // Continue used to render only on `tap?.passed`, so no agent could
    // reach the blocks, the recall or the debrief at all. This single
    // condition is what makes the rest of the run reachable.
    expect(briefing).toContain("(tap?.passed || synthetic)");
  });

  it("does not lie about the check in the data", () => {
    // Letting the run continue is a UI affordance. The recorded
    // condition stays whatever the check actually found, so a synthetic
    // row says the check was not passed.
    expect(briefing).toContain("tapCheckPassed: tap?.passed ?? false");
  });
});

describe("the synthetic flag on the run screen", () => {
  it("survives the navigation", () => {
    // The briefing is a minute. The run is fifteen. A warning that
    // vanishes at the handoff is a warning shown at the wrong time.
    expect(briefing).toMatch(/^\s*synthetic,\s*$/m);
    expect(runScreen).toContain("synthetic?: boolean");
    expect(runScreen).toContain("handoff.synthetic");
  });
});
