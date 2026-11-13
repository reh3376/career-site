package hiring

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func recordReqs() []RequirementInput {
	return []RequirementInput{
		{LocalID: "r1", Text: "Experience with Ignition SCADA", SourceQuote: "Ignition SCADA", Category: "must", Weight: 2, Type: "tool"},
		{LocalID: "r2", Text: "Lead front-end loading of capital projects", SourceQuote: "front-end loading", Category: "must", Weight: 3, Type: "outcome"},
		{LocalID: "r3", Text: "Experience integrating a manufacturing execution system", SourceQuote: "", Category: "nice", Weight: 2, Type: "skill"},
		{LocalID: "r4", Text: "Thrives in ambiguity and communicates clearly", SourceQuote: "", Category: "must", Weight: 1, Type: TypeBehavioral},
	}
}

func recordRun(t *testing.T, r *Repo, ctx context.Context, f fixture, runID string, attempt int) {
	t.Helper()
	w := NewWriter(nil, r)
	if err := w.Record(ctx, RunInput{
		SubmissionID: f.submissionID, Title: "Controls Engineer (test)",
		RunID: runID, Attempt: attempt, Score: 0.7, Model: "test-model",
		Prompts:       map[string]string{"jd_requirements": "v5:abcd", "requirement_judge": "v14:ef01"},
		JudgePromptID: "requirement_judge", JudgePromptVersion: 14,
		Embedder: "nomic", CorpusFingerprint: "abc123",
		Drafts:             drafts(t, r, ctx, recordReqs()),
		RequirementsSource: SourceExtraction,
		Judgments: []JudgmentInput{
			{LocalID: "r1", Verdict: "met", EvidenceChunkIDs: []int64{f.chunkIDs[1]},
				Determination: DetMet, Basis: BasisDirect, Uncertainty: []string{}, Rationale: "Designed the Ignition SCADA layer."},
			{LocalID: "r2", Verdict: "partial", EvidenceChunkIDs: []int64{f.chunkIDs[2]},
				Determination: DetUnresolved, Basis: BasisAdjacentDomain, Uncertainty: []string{UncEquivalenceUnconfirmed},
				Rationale:       "Led FEL-3 for an expansion; whether that is front-end loading as the posting means it is not stated.",
				RuleAdjustments: json.RawMessage(`[{"rule":"scope","rule_version":"v2","from_determination":"met","to_determination":"unresolved","from_basis":"direct","to_basis":"adjacent_domain","added_uncertainty":["equivalence_unconfirmed"],"reason":"test adjustment"}]`),
				ReportedFacts:   json.RawMessage(`{"stated_span_years": 4, "most_recent_year": 2024}`)},
			{LocalID: "r3", Verdict: "unmet",
				Determination: DetNotMet, Basis: BasisNotAddressed, Uncertainty: []string{}, Rationale: "No MES integration work was found."},
			{LocalID: "r4", Verdict: "partial", NotAssessed: true,
				Determination: DetUnresolved, Basis: BasisNotAddressed, Uncertainty: []string{UncEvidenceAmbiguous}, Rationale: "Not assessed."},
		},
		Offered: map[string][]Chunk{
			"r1": {{ID: f.chunkIDs[1], Text: "Designed the Ignition SCADA layer and its tag model."}},
			"r2": {{ID: f.chunkIDs[2], Text: "Led FEL-3 for a $60M expansion through to approval."}},
			"r3": {{ID: f.chunkIDs[0], Text: "Programmed Allen-Bradley PLCs for eight years across three plants."}},
		},
	}); err != nil {
		t.Fatalf("Record (attempt %d): %v", attempt, err)
	}
}

func judgmentByLocal(t *testing.T, rec Record, local string) RecordJudgment {
	t.Helper()
	for _, j := range rec.Judgments {
		if j.LocalID == local {
			return j
		}
	}
	t.Fatalf("the record has no judgment for %s", local)
	return RecordJudgment{}
}

