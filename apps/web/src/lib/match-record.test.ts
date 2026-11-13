import { describe, expect, it } from "vitest";

import { toRecordData, wholePercents, type MatchRecordResponse } from "./match-record";

// A record as Connect's JSON sends it to an admin: defaults left out,
// int64 as a string. Four rows: a met years item, an unresolved row a
// person settled, an unmet preference not included in the figures, and
// a working-style row.
function response(): MatchRecordResponse {
  return {
    determinations: [
      { code: "met", label: "Met", description: "The evidence satisfies the requirement." },
      { code: "met_equivalent", label: "Met by equivalence", description: "Through an equivalency." },
      { code: "unresolved", label: "Unresolved", description: "A nameable fact is missing." },
      { code: "not_met", label: "Not met", description: "Falls short or is not addressed." },
    ],
    bases: [
      { code: "direct", label: "Direct", description: "Same thing, same words or trivially the same." },
      { code: "adjacent_domain", label: "Adjacent domain", description: "Same skill in a neighbouring context." },
      { code: "not_addressed", label: "Not addressed", description: "Nothing in the evidence speaks to it." },
    ],
    uncertainties: [{ code: "equivalence_unconfirmed", label: "Equivalence unconfirmed", description: "Plausible, asserted by no one." }],
    types: [
      { code: "experience", label: "Experience", description: "A length of time." },
      { code: "outcome", label: "Outcome", description: "Something delivered." },
      { code: "credential", label: "Credential", description: "A degree or certificate." },
      { code: "behavioral", label: "Behavioral", description: "A working style." },
    ],
    classes: [
      { code: "required", label: "Required", description: "Full weight." },
      { code: "preferred", label: "Preferred", description: "A fraction of its weight." },
      { code: "ideal", label: "Ideal", description: "Bonus only." },
    ],
    sourceKinds: [{ code: "corpus_document", label: "Corpus document", description: "A passage." }],
    overrideRoles: [{ code: "hiring_manager", label: "Hiring manager", description: "Owns the role." }],
    record: {
      runId: "run-2",
      submissionId: "144",
      attempt: 2,
      supersedesRunId: "run-1",
      runTrigger: "release",
      roleTitle: "Senior Manager, Capital Projects",
      employer: "Acme Beverages",
      coverage: { evidenced: 1, equivalentShare: 0.6, formulaVersion: "v4-classes", preferredFraction: 0.25 },
      overrideCount: 1,
      requirementsEdited: true,
      fit: "possible",
      matchScore: 0.62,
      reviewedScore: 0.78,
      reviewedFit: "strong",
      finishedAt: "2026-11-11T05:20:00Z",
      corpusDocuments: 14,
      versions: { formula: "v4-classes", "prompt.requirement_judge": "v14:42984a17", requirements_source: "rows" },
      requirements: [
        { id: "q1", localId: "r1", text: "10+ years of engineering", requirementClass: "required", type: "experience", weight: 2,
          thresholds: { minYears: 10 }, included: true, inclusionReason: "Included: an equivalency is defined for this years item.",
          equivalency: "A graduate degree counts as two of the ten years." },
        { id: "q2", localId: "r2", text: "Lead front-end loading", sourceQuote: "front-end development", requirementClass: "required",
          type: "outcome", weight: 3, included: true, editedAt: "2026-11-10T00:00:00Z" },
        { id: "q3", localId: "r3", text: "PMP certification", requirementClass: "preferred", type: "credential", weight: 1,
          inclusionReason: "Not included in the evaluation: no equivalency has been defined for this credential item." },
        { id: "q4", localId: "r4", text: "Thrives in ambiguity", requirementClass: "required", type: "behavioral", weight: 1, included: true },
      ],
      judgments: [
        { id: "j1", requirementId: "q1", requirementLocalId: "r1", determination: "met", basis: "direct", effectiveDetermination: "met",
          effectiveBasis: "direct", rationale: "Roles from 1998 to present.", reportedFacts: { statedSpanYears: 28 },
          evidence: [{ id: "e1", sourceKind: "corpus_document", title: "Career facts", chunkId: "3" }, { id: "e2", sourceKind: "corpus_document" }],
          keywordKnown: true, keywordFound: true, keywordTerms: ["engineering", "years"] },
        { id: "j2", requirementId: "q2", requirementLocalId: "r2", determination: "unresolved", basis: "adjacent_domain",
          uncertainty: ["equivalence_unconfirmed"], effectiveDetermination: "met_equivalent", effectiveBasis: "adjacent_domain",
          gapClass: "lexical", rationale: "Led FEL-3.", keywordKnown: true,
          override: { determination: "met_equivalent", basis: "adjacent_domain", role: "hiring_manager", at: "2026-11-12T09:00:00Z",
            byDisplay: "A Reviewer", reason: "FEL-3 is front-end loading." } },
        { id: "j3", requirementId: "q3", requirementLocalId: "r3", determination: "not_met", basis: "not_addressed",
          effectiveDetermination: "not_met", effectiveBasis: "not_addressed", gapClass: "unaddressed", rationale: "No certificate appears.",
          keywordKnown: true },
        { id: "j4", requirementId: "q4", requirementLocalId: "r4", determination: "unresolved", basis: "not_addressed",
          effectiveDetermination: "unresolved", effectiveBasis: "not_addressed", notAssessed: true, rationale: "Not assessed." },
      ],
    },
  };
}

