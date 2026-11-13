import type { RecordData, RecordRow } from "@/components/match-record/record-view";

// A real match record, turned into the display model the record view
// draws (FSD FR-HF-17; Phase 0 step 6).
//
// HiringService.GetMatchRecord sends structure: requirement versions,
// judgments, three numbers, vocabulary codes with their labels. This
// file turns that into the sentences a non-expert reads, in the shape
// the pilot's stimulus has, which is the mock of this view: a real
// record and the stimulus are drawn by one component, so regenerating
// the stimulus from a real record is this function's output written to
// a file.
//
// Labels and meanings of vocabulary codes come from the registry the
// API sends with the record, never from copy here (14 §3.1). What is
// written here is what has no code: how a class of items is counted in
// a sentence, what a gap class means, what a mark on a row means.
//
// Connect's JSON leaves out a field at its default, so a false, a zero
// and an empty list all arrive as undefined, and int64 arrives as a
// string. Everything below reads them that way.

export type VocabCode = { code?: string; label?: string; description?: string };

export type ApiThresholds = {
  minYears?: number;
  recencyYears?: number;
  minScope?: { unit?: string; value?: number };
  credentialLevel?: string;
  credentialField?: string;
  relationshipKind?: string;
};

export type ApiRequirement = {
  id?: string;
  localId?: string;
  version?: number;
  text?: string;
  sourceQuote?: string;
  type?: string;
  category?: string;
  requirementClass?: string;
  weight?: number;
  thresholds?: ApiThresholds;
  included?: boolean;
  inclusionReason?: string;
  equivalency?: string;
  editedAt?: string;
};

// An override is sent to an admin only. The member who submitted a
// posting is sent the record as the model left it (owner's decision,
// 2026-11-13), so nothing below ever has one to draw for a member.
export type ApiOverride = {
  determination?: string;
  basis?: string;
  role?: string;
  at?: string;
  byDisplay?: string;
  reason?: string;
  // The run the override was set on, when it was carried to this run
  // because the requirement is unchanged.
  carriedFromAttempt?: number;
  // The model's verdict on this run is not the one the person overrode.
  recheck?: boolean;
  modelDeterminationThen?: string;
  modelBasisThen?: string;
};

export type ApiJudgment = {
  id?: string;
  requirementId?: string;
  requirementLocalId?: string;
  determination?: string;
  basis?: string;
  uncertainty?: string[];
  effectiveDetermination?: string;
  effectiveBasis?: string;
  override?: ApiOverride;
  rationale?: string;
  evidence?: { id?: string; sourceKind?: string; title?: string; chunkId?: string | number; withdrawn?: boolean }[];
  ruleAdjustments?: { rule?: string; reason?: string; fromDetermination?: string; toDetermination?: string }[];
  reportedFacts?: { statedSpanYears?: number; mostRecentYear?: number; statedScope?: { unit?: string; value?: number } };
  gapClass?: string;
  notAssessed?: boolean;
  stale?: boolean;
  keywordKnown?: boolean;
  keywordFound?: boolean;
  keywordTerms?: string[];
  decisionId?: string | number;
  earlierOverride?: {
    attempt?: number;
    determination?: string;
    basis?: string;
    role?: string;
    at?: string;
    sameVersion?: boolean;
  };
};

export type ApiRecord = {
  runId?: string;
  roleId?: string;
  submissionId?: string;
  attempt?: number;
  supersedesRunId?: string;
  superseded?: boolean;
  runTrigger?: string;
  roleTitle?: string;
  employer?: string;
  coverage?: {
    evidenced?: number;
    unresolved?: number;
    unmet?: number;
    equivalentShare?: number;
    selfAttestedShare?: number;
    formulaVersion?: string;
    preferredFraction?: number;
  };
  requirements?: ApiRequirement[];
  judgments?: ApiJudgment[];
  overrideCount?: number;
  keywordRule?: string;
  keywordSkipped?: string;
  versions?: Record<string, string>;
  fit?: string;
  matchScore?: number;
  finishedAt?: string;
  corpusDocuments?: number;
  model?: string;
  requirementsEdited?: boolean;
  // The gate's score and fit worked out again with a person's verdicts.
  // Sent to an admin when the record carries an override.
  reviewedScore?: number;
  reviewedFit?: string;
};