// TestOverrideChangesTheEffectiveValue is step 6's override (13 §2;
// FR-HF-08): an override changes the effective value and the record's
// numbers while the model's determination stays on the judgment, every
// call is logged, a withdrawal restores the record exactly, and a
// superseded record takes none.
func TestOverrideChangesTheEffectiveValue(t *testing.T) {
	r, ctx := testRepo(t)
	// The v2 score the gate read: r1 met (2), r2 partial (1.5), r3
	// preferred and unmet so outside the denominator, r4 behavioral and
	// excluded: 3.5 of 5.
	f := newFixture(t, r, ctx, 0.7)
	reviewer := testUser(t, r, ctx)
	recordRun(t, r, ctx, f, f.runID, 1)

	runID, attempt, latest, ok, err := r.LatestRecordRun(ctx, f.submissionID)
	if err != nil || !ok || runID != f.runID || attempt != 1 || latest != 1 {
		t.Fatalf("LatestRecordRun = %s, %d of %d, ok=%v, err=%v", runID, attempt, latest, ok, err)
	}

	// The record as read: v4 over r1 (2, met), r2 (3, unresolved) and r3
	// (2 preferred at a quarter, not met); r4 weighs nothing.
	rec, err := r.GetRecord(ctx, f.runID)
	if err != nil {
		t.Fatalf("GetRecord: %v", err)
	}
	const denom = 5.5
	if !rec.HasNumbers || rec.FormulaVersion != FormulaV4 || !near(rec.Coverage.Evidenced, 2/denom) ||
		!near(rec.Coverage.Unresolved, 3/denom) || !near(rec.Coverage.Unmet, 0.5/denom) || rec.OverrideCount != 0 {
		t.Fatalf("record before any override: %+v", rec.Coverage)
	}
	if len(rec.Requirements) != 4 || len(rec.Judgments) != 4 || rec.Requirements[0].LocalID != "r1" || rec.Judgments[3].LocalID != "r4" {
		t.Fatalf("the record must list requirements and judgments in the order judged: %d and %d", len(rec.Requirements), len(rec.Judgments))
	}
	if rec.Superseded || rec.RunTrigger == "" || rec.Versions["prompt.requirement_judge"] != "v14:ef01" || rec.PreferredFraction != DefaultPreferredFraction {
		t.Errorf("record header: superseded=%v trigger=%q versions=%v fraction=%v", rec.Superseded, rec.RunTrigger, rec.Versions, rec.PreferredFraction)
	}
	j1, j2, j3, j4 := judgmentByLocal(t, rec, "r1"), judgmentByLocal(t, rec, "r2"), judgmentByLocal(t, rec, "r3"), judgmentByLocal(t, rec, "r4")
	if j1.GapClass != "" || j2.GapClass != GapLexical || j3.GapClass != GapUnaddressed || j4.GapClass != "" || !j4.NotAssessed || j2.NotAssessed {
		t.Errorf("gap classes: r1=%q r2=%q r3=%q r4=%q (not assessed %v)", j1.GapClass, j2.GapClass, j3.GapClass, j4.GapClass, j4.NotAssessed)
	}
	if len(j1.Evidence) != 1 || j1.Evidence[0].Title != "Test note" || j1.Evidence[0].Public || j1.Evidence[0].ChunkID != f.chunkIDs[1] || j1.Evidence[0].SourceKind != "corpus_document" {
		t.Errorf("r1 evidence: %+v", j1.Evidence)
	}
	if len(j3.Evidence) != 0 || !j1.KeywordKnown || !j1.KeywordFound || !j3.KeywordKnown || j3.KeywordFound {
		t.Errorf("keyword screen: r1 known=%v found=%v, r3 known=%v found=%v", j1.KeywordKnown, j1.KeywordFound, j3.KeywordKnown, j3.KeywordFound)
	}
	if len(j2.RuleAdjustments) != 1 || j2.RuleAdjustments[0].Rule != "scope" || j2.Facts.StatedSpanYears != 4 || j2.Facts.MostRecentYear != 2024 {
		t.Errorf("r2 adjustments and facts: %+v %+v", j2.RuleAdjustments, j2.Facts)
	}

	// What is refused, and why.
	for name, c := range map[string]struct {
		in   OverrideInput
		want error
	}{
		"no reason":                 {OverrideInput{JudgmentID: j2.ID, Determination: DetMet, Basis: BasisDirect, Role: RoleHiringManager, By: reviewer}, ErrInvalidOverride},
		"no person":                 {OverrideInput{JudgmentID: j2.ID, Determination: DetMet, Basis: BasisDirect, Role: RoleHiringManager, Reason: "x"}, ErrInvalidOverride},
		"met on an equivalency":     {OverrideInput{JudgmentID: j2.ID, Determination: DetMet, Basis: BasisAdjacentDomain, Role: RoleHiringManager, Reason: "x", By: reviewer}, ErrInvalidOverride},
		"a code outside the vocab":  {OverrideInput{JudgmentID: j2.ID, Determination: "partial", Basis: BasisDirect, Role: RoleHiringManager, Reason: "x", By: reviewer}, ErrInvalidOverride},
		"the reserved role":         {OverrideInput{JudgmentID: j2.ID, Determination: DetMet, Basis: BasisDirect, Role: RoleCandidateDispute, Reason: "x", By: reviewer}, ErrInvalidOverride},
		"a role outside the vocab":  {OverrideInput{JudgmentID: j2.ID, Determination: DetMet, Basis: BasisDirect, Role: "owner", Reason: "x", By: reviewer}, ErrInvalidOverride},
		"a working-style row":       {OverrideInput{JudgmentID: j4.ID, Determination: DetMet, Basis: BasisDirect, Role: RoleHiringManager, Reason: "x", By: reviewer}, ErrOverrideRefused},
		"nothing to withdraw":       {OverrideInput{JudgmentID: j2.ID, Withdraw: true, Reason: "x", By: reviewer}, ErrOverrideRefused},
		"a judgment that is absent": {OverrideInput{JudgmentID: "00000000-0000-0000-0000-00000000dead", Determination: DetMet, Basis: BasisDirect, Role: RoleHiringManager, Reason: "x", By: reviewer}, ErrNotFound},
	} {
		if _, err := r.OverrideJudgment(ctx, c.in, nil); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
	var logged int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM decision_log WHERE run_id = $1::uuid AND kind = $2`, f.runID, DecisionKindOverride).Scan(&logged); err != nil || logged != 0 {
		t.Fatalf("a refused override must log nothing: %d rows (err %v)", logged, err)
	}

	// The hiring manager reads FEL-3 as front-end loading: met by
	// equivalence, through an adjacent domain.
	reg, err := r.LoadVocab(ctx)
	if err != nil {
		t.Fatalf("LoadVocab: %v", err)
	}
	const reason = "FEL-3 is front-end loading under another name; the work is the same."
	res, err := r.OverrideJudgment(ctx, OverrideInput{
		JudgmentID: j2.ID, Determination: DetMetEquivalent, Basis: BasisAdjacentDomain,
		Reason: "  " + reason + "  ", Role: RoleHiringManager, By: reviewer,
	}, reg)
	if err != nil {
		t.Fatalf("OverrideJudgment: %v", err)
	}
	if res.RunID != f.runID || res.LocalID != "r2" || res.ModelDetermined != DetUnresolved || res.Effective != DetMetEquivalent ||
		res.PreviousEffective != DetUnresolved || res.OverrideCount != 1 || res.DecisionID == 0 || res.SubmissionID != f.submissionID {
		t.Errorf("override result: %+v", res)
	}
	rec, err = r.GetRecord(ctx, f.runID)
	if err != nil {
		t.Fatalf("GetRecord after the override: %v", err)
	}
	if !near(rec.Coverage.Evidenced, 5/denom) || rec.Coverage.Unresolved != 0 || !near(rec.Coverage.Unmet, 0.5/denom) ||
		!near(rec.Coverage.EquivalentShare, 0.6) || rec.OverrideCount != 1 {
		t.Errorf("the numbers must be computed again over effective values: %+v count=%d", rec.Coverage, rec.OverrideCount)
	}
	j2 = judgmentByLocal(t, rec, "r2")
	if j2.Determination != DetUnresolved || j2.Basis != BasisAdjacentDomain || j2.DerivedVerdict != "partial" ||
		len(j2.Uncertainty) != 1 || j2.EffectiveDetermination != DetMetEquivalent || j2.EffectiveBasis != BasisAdjacentDomain {
		t.Errorf("the model's values must stay beside the effective ones: %+v", j2)
	}
	if o := j2.Override; o == nil || o.Determination != DetMetEquivalent || o.Role != RoleHiringManager || o.Reason != reason ||
		o.By != reviewer || o.ByDisplay != "Test Manager" || o.At.IsZero() {
		t.Errorf("the override as stored: %+v", j2.Override)
	}

	// Logged with the provenance of a decision, already reviewed, on
	// the run it belongs to.
	var kind, model, refKind, key, humanVerdict, humanNote string
	var refID, reviewedBy int64
	var input, output []byte
	var reviewed bool
	if err := r.pool.QueryRow(ctx, `
    SELECT kind, model, ref_kind, ref_id, key, input, output, COALESCE(human_verdict, ''), COALESCE(human_note, ''),
           COALESCE(reviewed_by, 0), reviewed_at IS NOT NULL
    FROM decision_log WHERE id = $1 AND run_id = $2::uuid`, res.DecisionID, f.runID).
		Scan(&kind, &model, &refKind, &refID, &key, &input, &output, &humanVerdict, &humanNote, &reviewedBy, &reviewed); err != nil {
		t.Fatalf("decision row: %v", err)
	}
	var in struct {
		JudgmentID string `json:"judgment_id"`
		Model      struct {
			Determination string `json:"determination"`
		} `json:"model"`
		PreviousOverride any `json:"previous_override"`
	}
	var out struct {
		Action, Determination, Basis, Role, Reason, Verdict string
	}
	_ = json.Unmarshal(input, &in)
	_ = json.Unmarshal(output, &out)
	if kind != DecisionKindOverride || model != "human" || refKind != "jd_submission" || refID != f.submissionID || key != "r2" ||
		humanVerdict != "met" || humanNote != reason || reviewedBy != reviewer || !reviewed {
		t.Errorf("decision row: kind=%s model=%s ref=%s/%d key=%s verdict=%s note=%q by=%d reviewed=%v", kind, model, refKind, refID, key, humanVerdict, humanNote, reviewedBy, reviewed)
	}
	if in.JudgmentID != j2.ID || in.Model.Determination != DetUnresolved || in.PreviousOverride != nil ||
		out.Action != "override" || out.Determination != DetMetEquivalent || out.Role != RoleHiringManager || out.Verdict != "met" {
		t.Errorf("decision row content: in=%+v out=%+v", in, out)
	}

	// The hand-over check reads the model's verdicts, so an override
	// does not make a run disagree with itself. (The fixture's run row
	// carries no requirement count, which the view also compares.)
	if _, err := r.pool.Exec(ctx, `UPDATE jd_runs SET requirement_count = 4 WHERE run_id = $1::uuid`, f.runID); err != nil {
		t.Fatal(err)
	}
	var agrees bool
	if err := r.pool.QueryRow(ctx, `SELECT agrees FROM v_dual_write_agreement WHERE run_id = $1::uuid`, f.runID).Scan(&agrees); err != nil || !agrees {
		t.Errorf("v_dual_write_agreement after an override: agrees=%v err=%v", agrees, err)
	}
	// And the agreement view now has a row to count: the person said
	// met_equivalent where the model said unresolved.
	var disagreed int
	if err := r.pool.QueryRow(ctx, `
    SELECT COALESCE(sum(judgments), 0) FROM v_determination_agreement
    WHERE model_determination = 'unresolved' AND override_determination = 'met_equivalent' AND NOT agreed AND model = 'test-model' AND prompt_version = 14`).Scan(&disagreed); err != nil || disagreed < 1 {
		t.Errorf("v_determination_agreement must count the override: %d (err %v)", disagreed, err)
	}

	// The same override again changes nothing and is refused.
	if _, err := r.OverrideJudgment(ctx, OverrideInput{
		JudgmentID: j2.ID, Determination: DetMetEquivalent, Basis: BasisAdjacentDomain, Reason: reason, Role: RoleHiringManager, By: reviewer,
	}, reg); !errors.Is(err, ErrInvalidOverride) {
		t.Errorf("an identical override: err = %v, want invalid", err)
	}

	// Replaced: the recruiter reads it as a scale that falls short.
	res, err = r.OverrideJudgment(ctx, OverrideInput{
		JudgmentID: j2.ID, Determination: DetNotMet, Basis: BasisScopeShortfall, Reason: "One expansion is not a portfolio.", Role: RoleRecruiter, By: reviewer,
	}, reg)
	if err != nil {
		t.Fatalf("replace override: %v", err)
	}
	rec, _ = r.GetRecord(ctx, f.runID)
	if !near(rec.Coverage.Evidenced, 2/denom) || !near(rec.Coverage.Unmet, 3.5/denom) || rec.OverrideCount != 1 ||
		judgmentByLocal(t, rec, "r2").GapClass != GapSubstantive || res.PreviousEffective != DetMetEquivalent {
		t.Errorf("after replacing: %+v count=%d gap=%s previous=%s", rec.Coverage, rec.OverrideCount, judgmentByLocal(t, rec, "r2").GapClass, res.PreviousEffective)
	}
	var previous string
	if err := r.pool.QueryRow(ctx, `SELECT input->'previous_override'->>'determination' FROM decision_log WHERE id = $1`, res.DecisionID).Scan(&previous); err != nil || previous != DetMetEquivalent {
		t.Errorf("a replacement logs the override it replaced: %q (err %v)", previous, err)
	}

	// Withdrawn: the model's answer is the effective one again and the
	// numbers are exactly what the run wrote.
	if _, err := r.OverrideJudgment(ctx, OverrideInput{JudgmentID: j2.ID, Withdraw: true, Reason: "Set on the wrong row.", By: reviewer}, reg); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	rec, _ = r.GetRecord(ctx, f.runID)
	j2 = judgmentByLocal(t, rec, "r2")
	if !near(rec.Coverage.Evidenced, 2/denom) || !near(rec.Coverage.Unresolved, 3/denom) || !near(rec.Coverage.Unmet, 0.5/denom) ||
		rec.Coverage.EquivalentShare != 0 || rec.OverrideCount != 0 || j2.Override != nil || j2.EffectiveDetermination != DetUnresolved {
		t.Errorf("after withdrawing: %+v count=%d override=%+v", rec.Coverage, rec.OverrideCount, j2.Override)
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM decision_log WHERE run_id = $1::uuid AND kind = $2`, f.runID, DecisionKindOverride).Scan(&logged); err != nil || logged != 3 {
		t.Errorf("three calls, three rows: %d (err %v)", logged, err)
	}

	// A reviewer confirms r1 as the model had it, then the posting is
	// judged again. The new record starts without overrides, says so on
	// the row, and the old record takes no more.
	if _, err := r.OverrideJudgment(ctx, OverrideInput{
		JudgmentID: j1.ID, Determination: DetMet, Basis: BasisDirect, Reason: "Confirmed against the project file.", Role: RoleHiringManager, By: reviewer,
	}, reg); err != nil {
		t.Fatalf("confirming override: %v", err)
	}
	var run2 string
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO jd_runs (run_id, submission_id, attempt, trigger, status, match_score, score_formula, model, prompts, finished_at)
    VALUES (gen_random_uuid(), $1, 2, 'rescore', 'ready', 0.7, 'v2-nice-bonus', 'test-model', '{}'::jsonb, now())
    RETURNING run_id::text`, f.submissionID).Scan(&run2); err != nil {
		t.Fatalf("make run 2: %v", err)
	}
	recordRun(t, r, ctx, f, run2, 2)
	if runID, attempt, latest, ok, err := r.LatestRecordRun(ctx, f.submissionID); err != nil || !ok || runID != run2 || attempt != 2 || latest != 2 {
		t.Errorf("LatestRecordRun after run 2 = %s, %d of %d, ok=%v, err=%v", runID, attempt, latest, ok, err)
	}
	old, err := r.GetRecord(ctx, f.runID)
	if err != nil || !old.Superseded || old.OverrideCount != 1 {
		t.Errorf("run 1 after run 2: superseded=%v count=%d err=%v", old.Superseded, old.OverrideCount, err)
	}
	if _, err := r.OverrideJudgment(ctx, OverrideInput{
		JudgmentID: j3.ID, Determination: DetNotMet, Basis: BasisContradicted, Reason: "x", Role: RoleHiringManager, By: reviewer,
	}, reg); !errors.Is(err, ErrOverrideRefused) {
		t.Errorf("an override on a superseded record: err = %v, want refused", err)
	}
	current, err := r.GetRecord(ctx, run2)
	if err != nil {
		t.Fatalf("GetRecord run 2: %v", err)
	}
	if current.Superseded || current.OverrideCount != 0 || current.SupersedesRunID != f.runID || current.RunTrigger != "rescore" {
		t.Errorf("run 2: superseded=%v count=%d supersedes=%s trigger=%s", current.Superseded, current.OverrideCount, current.SupersedesRunID, current.RunTrigger)
	}
	c1, c2 := judgmentByLocal(t, current, "r1"), judgmentByLocal(t, current, "r2")
	if c1.Override != nil || c1.Earlier == nil || c1.Earlier.Attempt != 1 || c1.Earlier.Determination != DetMet || !c1.Earlier.SameVersion || c1.Earlier.Role != RoleHiringManager {
		t.Errorf("run 2's r1 must say a person decided it on run 1: %+v", c1.Earlier)
	}
	if c2.Earlier != nil {
		t.Errorf("r2's override was withdrawn, so run 2 has nothing earlier to show: %+v", c2.Earlier)
	}

	// A run with no record is not found, and a posting with none says
	// so without an error.
	if _, err := r.GetRecord(ctx, "00000000-0000-0000-0000-00000000dead"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetRecord of an absent run: %v", err)
	}
	if _, _, _, ok, err := r.LatestRecordRun(ctx, -1); ok || err != nil {
		t.Errorf("LatestRecordRun of an absent posting: ok=%v err=%v", ok, err)
	}
}