// The same run as the API sends it to the member who submitted the
// posting: the model's verdicts as the effective ones, the numbers over
// those, no override, no reviewed fit, no private titles or passages.
function memberResponse(): MatchRecordResponse {
  const r = response();
  const rec = r.record!;
  rec.coverage = { evidenced: 0.4, unresolved: 0.6, formulaVersion: "v4-classes", preferredFraction: 0.25 };
  rec.overrideCount = undefined;
  rec.reviewedScore = undefined;
  rec.reviewedFit = undefined;
  rec.judgments = (rec.judgments ?? []).map((j) => ({
    ...j,
    override: undefined,
    effectiveDetermination: j.determination,
    effectiveBasis: j.basis,
    evidence: j.evidence?.map((e) => ({ ...e, chunkId: undefined })),
  }));
  return { ...r, canOverride: undefined, overrideChoices: undefined };
}

describe("wholePercents", () => {
  it("adds to exactly 100 by the largest remainder", () => {
    expect(wholePercents([1 / 3, 1 / 3, 1 / 3])).toEqual([34, 33, 33]);
    expect(wholePercents([0.8127, 0.1733, 0.014])).toEqual([81, 17, 2]);
    expect(wholePercents([1, 0, 0])).toEqual([100, 0, 0]);
  });
  it("is all zeros for a record that counts nothing", () => {
    expect(wholePercents([0, 0, 0])).toEqual([0, 0, 0]);
  });
});