export type OverrideChoice = { determination?: string; bases?: string[] };

// A résumé written at the owner's request on a run's verdicts after
// review. Sent to an admin only; it is never part of a member's review.
export type ApiReviewResume = {
  runId?: string;
  status?: string;
  error?: string;
  markdown?: string;
  requestedAt?: string;
  finishedAt?: string;
  reviewedScore?: number;
  overrideCount?: number;
  downloadPath?: string;
  model?: string;
};

export type MatchRecordResponse = {
  record?: ApiRecord;
  reviewResume?: ApiReviewResume;
  canWriteReviewResume?: boolean;
  reviewResumeNote?: string;
  determinations?: VocabCode[];
  bases?: VocabCode[];
  uncertainties?: VocabCode[];
  types?: VocabCode[];
  classes?: VocabCode[];
  sourceKinds?: VocabCode[];
  overrideRoles?: VocabCode[];
  canOverride?: boolean;
  overrideChoices?: OverrideChoice[];
};

export type Audience = "member" | "admin";

const EVIDENCED = new Set(["met", "met_equivalent"]);

const FIT_LABEL: Record<string, string> = {
  very_strong: "very strong fit",
  strong: "strong fit",
  possible: "possible fit",
  weak: "weak fit",
  very_weak: "very weak fit",
};

function fitValue(fit: string | undefined, score: number | undefined): string {
  const name = fit ? (FIT_LABEL[fit] ?? fit.replace(/_/g, " ")) : "";
  const figure = typeof score === "number" ? score.toFixed(2) : "";
  return [name, figure].filter(Boolean).join(" · ");
}

// What a gap class means. The class itself is derived by the API from
// the basis; there is no vocabulary row for it to carry a label.
const GAP: Record<string, { label: string; meaning: string }> = {
  lexical: {
    label: "Wording or equivalency",
    meaning:
      "The documents may show the same thing under another name, tool or credential. This is the gap a keyword screen turns into a rejection, and the one an equivalency or a question to the candidate can close.",
  },
  substantive: {
    label: "Substantive",
    meaning: "The work itself falls short of what was asked: in scale, in years, in how recent it is, or in the relationship named.",
  },
  unaddressed: {
    label: "Not addressed",
    meaning: "Nothing in the documents speaks to this requirement.",
  },
};

function label(list: VocabCode[] | undefined, code: string | undefined): string {
  if (!code) return "";
  const hit = (list ?? []).find((c) => c.code === code);
  // A code this build's registry does not hold is shown as written
  // rather than dropped (04 §0 rule 3).
  return hit?.label || code.replace(/_/g, " ");
}

function meaning(list: VocabCode[] | undefined, code: string | undefined): string {
  return (list ?? []).find((c) => c.code === code)?.description ?? "";
}

function lowerFirst(s: string): string {
  return s ? s.charAt(0).toLowerCase() + s.slice(1) : s;
}

function day(iso: string | undefined): string {
  if (!iso) return "";
  const d = new Date(iso);
  // The date as written in UTC, so the server's render and the
  // browser's are the same string.
  return Number.isNaN(d.getTime()) ? "" : d.toISOString().slice(0, 10);
}

function num(n: number): string {
  return Number.isInteger(n) ? String(n) : n.toFixed(1).replace(/\.0$/, "");
}

function plural(n: number, one: string, many: string): string {
  return n === 1 ? one : many;
}

function joinAnd(parts: string[]): string {
  if (parts.length <= 1) return parts.join("");
  return `${parts.slice(0, -1).join(", ")} and ${parts[parts.length - 1]}`;
}

