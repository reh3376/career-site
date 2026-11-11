package hiring

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// testUser makes a user to be the editor and removes it afterwards.
func testUser(t *testing.T, r *Repo, ctx context.Context) int64 {
	t.Helper()
	var id int64
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO users (email, name, status, role)
    VALUES ('hm-' || gen_random_uuid()::text || '@example.test', 'Test Manager', 'active', 'admin')
    RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("make user: %v", err)
	}
	t.Cleanup(func() { _, _ = r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id) })
	return id
}

// TestEditReleaseRejudge is the step's exit (13 §2 step 5): an edited
// requirement is judged as edited and the judgment names the version.
// It walks J8: a first run on the extraction, the hiring manager's
// edit of the capital-projects requirement, the release, and a second
// run on the released rows.
func TestEditReleaseRejudge(t *testing.T) {
	r, ctx := testRepo(t)
	f := newFixture(t, r, ctx, 0.8)
	editor := testUser(t, r, ctx)
	w := NewWriter(nil, r)
	catalog, err := r.LoadCatalog(ctx)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	// Run 1: the extraction is written as released rows before judging.
	prepared, err := w.Prepare(ctx, f.submissionID, "Controls Engineer (test)", reqs(), Provenance{PromptID: "jd_requirements", PromptFingerprint: "v5:abcd", Model: "test-model"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if prepared.Source != SourceExtraction || len(prepared.Rows) != 3 {
		t.Fatalf("prepared = %+v", prepared)
	}
	byLocal := map[string]Requirement{}
	for _, q := range prepared.Rows {
		if q.Status != StatusActive || q.Version != 1 || q.ReleasedAt == nil || q.ReleasedBy != 0 || q.EditedBy != 0 {
			t.Errorf("%s: an extraction is released by the pipeline as version 1: %+v", q.LocalID, q)
		}
		byLocal[q.LocalID] = q
	}
	run1 := RunInput{
		SubmissionID: f.submissionID, RunID: f.runID, Attempt: 1, Score: 0.8, Model: "test-model",
		Prompts: map[string]string{"requirement_judge": "v14:0001"}, JudgePromptID: "requirement_judge", JudgePromptVersion: 14,
		Rows: prepared.Rows, RequirementsSource: prepared.Source,
		Judgments: []JudgmentInput{
			{LocalID: "r1", Verdict: "met", EvidenceChunkIDs: []int64{f.chunkIDs[0]}, Determination: DetMet, Basis: BasisDirect, Rationale: "Eight years stated."},
			{LocalID: "r2", Verdict: "met", EvidenceChunkIDs: []int64{f.chunkIDs[1]}, Determination: DetMet, Basis: BasisDirect, Rationale: "Ignition named."},
			{LocalID: "r3", Verdict: "partial", EvidenceChunkIDs: []int64{f.chunkIDs[2]}, Determination: DetUnresolved, Basis: BasisVocabulary, Uncertainty: []string{UncEquivalenceUnconfirmed}, Rationale: "Read as software work; none found."},
		},
		Offered: map[string][]Chunk{},
	}
	if err := w.Record(ctx, run1); err != nil {
		t.Fatalf("Record run 1: %v", err)
	}
	if edited, err := w.Edited(ctx, f.submissionID); err != nil || edited {
		t.Fatalf("Edited before any edit = %v (err %v)", edited, err)
	}

	// The edit: r3 is the capital-projects requirement the extractor
	// compressed into something that reads as web work.
	text := "Lead front-end loading of major capital investments: scope definition, alternatives analysis, basis of design"
	typ := "outcome"
	if _, err := r.UpdateRequirement(ctx, byLocal["r3"].ID, RequirementPatch{Text: &text, Type: &typ}, editor, "", catalog); !errors.Is(err, ErrInvalidRequirement) {
		t.Errorf("an edit with no reason: err = %v", err)
	}
	draft, err := r.UpdateRequirement(ctx, byLocal["r3"].ID, RequirementPatch{Text: &text, Type: &typ}, editor,
		"The posting means the definition phase of a capital project, not user-interface work.", catalog)
	if err != nil {
		t.Fatalf("UpdateRequirement: %v", err)
	}
	if draft.Version != 2 || draft.Status != StatusDraft || draft.Supersedes != byLocal["r3"].ID || draft.EditedBy != editor ||
		draft.EditedAt == nil || draft.EditedByDisplay != "Test Manager" || draft.ReleasedAt != nil {
		t.Errorf("the edit is a draft, version 2, superseding version 1, by the editor: %+v", draft)
	}
	if draft.Text != text || draft.Type != "outcome" || draft.Category != "nice" || draft.Class != ClassPreferred {
		t.Errorf("draft fields: %+v", draft)
	}
	if !strings.Contains(draft.EquivalenceNotes, "front-end loading") || draft.EvidenceStandardTemplate != "outcome@"+TemplateVersion {
		t.Errorf("the derived fields did not follow the edit: notes=%q template=%s", draft.EquivalenceNotes, draft.EvidenceStandardTemplate)
	}
	if !strings.Contains(string(draft.Extraction), "jd_requirements") {
		t.Errorf("the edit lost the provenance of the draft it came from: %s", draft.Extraction)
	}

	// Until the release, a run still judges version 1.
	active, err := r.ActiveRequirements(ctx, prepared.RoleID)
	if err != nil || len(active) != 3 {
		t.Fatalf("active = %d (err %v)", len(active), err)
	}
	for _, q := range active {
		if q.LocalID == "r3" && (q.Version != 1 || q.ID != byLocal["r3"].ID) {
			t.Errorf("the released r3 changed before the release: %+v", q)
		}
	}
	// The released version cannot be edited while a draft is pending,
	// and the draft can: a second edit replaces the first.
	other := "something else"
	if _, err := r.UpdateRequirement(ctx, byLocal["r3"].ID, RequirementPatch{Text: &other}, editor, "second thoughts", catalog); !errors.Is(err, ErrNotEditable) {
		t.Errorf("editing the released version beside a pending draft: err = %v", err)
	}
	weight := 3
	draft2, err := r.UpdateRequirement(ctx, draft.ID, RequirementPatch{Weight: &weight}, editor, "This is the core of the role.", catalog)
	if err != nil {
		t.Fatalf("editing the draft: %v", err)
	}
	if draft2.Version != 3 || draft2.Supersedes != draft.ID || draft2.Weight != 3 || draft2.Text != text {
		t.Errorf("second edit: %+v", draft2)
	}
	if old, err := r.GetRequirement(ctx, draft.ID); err != nil || old.Status != StatusRetired {
		t.Errorf("the replaced draft: status %s (err %v), want retired", old.Status, err)
	}
	if _, err := r.UpdateRequirement(ctx, draft.ID, RequirementPatch{Text: &other}, editor, "too late", catalog); !errors.Is(err, ErrNotEditable) {
		t.Errorf("editing a retired version: err = %v", err)
	}
	// An extraction never overwrites a person's edit.
	if _, err := w.Prepare(ctx, f.submissionID, "Controls Engineer (test)", reqs(), Provenance{}); !errors.Is(err, ErrHumanEdited) {
		t.Errorf("Prepare over a pending edit: err = %v, want ErrHumanEdited", err)
	}
	if edited, err := w.Edited(ctx, f.submissionID); err != nil || !edited {
		t.Errorf("Edited with a pending draft = %v (err %v)", edited, err)
	}
	// The equivalency on r1 brings the years item into the numbers.
	eq := "Eight years of PLC work on any major platform counts in full."
	if _, err := r.UpdateRequirement(ctx, byLocal["r1"].ID, RequirementPatch{Equivalency: &eq}, editor, "Defining what stands in for the five years.", catalog); err != nil {
		t.Fatalf("defining the equivalency: %v", err)
	}

	// The listing an editor shows: three released, two pending.
	listed, err := r.ListRequirements(ctx, prepared.RoleID, false)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, q := range listed {
		counts[q.Status]++
	}
	if counts[StatusActive] != 3 || counts[StatusDraft] != 2 || len(listed) != 5 {
		t.Errorf("listing = %v", counts)
	}
	if listed[0].LocalID != "r1" || listed[len(listed)-1].LocalID != "r3" {
		t.Errorf("listing order: first %s, last %s", listed[0].LocalID, listed[len(listed)-1].LocalID)
	}
	all, _ := r.ListRequirements(ctx, prepared.RoleID, true)
	if len(all) != 6 {
		t.Errorf("listing with retired versions = %d, want 6", len(all))
	}

	// The release.
	released, err := r.ReleaseRequirements(ctx, prepared.RoleID, editor)
	if err != nil || len(released) != 2 {
		t.Fatalf("ReleaseRequirements = %d (err %v), want 2", len(released), err)
	}
	for _, q := range released {
		if q.Status != StatusActive || q.ReleasedBy != editor || q.ReleasedAt == nil {
			t.Errorf("released %s: %+v", q.LocalID, q)
		}
	}
	if again, err := r.ReleaseRequirements(ctx, prepared.RoleID, editor); err != nil || len(again) != 0 {
		t.Errorf("a second release = %d (err %v), want none and no error", len(again), err)
	}
	if old, _ := r.GetRequirement(ctx, byLocal["r3"].ID); old.Status != StatusRetired {
		t.Errorf("the replaced r3 version 1: status %s, want retired", old.Status)
	}

	// Run 2 judges the released rows, without extracting.
	rows2, err := w.Released(ctx, f.submissionID)
	if err != nil || rows2.Source != SourceRows || len(rows2.Rows) != 3 {
		t.Fatalf("Released = %+v (err %v)", rows2, err)
	}
	var r3 Requirement
	for _, q := range rows2.Rows {
		if q.LocalID == "r3" {
			r3 = q
		}
		if q.LocalID == "r1" && (!q.Included || q.Equivalency != eq || q.Version != 2) {
			t.Errorf("r1 after its equivalency was defined: %+v", q)
		}
	}
	if r3.Version != 3 || r3.Text != text || r3.Weight != 3 || r3.Type != "outcome" || r3.EditedBy != editor {
		t.Fatalf("the released r3 is not the edit: %+v", r3)
	}
	var run2 string
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO jd_runs (run_id, submission_id, attempt, status, match_score, score_formula, finished_at)
    VALUES (gen_random_uuid(), $1, 2, 'ready', 1.0, 'v2-nice-bonus', now()) RETURNING run_id::text`, f.submissionID).Scan(&run2); err != nil {
		t.Fatal(err)
	}
	in2 := run1
	in2.RunID, in2.Attempt, in2.Score = run2, 2, 1.0
	in2.Rows, in2.RequirementsSource = rows2.Rows, rows2.Source
	in2.Judgments = []JudgmentInput{
		run1.Judgments[0], run1.Judgments[1],
		{LocalID: "r3", Verdict: "met", EvidenceChunkIDs: []int64{f.chunkIDs[2]}, Determination: DetMetEquivalent, Basis: BasisVocabulary, Rationale: "Led FEL-3 through approval."},
	}
	if err := w.Record(ctx, in2); err != nil {
		t.Fatalf("Record run 2: %v", err)
	}

	// The exit: the second run's judgment of r3 names version 3, the
	// edit, and the first run's still names version 1.
	version := func(run string) (v int, text string, edited bool) {
		t.Helper()
		if err := r.pool.QueryRow(ctx, `
      SELECT requirement_version, requirement_text, requirement_edited FROM v_judgments_effective
      WHERE run_id = $1::uuid AND requirement_local_id = 'r3'`, run).Scan(&v, &text, &edited); err != nil {
			t.Fatalf("judgment of r3 in run %s: %v", run, err)
		}
		return
	}
	if v, txt, edited := version(f.runID); v != 1 || edited || !strings.Contains(txt, "Front-end development") {
		t.Errorf("run 1 judged r3 version %d (edited %v): %q; want version 1, the extraction", v, edited, txt)
	}
	if v, txt, edited := version(run2); v != 3 || !edited || txt != text {
		t.Errorf("run 2 judged r3 version %d (edited %v): %q; want version 3, the edit", v, edited, txt)
	}

	// The record: r1 now counts (3, met), r2 required (2, met), r3
	// preferred at a quarter of weight 3 (0.75, met by equivalency).
	var evidenced, eqShare float64
	var source string
	if err := r.pool.QueryRow(ctx, `
    SELECT evidenced, coverage_equivalent_share, versions->>'requirements_source' FROM match_records WHERE run_id = $1::uuid`, run2).
		Scan(&evidenced, &eqShare, &source); err != nil {
		t.Fatal(err)
	}
	if evidenced != 1 || source != SourceRows || eqShare < 0.75/5.75-1e-9 || eqShare > 0.75/5.75+1e-9 {
		t.Errorf("run 2 record: evidenced=%v equivalent share=%v source=%s", evidenced, eqShare, source)
	}

	// What a person edited, and from what, is one query.
	var edits, fromExtraction int
	if err := r.pool.QueryRow(ctx, `
    SELECT count(*), count(*) FILTER (WHERE previous_was_extraction) FROM v_requirement_edits WHERE role_id = $1::uuid`, prepared.RoleID).
		Scan(&edits, &fromExtraction); err != nil {
		t.Fatal(err)
	}
	if edits != 3 || fromExtraction != 2 {
		t.Errorf("v_requirement_edits: %d edits, %d of an extraction; want 3 and 2", edits, fromExtraction)
	}

	// A judged version cannot be deleted, which is the point of
	// versioning it.
	if _, err := r.pool.Exec(ctx, `DELETE FROM requirements WHERE id = $1::uuid`, byLocal["r3"].ID); err == nil {
		t.Error("a judged requirement version was deleted")
	}
}

// One released version and one pending draft per requirement, enforced
// where a second writer cannot get round it.
func TestOneActiveAndOneDraftPerRequirement(t *testing.T) {
	r, ctx := testRepo(t)
	f := newFixture(t, r, ctx, 0.5)
	prepared, err := NewWriter(nil, r).Prepare(ctx, f.submissionID, "Controls Engineer (test)", reqs(), Provenance{})
	if err != nil {
		t.Fatal(err)
	}
	q := prepared.Rows[0]
	for _, status := range []string{StatusActive} {
		_, err := r.pool.Exec(ctx, `
      INSERT INTO requirements (role_id, local_id, version, vocab_version, text, type, category, weight, status)
      VALUES ($1::uuid, $2, 99, '1.1', 'a second released version', 'skill', 'must', 2, $3)`, q.RoleID, q.LocalID, status)
		if err == nil {
			t.Errorf("a second %s version of %s was accepted", status, q.LocalID)
		}
	}
	if _, err := r.pool.Exec(ctx, `
    INSERT INTO requirements (role_id, local_id, version, vocab_version, text, type, category, weight, status)
    VALUES ($1::uuid, $2, 98, '1.1', 'first draft', 'skill', 'must', 2, 'draft')`, q.RoleID, q.LocalID); err != nil {
		t.Fatalf("a first draft: %v", err)
	}
	if _, err := r.pool.Exec(ctx, `
    INSERT INTO requirements (role_id, local_id, version, vocab_version, text, type, category, weight, status)
    VALUES ($1::uuid, $2, 97, '1.1', 'second draft', 'skill', 'must', 2, 'draft')`, q.RoleID, q.LocalID); err == nil {
		t.Error("a second draft of the same requirement was accepted")
	}
}