describe("toRecordData", () => {
  it("is null when the response holds no record", () => {
    expect(toRecordData({}, "member")).toBeNull();
  });

  it("builds the header from the record, in whole percentages and plain counts", () => {
    const d = toRecordData(response(), "admin");
    expect(d?.header?.run_label).toBe("Match record · run 2 · 2026-11-11");
    expect(d?.header?.subtitle).toBe("Acme Beverages · Posting #144");
    expect([d?.header?.evidenced_pct, d?.header?.unresolved_pct, d?.header?.unmet_pct]).toEqual([100, 0, 0]);
    expect(d?.header?.summary).toBe(
      "Of the 3 required items, 1 is met, 1 is met through an equivalency, none is unmet and 1 is left to human judgement. The one preferred item is not included in the figures.",
    );
    expect(d?.header?.equivalency_line).toContain("60% rests on an equivalency (1 item");
    expect(d?.header?.run_note).toContain("run 2");
    expect(d?.header?.figure_notes?.[0]).toContain("1 verdict on this record was set or confirmed by a person");
    expect(d?.header?.figure_notes?.[0]).toContain("The recruiter who submitted the posting is shown the model's verdicts and figures only.");
  });

  it("shows the hiring side both fits, the run's own and the one after review", () => {
    const lines = toRecordData(response(), "admin")?.header?.fit_lines ?? [];
    expect(lines.map((l) => `${l.label}: ${l.value}`)).toEqual([
      "Fit at run time: possible fit · 0.62",
      "Fit after review: strong fit · 0.78",
    ]);
    expect(lines[1].note).toContain("no résumé is written or sent on this figure");
    // With no override there is nothing after review to show.
    const plain = response();
    plain.record!.reviewedScore = undefined;
    plain.record!.reviewedFit = undefined;
    expect(toRecordData(plain, "admin")?.header?.fit_lines).toHaveLength(1);
  });

  it("shows a recruiter the model's record and nothing of an override", () => {
    const d = toRecordData(memberResponse(), "member");
    const row = d?.rows?.[1];
    expect(row?.determination).toBe("unresolved");
    expect(row?.reviewed).toBeUndefined();
    expect(row?.marks).toEqual(["edited before judging"]);
    expect(row?.missing?.[0].label).toBe("Equivalence unconfirmed");
    expect(row?.gap?.label).toBe("Wording or equivalency");
    expect([d?.header?.evidenced_pct, d?.header?.unresolved_pct, d?.header?.unmet_pct]).toEqual([40, 60, 0]);
    expect(d?.header?.fit_lines).toBeUndefined();
    expect(d?.header?.figure_notes).toBeUndefined();
    expect(d?.header?.intro).not.toContain("overrule");
    const all = JSON.stringify(d);
    expect(all).not.toContain("set by a person");
    expect(all).not.toContain("Set by a person");
    expect(all).not.toContain("A Reviewer");
    expect(all).not.toContain("passage 3");
    expect(d?.rows?.[0].evidence).toBe("Career facts; 1 passage · corpus document, not public");
  });

  it("names the fact behind a basis and marks what code checked", () => {
    const rows = toRecordData(memberResponse(), "member")?.rows ?? [];
    expect(rows[0].basis).toBe("Direct · 28 years stated against a threshold of 10 (checked in code)");
    expect(rows[0].marks).toEqual(["checked in code"]);
    expect(rows[0].class_label).toBe("Required");
    expect(rows[0].category).toBe("must");
    expect(rows[0].inclusion).toEqual({
      status: "Included in the evaluation",
      reason: "an equivalency is defined for this years item. Accepted in its place: A graduate degree counts as two of the ten years.",
    });
    expect(rows[2].inclusion?.status).toBe("Not included in the evaluation");
    expect(rows[2].basis).toBe("Not addressed · nothing in the evidence speaks to it");
    expect(rows[2].gap?.label).toBe("Not addressed");
  });

  it("shows the hiring side a verdict a person set, with the model's beside it", () => {
    const row = toRecordData(response(), "admin")?.rows?.[1];
    expect(row?.determination).toBe("met_equivalent");
    expect(row?.flagged_for_audit).toBe(true);
    expect(row?.marks).toEqual(["edited before judging", "set by a person"]);
    expect(row?.reviewed?.title).toBe("Verdict set by a person");
    expect(row?.reviewed?.lines?.[0]).toBe(
      "A Reviewer, as hiring manager, set this verdict on 2026-11-12: Met by equivalence (adjacent domain).",
    );
    expect(row?.reviewed?.lines?.[1]).toContain("The model's verdict on this run is Unresolved (adjacent domain)");
    expect(row?.reviewed?.lines?.at(-1)).toBe("Reason given: FEL-3 is front-end loading.");
    expect(toRecordData(response(), "admin")?.rows?.[0].evidence).toBe(
      "Career facts · passage 3; 1 passage · corpus document, not public",
    );
    // A settled row no longer lists what was missing, and is not a gap.
    expect(row?.missing).toBeUndefined();
    expect(row?.gap).toBeUndefined();
  });

  it("says where a carried verdict came from and when it needs a second look", () => {
    const r = response();
    const o = r.record!.judgments![1].override!;
    o.carriedFromAttempt = 2;
    let row = toRecordData(r, "admin")?.rows?.[1];
    expect(row?.marks).toEqual(["edited before judging", "set by a person", "carried from run 2"]);
    expect(row?.reviewed?.title).toBe("Verdict set by a person");
    expect(row?.reviewed?.lines?.join(" ")).toContain("Carried from run 2: the requirement has not changed since");

    o.recheck = true;
    o.modelDeterminationThen = "not_met";
    o.modelBasisThen = "not_addressed";
    const d = toRecordData(r, "admin");
    row = d?.rows?.[1];
    expect(row?.marks).toContain("second look needed");
    expect(row?.reviewed?.title).toBe("Verdict set by a person · second look needed");
    expect(row?.reviewed?.lines?.join(" ")).toContain(
      "When this was set the model's verdict was Not met (not addressed); on this run it is Unresolved (adjacent domain).",
    );
    expect(d?.header?.figure_notes?.[1]).toBe(
      "1 carried verdict needs a second look: the model's verdict on this run is not the one the person overrode.",
    );
  });

  it("says what a keyword screen would have done with each row", () => {
    const rows = toRecordData(response(), "admin")?.rows ?? [];
    expect(rows[0].keyword?.status).toBe("A keyword screen would have found this");
    expect(rows[1].keyword?.status).toBe("A keyword screen would have missed this");
    expect(rows[2].keyword?.status).toBe("A keyword screen would not have found this either");
    expect(rows[3].keyword).toBeUndefined();
  });

  it("leaves a working-style row unscored and closed to override", () => {
    const row = toRecordData(response(), "admin")?.rows?.[3];
    expect(row?.determination).toBe("not_assessed");
    expect(row?.determination_label).toBe("Human judgement required");
    expect(row?.no_override).toBe(true);
    expect(row?.evidence).toBeUndefined();
    expect(row?.note).toContain("no bearing on the figures");
  });

  it("defines only the terms the record uses, from the registry", () => {
    const groups = toRecordData(response(), "admin")?.definitions?.groups ?? [];
    const terms = (title: string) => groups.find((g) => g.title?.startsWith(title))?.terms?.map((t) => t.term) ?? [];
    expect(terms("What a basis means")).toEqual(["Direct", "Adjacent domain", "Not addressed", "Not scored"]);
    expect(terms("Kinds of gap")).toEqual(["Not addressed"]);
    expect(terms("Marks on a row")).toContain("Set by a person");
    // A recruiter's record has no such mark to define.
    const mine = toRecordData(memberResponse(), "member")?.definitions?.groups ?? [];
    expect(mine.find((g) => g.title === "Marks on a row")?.terms?.map((t) => t.term)).not.toContain("Set by a person");
    expect(terms("How items are classed").slice(0, 3)).toEqual(["Required", "Preferred", "Ideal"]);
  });

  it("writes no em dash anywhere a reader will see", () => {
    expect(JSON.stringify(toRecordData(response(), "admin"))).not.toMatch(/—|&mdash;/);
  });
});