// Three shares as whole percentages that add to exactly 100, by the
// largest remainder, so the bar never shows a gap or an overhang.
export function wholePercents(shares: number[]): number[] {
  const total = shares.reduce((a, b) => a + b, 0);
  if (total <= 0) return shares.map(() => 0);
  const exact = shares.map((s) => (s / total) * 100);
  const out = exact.map(Math.floor);
  let left = 100 - out.reduce((a, b) => a + b, 0);
  const order = exact.map((x, i) => ({ i, frac: x - Math.floor(x) })).sort((a, b) => b.frac - a.frac);
  for (const o of order) {
    if (left <= 0) break;
    out[o.i] += 1;
    left -= 1;
  }
  return out;
}

type Counted = { q: ApiRequirement; j: ApiJudgment | undefined };

// How one class of items stands, as a sentence: "Of the 7 required
// items, 4 are met, 1 is met through an equivalency, 2 are unresolved
// and none is unmet."
function classSentence(classLabel: string, items: Counted[]): string {
  const n = items.length;
  if (n === 0) return "";
  let met = 0;
  let equivalent = 0;
  let unresolved = 0;
  let notMet = 0;
  let notIncluded = 0;
  let human = 0;
  for (const { q, j } of items) {
    if (j?.notAssessed) human += 1;
    else if (!q.included) notIncluded += 1;
    else if (j?.effectiveDetermination === "met") met += 1;
    else if (j?.effectiveDetermination === "met_equivalent") equivalent += 1;
    else if (j?.effectiveDetermination === "unresolved") unresolved += 1;
    else notMet += 1;
  }
  const name = classLabel.toLowerCase();
  if (n === 1) {
    const state = human
      ? "is left to human judgement"
      : notIncluded
        ? "is not included in the figures"
        : met
          ? "is met"
          : equivalent
            ? "is met through an equivalency"
            : unresolved
              ? "is unresolved"
              : "is not met";
    return `The one ${name} item ${state}.`;
  }
  const be = (k: number) => plural(k, "is", "are");
  const parts: string[] = [];
  if (met) parts.push(`${met} ${be(met)} met`);
  if (equivalent) parts.push(`${equivalent} ${be(equivalent)} met through an equivalency`);
  if (unresolved) parts.push(`${unresolved} ${be(unresolved)} unresolved`);
  if (notMet) parts.push(`${notMet} ${be(notMet)} not met`);
  else if (met + equivalent + unresolved > 0) parts.push("none is unmet");
  if (notIncluded) parts.push(`${notIncluded} ${be(notIncluded)} not included in the figures`);
  if (human) parts.push(`${human} ${be(human)} left to human judgement`);
  return `Of the ${n} ${name} items, ${joinAnd(parts)}.`;
}

function evidenceLine(j: ApiJudgment, r: MatchRecordResponse, audience: Audience): string {
  const items = j.evidence ?? [];
  if (items.length === 0) return "none";
  const named: string[] = [];
  const unnamed = new Map<string, number>();
  for (const e of items) {
    const kind = label(r.sourceKinds, e.sourceKind);
    const chunk = Number(e.chunkId ?? 0);
    const withdrawn = e.withdrawn ? " (withdrawn)" : "";
    if (e.title) {
      named.push(`${e.title}${audience === "admin" && chunk > 0 ? ` · passage ${chunk}` : ""}${withdrawn}`);
    } else if (audience === "admin" && chunk > 0) {
      named.push(`${kind} · passage ${chunk}${withdrawn}`);
    } else {
      unnamed.set(kind, (unnamed.get(kind) ?? 0) + 1);
    }
  }
  for (const [kind, k] of unnamed) {
    named.push(`${k} ${plural(k, "passage", "passages")} · ${lowerFirst(kind)}, not public`);
  }
  return named.join("; ");
}

