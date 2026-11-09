# 0032. Generalise the JD reviewer into the hiring framework's Phase 0

Status: Accepted 2026-11-08

## Context

The JD reviewer (FSD §5.11, live since 2026-09-22) does one thing for
one person: it reads a posting, extracts requirements, judges each
against the owner's corpus with a local model, computes a score in
code, logs every decision, and evaluates itself against a golden set.
It was built as the owner's answer to keyword screening, and the
landing page states the thesis it exists to prove: the terms that
matter are usually equivalent rather than identical, and the screen
happens before the conversation.

Between 2026-11-01 and 2026-11-08 the owner wrote the design for the
general case in a claude.ai project, copied here as
`docs/framework-research/` (eleven documents, PR #382, #384). The
founding document names the problem (keyword false negatives and the
screener expertise gap), the gap analysis maps this repository against
it, and the owner approved the two hiring workflows the components
attach to. On 2026-11-08 the owner delegated the build order and the
open operational decisions to the agent; `13-phase-0-build-plan.md`
records those decisions with their reasoning.

Three facts about the prototype decide the shape of the work.

**The strongest part is provenance and calibration, not the data
model.** Every verdict is in `decision_log` with its inputs; every run
is an immutable `jd_runs` row with prompt fingerprints and a corpus
fingerprint; the golden set and `eval_runs` measure every change; ten
SQL views define every number once. That is ahead of what the
framework document asks of Phase 0. Meanwhile requirements and
judgments live as JSON inside one `jd_submissions.assessment` column:
not versioned, not editable, not joinable to outcomes on their own, and
not shareable across candidates. The gap analysis calls this the single
biggest structural gap, and the owner's "next concrete step" was to lock
those two schemas.

**The three-way verdict cannot carry what the framework needs.**
`partial` absorbs every hard case (different tool, experience for a
degree, right work at smaller scale, old work, and "the corpus does not
mention it") at one value, 0.5. The prototype already shows the strain:
`adjusted`, `stated_span_years`, `parties_evidenced` and
`rationale_issues` were bolted on because the verdict alone could not
say what happened. The owner has decided the replacement
(`03-verdict-taxonomy-and-clarification-loop.md`): `determination`
(`met` · `met_equivalent` · `unresolved` · `not_met`), `basis` (twelve
equivalency categories) and `uncertainty[]` codes, with three reported
numbers instead of one score.

**The cost asymmetry favours doing the data work now.** ADR 0029 made
the same argument for the tenancy seam: a column added while the tables
are small costs one migration; the same column after a year of rows
costs a migration plus an audit of everything that reads them. The
judgments accumulating today are the asset the framework will calibrate
against, and a judgment stored without its requirement version, its
basis and its evidence provenance cannot be given them later.

The alternative was to wait. The handoff said Phase 0 begins after the
employer-side pilot, so that scope follows measured problems rather than
assumed ones. The pilot decides four things, all of them workflow
surfaces: whether hiring managers cut individuals from a slate (C-14),
whether a rejected candidate can dispute a judgment (K-09), whether a
pre-application gap view is worth building (K-04), and how high the
hold-state requirement ranks. It cannot change the core the owner
already approved. Waiting would have idled the part that is decided to
protect the part that is not.

## Decision

**Build Phase 0 of the hiring framework inside this repository, by
promoting the reviewer's JSON to first-class entities, on the parts the
pilot cannot change.** Specifically:

1. **Requirement becomes a versioned row** (`04` §1): `type` taxonomy,
   `category`, `weight`, `alternatives`, `thresholds`, `named_parties`,
   `evidence_standard`, `equivalence_notes`, extraction provenance,
   `edited_by` and `edit_reason`, `supersedes`. The extractor writes a
   draft; a human may edit before judging; a judgment names the exact
   version it was judged against.
2. **Judgment carries determination, basis and uncertainty** (`04`
   §2), with `reported_facts` the model extracts and code compares, an
   override that changes the *effective* value while the model's value
   is preserved, and a provenance summary by evidence kind. The output
   stays schema-bounded (FR-JD-15); consistency rules reject and retry.
3. **Model reports, code decides**, extended. Duration and named
   parties are already code rules; scope, recency and credential join
   them. Every adjustment is recorded as `rule_adjustments` with a
   `rule_version`. A comparison the model makes is never accepted.
4. **Three numbers replace the score** (formula v3): evidenced
   coverage, unresolved weight, unmet weight, over effective
   determinations, plus the share of evidenced weight that rests on
   equivalency and the share that rests on self-attestation.
   `unresolved` is never silently 0.5.
5. **The old verdict is derived, not abandoned.** `met` and
   `met_equivalent` map to met, `unresolved` to partial, `not_met` to
   unmet, in code, so the owner's 31 graded verdicts stay a comparable
   closed series and the agreement rate survives the change. Basis
   agreement starts fresh.
6. **A keyword baseline rides every record**: per requirement, whether
   plain term overlap over the same evidence would have found it. Code,
   no model. Without it the false-negative claim has no comparison.
7. **One candidate in Phase 0.** Every new table carries `candidate_id`
   and `tenant_id` from birth; no second person's documents enter a
   corpus until the owner decides the confidential-material question
   (D-33).
8. **Out of Phase 0 by name**: the HM slate-cut override view (C-14),
   candidate dispute (K-09), the pre-application gap view (K-04), the
   hold-state entity's priority and mechanics, ranking across
   candidates, employer-side outcome links, working-style indicators.
   Each is a `D-` row or a pilot question, not a quiet omission.
9. **Transition by dual write.** The pipeline writes the old jsonb and
   the new rows side by side while a test proves they agree on every
   golden run; the jsonb loses authority when judge v13 passes the
   golden set. Two representations are tolerated only inside that
   window.
10. **Spec first.** The requirements are FSD §5.15 (FR-HF-01 to 21),
    every row Not built at the time of this record. Any step that
    changes the judge logs its calibration in `docs/llm-tuning-log.md`
    and runs the golden set before and after. The sequence is
    `13-phase-0-build-plan.md` §2.

## Consequences

**The judge changes under a four-billion-parameter model.** Twelve
basis codes, eight uncertainty codes and a `reported_facts` object are
more for `qwen3:4b-q8_0` to produce correctly than three words were.
The mitigations are the ones the prototype already proved: field order
settled before free text, a grammar-compiled schema, retry on
inconsistency, and the golden set as the gate. If gate accuracy falls,
v12 stays in production and v13 iterates behind the evaluation. The
model is not changing; the owner decided that in September and nothing
here reopens it.

**The résumé gate's meaning changes.** Under formula v2 a posting with
many `partial` verdicts scored half credit for each; under v3 those
become unresolved weight, reported beside the score rather than inside
it, so evidenced coverage will read lower than the v2 score for the same
posting. Whether the fit bands move is D-32, decided from the step-4
golden run, not by hand.

**The public disclosure page becomes wrong the day v13 ships** unless
it changes in the same deploy. `/how-ask-roger-works` describes met,
partial and unmet and the 1 / 0.5 / 0 arithmetic in detail, and the
page's whole argument is that it describes what runs. FR-HF-21 ties the
copy change to the step that ships the vocabulary.

**Every step touches the FSD and the build statistics.** The status
table in §2b and `build-stats.json` both count requirement rows, so
each PR in the sequence moves counts and regenerates the JSON, or CI
fails. That is the documentation discipline working as designed and it
is budgeted for.

**Evaluations are the schedule's long pole.** A golden run is five to
six hours on the box and refuses to start while the site is busy. The
sequence groups changes so the judge changes once and the extractor
once; each costs one overnight run.

**What this does not decide.** The gate under v3 (D-32), the second
candidate and other people's confidential material (D-33), throughput
for ranking a pool (D-34), the clarification trigger and round policy
beyond Phase 0's one round at the candidate's request (D-35), an
external concept seed such as O*NET or ESCO (D-36), the hold-state
priority (D-37) and the dispute and slate-cut surfaces (D-38). Each is
in FSD §14 with the evidence that will settle it.

**This record will go stale in a specific way.** It describes a
decision taken before any of its code exists. The FR-HF rows say Not
built and will be updated as each lands; if they are still Not built in
three months, this record is a plan that did not happen, and should say
so in a dated status update rather than be deleted.

Related: ADR 0029 (the tenancy seam this builds on), ADR 0030 (collect
relationally, analyse in a graph later; the new tables follow the same
rule), ADR 0031 (a declared control is an enforced control; FR-HF-19's
boundary is enforced in the query, not asserted).