// The basis as a line that names its fact (15 §14, point 3): the label,
// then what the label rests on for this row.
function basisLine(q: ApiRequirement, j: ApiJudgment, r: MatchRecordResponse): string {
  if (j.notAssessed) return "Not scored · working-style requirements are not judged from documents";
  const code = j.effectiveBasis;
  const name = label(r.bases, code);
  const years = q.thresholds?.minYears ?? 0;
  const stated = j.reportedFacts?.statedSpanYears ?? 0;
  if (years > 0 && stated > 0 && !j.override) {
    return `${name} · ${num(stated)} ${plural(stated, "year", "years")} stated against a threshold of ${num(years)} (checked in code)`;
  }
  const what = meaning(r.bases, code).replace(/\.$/, "");
  return what ? `${name} · ${lowerFirst(what)}` : name;
}

function keywordLine(j: ApiJudgment): RecordRow["keyword"] {
  if (j.notAssessed || !j.keywordKnown) return undefined;
  const found = Boolean(j.keywordFound);
  const evidenced = EVIDENCED.has(j.effectiveDetermination ?? "");
  const terms = (j.keywordTerms ?? []).join(", ");
  if (found && evidenced) {
    return { status: "A keyword screen would have found this", note: terms ? `Matched terms: ${terms}.` : undefined };
  }
  if (!found && evidenced) {
    return {
      status: "A keyword screen would have missed this",
      note: "The requirement's own terms do not appear in the passages; the verdict rests on what the passages describe.",
    };
  }
  if (found && !evidenced) {
    return {
      status: "A keyword screen would have passed this",
      note: `The terms appear in the passages${terms ? ` (${terms})` : ""}, but the passages do not show the requirement satisfied.`,
    };
  }
  return { status: "A keyword screen would not have found this either" };
}

function reviewedBlock(j: ApiJudgment, r: MatchRecordResponse): RecordRow["reviewed"] {
  const o = j.override;
  if (!o) return undefined;
  const role = label(r.overrideRoles, o.role) || "A reviewer";
  const who = o.byDisplay ? `${o.byDisplay}, as ${lowerFirst(role)},` : `A person acting as ${lowerFirst(role)}`;
  const on = day(o.at) ? ` on ${day(o.at)}` : "";
  const pair = (det: string | undefined, basis: string | undefined) =>
    `${label(r.determinations, det)} (${lowerFirst(label(r.bases, basis))})`;
  const set = pair(o.determination, o.basis);
  const agrees = o.determination === j.determination && o.basis === j.basis;
  const carried = (o.carriedFromAttempt ?? 0) > 0;
  const lines = agrees
    ? [`${who} confirmed the model's verdict${on}: ${set}.`]
    : [
        `${who} set this verdict${on}: ${set}.`,
        `The model's verdict on this run is ${pair(j.determination, j.basis)}. It is kept for comparison and does not count in the figures.`,
      ];
  if (carried) {
    lines.push(
      `Carried from run ${o.carriedFromAttempt}: the requirement has not changed since, so the verdict set there still stands on this run.`,
    );
  }
  if (o.recheck) {
    lines.push(
      `Second look needed. When this was set the model's verdict was ${pair(o.modelDeterminationThen, o.modelBasisThen)}; on this run it is ${pair(j.determination, j.basis)}. The person's verdict still counts. Save it again to confirm it, change it, or withdraw it.`,
    );
  }
  if (o.reason) lines.push(`Reason given: ${o.reason}`);
  const title = o.recheck
    ? "Verdict set by a person · second look needed"
    : agrees
      ? "Verdict confirmed by a person"
      : "Verdict set by a person";
  return { title, lines };
}

function earlierLine(j: ApiJudgment, r: MatchRecordResponse): string | undefined {
  const e = j.earlierOverride;
  if (!e) return undefined;
  const role = lowerFirst(label(r.overrideRoles, e.role) || "reviewer");
  const set = `${label(r.determinations, e.determination)} (${lowerFirst(label(r.bases, e.basis))})`;
  const version = e.sameVersion ? "the same version of this requirement" : "an earlier version of this requirement";
  return `On run ${e.attempt ?? "?"} a person acting as ${role} set ${version} to ${set}. This run judged the row again and starts without that override.`;
}

function toRow(q: ApiRequirement, j: ApiJudgment | undefined, r: MatchRecordResponse, audience: Audience): RecordRow {
  const cls = q.requirementClass ?? "";
  const row: RecordRow = {
    id: q.localId ?? "",
    requirement: q.text,
    source_quote: q.sourceQuote || undefined,
    // The view tones the class by this; required reads as "must".
    category: cls === "required" ? "must" : cls === "ideal" ? "ideal" : "nice",
    class_label: label(r.classes, cls),
    type: lowerFirst(label(r.types, q.type)),
    weight: q.weight,
  };
  if (!j) {
    row.determination = "not_assessed";
    row.determination_label = "Not judged in this run";
    row.rationale = "This run holds no judgment for the requirement.";
    return row;
  }
  const effective = j.effectiveDetermination ?? "";
  const t = q.thresholds ?? {};
  const marks: string[] = [];
  const compared = (t.minYears ?? 0) > 0 || (t.recencyYears ?? 0) > 0 || (t.minScope?.value ?? 0) > 0;
  const adjustments = (j.ruleAdjustments ?? []).filter((a) => a.rule !== "behavioral" && a.reason);
  if (!j.notAssessed && (compared || adjustments.length > 0)) marks.push("checked in code");
  if (q.editedAt) marks.push("edited before judging");
  if (j.override) {
    marks.push("set by a person");
    if ((j.override.carriedFromAttempt ?? 0) > 0) marks.push(`carried from run ${j.override.carriedFromAttempt}`);
    if (j.override.recheck) marks.push("second look needed");
  }
  row.marks = marks.length ? marks : undefined;

  row.determination = j.notAssessed ? "not_assessed" : effective;
  row.determination_label = j.notAssessed ? "Human judgement required" : label(r.determinations, effective);
  row.basis = basisLine(q, j, r);
  row.basis_code = j.effectiveBasis;
  row.flagged_for_audit = !j.notAssessed && effective === "met_equivalent";
  row.no_override = Boolean(j.notAssessed);
  row.rationale = j.rationale;
  row.evidence = j.notAssessed ? undefined : evidenceLine(j, r, audience);
  row.keyword = keywordLine(j);
  row.reviewed = reviewedBlock(j, r);
  row.earlier = audience === "admin" ? earlierLine(j, r) : undefined;
  if (adjustments.length) row.adjustments = adjustments.map((a) => a.reason ?? "");

  if (!j.notAssessed && (effective === "unresolved" || effective === "not_met") && j.gapClass && GAP[j.gapClass]) {
    row.gap = GAP[j.gapClass];
  }
  // The model's codes for what is missing stand while the effective
  // verdict is still unresolved; a person who settled the row settled
  // them too.
  if (!j.notAssessed && effective === "unresolved") {
    const codes = j.uncertainty ?? [];
    if (codes.length) {
      row.missing = codes.map((c) => ({ label: label(r.uncertainties, c), meaning: meaning(r.uncertainties, c) }));
    }
  }
  if (!j.notAssessed && (!q.included || q.inclusionReason)) {
    const why = (q.inclusionReason ?? "").replace(/^(Not included|Included)[^.:]*[.:]\s*/i, "");
    const accepted = q.included && q.equivalency ? `Accepted in its place: ${q.equivalency}` : "";
    row.inclusion = {
      status: q.included ? "Included in the evaluation" : "Not included in the evaluation",
      reason: [why, accepted].filter(Boolean).join(" ") || undefined,
    };
  }
  const notes: string[] = [];
  if (j.notAssessed) notes.push("This item has no bearing on the figures or the fit.");
  if (j.stale) notes.push("A document this verdict cites has since been withdrawn; the verdict stands as it was made.");
  if (notes.length) row.note = notes.join(" ");
  return row;
}

function definitions(r: MatchRecordResponse, rows: Counted[]): RecordData["definitions"] {
  const used = <T,>(xs: (T | undefined)[]) => new Set(xs.filter((x): x is T => Boolean(x)));
  const bases = used(rows.flatMap(({ j }) => (j && !j.notAssessed ? [j.effectiveBasis, j.basis] : [])));
  const types = used(rows.map(({ q }) => q.type));
  const gaps = used(
    rows.map(({ j }) =>
      j && !j.notAssessed && (j.effectiveDetermination === "unresolved" || j.effectiveDetermination === "not_met") ? j.gapClass : undefined,
    ),
  );
  const anyHuman = rows.some(({ j }) => j?.notAssessed);
  const anyOverride = rows.some(({ j }) => j?.override);
  const anyEdited = rows.some(({ q }) => q.editedAt);
  const term = (c: VocabCode) => ({ term: c.label ?? c.code, meaning: c.description ?? "" });

  const groups: NonNullable<NonNullable<RecordData["definitions"]>["groups"]> = [
    {
      title: "How items are classed",
      terms: [
        ...(r.classes ?? []).map(term),
        { term: "Weight", meaning: "How much the item matters, 1 to 3. The figures add up weights, not items." },
        {
          term: "Included in the evaluation",
          meaning:
            "A years-of-experience or credential item counts only when the hiring side has defined what is equivalent to it, or the posting's own wording accepts an equivalent. Without that the item is shown and judged but not counted.",
        },
      ],
    },
    {
      title: "What a verdict means",
      terms: [
        ...(r.determinations ?? []).map(term),
        ...(anyHuman
          ? [
              {
                term: "Human judgement required",
                meaning: "A working-style or personality requirement. Not judged from documents and not counted; left to the people who interview.",
              },
            ]
          : []),
      ],
    },
    {
      title: "What a basis means (the reason behind a verdict)",
      terms: [
        ...(r.bases ?? []).filter((c) => bases.has(c.code ?? "")).map(term),
        ...(anyHuman ? [{ term: "Not scored", meaning: "Working-style requirements are not judged from documents." }] : []),
      ],
    },
    { title: "Kinds of requirement", terms: (r.types ?? []).filter((c) => types.has(c.code ?? "")).map(term) },
  ];
  if (gaps.size) {
    groups.push({
      title: "Kinds of gap (on a row that is not met)",
      terms: Object.entries(GAP)
        .filter(([code]) => gaps.has(code))
        .map(([, g]) => ({ term: g.label, meaning: g.meaning })),
    });
  }
  groups.push({
    title: "Marks on a row",
    terms: [
      {
        term: "Checked in code",
        meaning:
          "A number the posting stated (years, how recent, how large) was compared by the software against the number the documents state, not judged by the model.",
      },
      {
        term: "Flagged for expert audit",
        meaning: "The verdict rests on an equivalency; someone who knows the field is asked to confirm it.",
      },
      {
        term: "Keyword screen",
        meaning:
          "Whether a plain match on the requirement's own terms, over the same passages the judge read, would have found it. Shown for comparison; it counts for nothing.",
      },
      ...(anyEdited
        ? [
            {
              term: "Edited before judging",
              meaning: "A person corrected the requirement after it was read from the posting; the row was judged as edited.",
            },
          ]
        : []),
      ...(anyOverride
        ? [
            {
              term: "Set by a person",
              meaning:
                "A reviewer on the hiring side replaced or confirmed the model's verdict. The figures count the person's verdict; the model's is kept for comparison. The recruiter who submitted the posting is shown the model's verdicts only.",
            },
            {
              term: "Carried from an earlier run",
              meaning:
                "A verdict a person set stays in force on later runs while the requirement is unchanged. It is marked for a second look when the model's verdict is no longer the one the person overrode.",
            },
          ]
        : []),
    ],
  });
  return {
    title: "What the words on this record mean",
    intro: "Every term used on the rows is defined here, so that nothing on the record has to be guessed at.",
    groups: groups.filter((g) => (g.terms ?? []).length > 0),
  };
}

function footer(rec: ApiRecord): RecordData["footer"] {
  const v = rec.versions ?? {};
  const fraction = rec.coverage?.preferredFraction ?? 0.25;
  const docs = rec.corpusDocuments ?? 0;
  const how = [
    rec.requirementsEdited
      ? "Requirements were read from the posting by a model, then edited by a person."
      : "Requirements were read from the posting by a model.",
    `Each requirement was judged in one model call against passages retrieved from the candidate's documents${docs ? ` (${docs} ${plural(docs, "document", "documents")})` : ""}.`,
    "Comparisons of years, recency, scale and named organisations were made in code from the facts the model reported.",
    rec.keywordSkipped
      ? "The keyword screen could not be run for this record."
      : "A plain keyword screen was run over the same passages for comparison.",
    `Required items count at their full weight, preferred items at ${num(fraction * 100)}% of it, ideal items as a bonus only; working-style items are not judged from documents.`,
    "Every judgment stores its prompt, the model's response and the evidence it was shown, for audit.",
  ].join(" ");
  const line = (pairs: [string, string | undefined][]) =>
    pairs
      .filter(([, value]) => value)
      .map(([name, value]) => `${name} ${value}`)
      .join(" · ");
  const versions = [
    line([
      ["requirement_schema", v.requirement_schema],
      ["judgment_schema", v.judgment_schema],
      ["vocab", v.vocab],
    ]),
    line([
      ["formula", v.formula],
      ["rules", v.rules],
      ["keyword screen", v.keyword_rule],
      ["inclusion", v.inclusion_rule],
      ["thresholds", v.thresholds_rule],
    ]),
    line([
      ["extractor", v["prompt.jd_requirements"]],
      ["judge", v["prompt.requirement_judge"]],
      ["model", v.model || rec.model],
    ]),
    line([
      ["corpus", v.corpus_fingerprint ? `${v.corpus_fingerprint.slice(0, 8)}…` : undefined],
      ["run", rec.attempt ? `${rec.attempt}${rec.supersedesRunId ? ", supersedes the run before it" : ""}` : undefined],
      ["requirements from", v.requirements_source === "rows" ? "the released list" : v.requirements_source ? "the posting" : undefined],
    ]),
  ].filter(Boolean);
  return { how_title: "How this was produced", how, versions };
}

// toRecordData builds the display model of a real record for its
// reader, or null when the response holds no record.
export function toRecordData(r: MatchRecordResponse, audience: Audience): RecordData | null {
  const rec = r.record;
  if (!rec) return null;
  const byRequirement = new Map<string, ApiJudgment>();
  for (const j of rec.judgments ?? []) if (j.requirementId) byRequirement.set(j.requirementId, j);
  const counted: Counted[] = (rec.requirements ?? []).map((q) => ({ q, j: byRequirement.get(q.id ?? "") }));
  const n = counted.length;

  const cov = rec.coverage;
  const [ev, un, nm] = cov ? wholePercents([cov.evidenced ?? 0, cov.unresolved ?? 0, cov.unmet ?? 0]) : [0, 0, 0];
  const equivalentItems = counted.filter(({ q, j }) => q.included && !j?.notAssessed && j?.effectiveDetermination === "met_equivalent").length;
  const equivalentPct = Math.round((cov?.equivalentShare ?? 0) * 100);
  const selfPct = Math.round((cov?.selfAttestedShare ?? 0) * 100);
  const fraction = cov?.preferredFraction ?? 0.25;

  const summary = (r.classes ?? [])
    .map((c) =>
      classSentence(
        c.label ?? c.code ?? "",
        counted.filter(({ q }) => q.requirementClass === c.code),
      ),
    )
    .filter(Boolean)
    .join(" ");

  const attempt = rec.attempt ?? 1;
  let runNote: string | undefined;
  if (rec.superseded) {
    runNote = "A later run has replaced this record. It is shown as it stood and takes no further changes.";
  } else if (attempt > 1 && rec.runTrigger === "release") {
    runNote = `This is run ${attempt} on this posting. After an earlier run a person edited the requirements, and the edited list was judged again without reading the posting a second time. Rows marked "edited before judging" are the ones that changed.`;
  } else if (attempt > 1) {
    runNote = `This is run ${attempt} on this posting; it replaces the runs before it.`;
  }

  const overrides = rec.overrideCount ?? 0;
  const figureNotes: string[] = [];
  if (overrides > 0) {
    const second = (rec.judgments ?? []).filter((j) => j.override?.recheck).length;
    figureNotes.push(
      `${overrides} ${plural(overrides, "verdict on this record was", "verdicts on this record were")} set or confirmed by a person, and the figures above count ${plural(overrides, "it", "them")}. The recruiter who submitted the posting is shown the model's verdicts and figures only.`,
    );
    if (second > 0) {
      figureNotes.push(
        `${second} carried ${plural(second, "verdict needs", "verdicts need")} a second look: the model's verdict on this run is not the one the person overrode.`,
      );
    }
  }
  // The fit is the hiring side's reading: the run's own, and beside it
  // the same score worked out again with a person's verdicts. Never
  // one in place of the other.
  let fitLines: NonNullable<RecordData["header"]>["fit_lines"];
  if (audience === "admin" && (rec.fit || typeof rec.matchScore === "number")) {
    fitLines = [{ label: "Fit at run time", value: fitValue(rec.fit, rec.matchScore), note: "from the model's verdicts" }];
    if (typeof rec.reviewedScore === "number") {
      fitLines.push({
        label: "Fit after review",
        value: fitValue(rec.reviewedFit, rec.reviewedScore),
        note: `with ${overrides} ${plural(overrides, "verdict", "verdicts")} set by a person; nothing is sent on this figure`,
      });
    }
  }

  const header: RecordData["header"] = {
    run_label: `Match record · run ${attempt}${day(rec.finishedAt) ? ` · ${day(rec.finishedAt)}` : ""}`,
    role_title: rec.roleTitle || "Untitled role",
    subtitle: [rec.employer, rec.submissionId ? `Posting #${rec.submissionId}` : ""].filter(Boolean).join(" · "),
    intro: `${n} ${plural(n, "requirement", "requirements")} ${
      rec.requirementsEdited ? "read from the posting and edited by a person on the hiring side" : "read from the posting"
    }. Each one was judged against the candidate's own documents. Every row below shows the requirement, the posting's own words, the verdict, the basis for the verdict, the reasoning and the evidence it rests on.${
      audience === "admin" ? " A person on the hiring side can overrule any verdict; where one has, the row says so." : ""
    }`,
    run_note: runNote,
    fit_lines: fitLines,
    summary: summary || undefined,
    coverage_label: `Counted weight · required in full, preferred at ${num(fraction * 100)}%`,
    evidenced_pct: ev,
    unresolved_pct: un,
    unmet_pct: nm,
    equivalency_line:
      equivalentPct > 0
        ? `Of the evidenced weight, ${equivalentPct}% rests on an equivalency (${equivalentItems} ${plural(equivalentItems, "item", "items")}, flagged for expert audit).`
        : "None of the evidenced weight rests on an equivalency.",
    self_attested_line: `${selfPct}% rests on the candidate's own answers to follow-up questions.`,
    figure_notes: figureNotes.length ? figureNotes : undefined,
  };

  return {
    version: "live",
    header,
    definitions: definitions(r, counted),
    rows: counted.map(({ q, j }) => toRow(q, j, r, audience)),
    footer: footer(rec),
  };
}
