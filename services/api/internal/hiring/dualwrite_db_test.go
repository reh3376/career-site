package hiring

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testRepo connects to TEST_DATABASE_URL the way users' database tests
// do: skipped locally without it, fatal in CI, because a skip there
// would look exactly like a pass.
func testRepo(t *testing.T) (*Repo, context.Context) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_DATABASE_URL is unset in CI: the database-backed tests would skip silently")
		}
		t.Skip("TEST_DATABASE_URL not set; skipping the database-backed path")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return NewRepo(pool), ctx
}

type fixture struct {
	submissionID int64
	runID        string
	chunkIDs     []int64
}

// newFixture makes a submission, a finished jd_runs row with a match
// score, and a corpus document with three chunks, and removes them all
// afterwards. The role the writer creates is removed too: roles keep
// their row when the submission goes (ON DELETE SET NULL), by design.
func newFixture(t *testing.T, r *Repo, ctx context.Context, score float64) fixture {
	t.Helper()
	var f fixture
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO jd_submissions (source_kind, jd_text, jd_hash, text_head, status)
    VALUES ('paste', 'Controls Engineer. 5+ years PLC programming. Ignition SCADA.', gen_random_uuid()::text::bytea, 'Controls Engineer', 'ready')
    RETURNING id`).Scan(&f.submissionID); err != nil {
		t.Fatalf("make submission: %v", err)
	}
	t.Cleanup(func() {
		_, _ = r.pool.Exec(ctx, `DELETE FROM roles WHERE submission_id = $1 OR submission_id IS NULL AND title = 'Controls Engineer (test)'`, f.submissionID)
		_, _ = r.pool.Exec(ctx, `DELETE FROM jd_submissions WHERE id = $1`, f.submissionID)
	})
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO jd_runs (run_id, submission_id, attempt, status, match_score, score_formula, model, prompts, finished_at)
    VALUES (gen_random_uuid(), $1, 1, 'ready', $2, 'v2-nice-bonus', 'test-model', '{"jd_requirements":"v2:abcd"}'::jsonb, now())
    RETURNING run_id::text`, f.submissionID, score).Scan(&f.runID); err != nil {
		t.Fatalf("make run: %v", err)
	}
	var docID int64
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO corpus_documents (source_kind, source_path, title, visibility)
    VALUES ('career_note', 'test/' || gen_random_uuid()::text, 'Test note', 'corpus_only')
    RETURNING id`).Scan(&docID); err != nil {
		t.Fatalf("make document: %v", err)
	}
	t.Cleanup(func() { _, _ = r.pool.Exec(ctx, `DELETE FROM corpus_documents WHERE id = $1`, docID) })
	for i, text := range []string{
		"Programmed Allen-Bradley PLCs for eight years across three plants.",
		"Designed the Ignition SCADA layer and its tag model.",
		"Led FEL-3 for a $60M expansion through to approval.",
	} {
		var id int64
		if err := r.pool.QueryRow(ctx, `
      INSERT INTO corpus_chunks (document_id, chunk_index, text) VALUES ($1, $2, $3) RETURNING id`,
			docID, i, text).Scan(&id); err != nil {
			t.Fatalf("make chunk: %v", err)
		}
		f.chunkIDs = append(f.chunkIDs, id)
	}
	return f
}

// drafts is reqs as the extraction stage drafts them, with the
// database's catalog, which is what the pipeline hands the writer.
func drafts(t *testing.T, r *Repo, ctx context.Context, in []RequirementInput) []Requirement {
	t.Helper()
	catalog, err := r.LoadCatalog(ctx)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	out := make([]Requirement, 0, len(in))
	for _, x := range in {
		out = append(out, Draft(x, catalog, Provenance{PromptID: "jd_requirements", PromptFingerprint: "v5:abcd", Model: "test-model"}))
	}
	return out
}

func reqs() []RequirementInput {
	return []RequirementInput{
		{LocalID: "r1", Text: "5+ years of PLC programming experience", SourceQuote: "5+ years PLC programming", Category: "must", Weight: 3},
		{LocalID: "r2", Text: "Experience with Ignition SCADA", SourceQuote: "Ignition SCADA", Category: "must", Weight: 2},
		{LocalID: "r3", Text: "Front-end development on capital projects", SourceQuote: "", Category: "nice", Weight: 1, NamedParties: nil},
	}
}

// TestDualWriteAgrees is the test 14 §4 names: the rows the writer
// produces agree with the assessment they were written from,
// requirement by requirement, and the match record agrees with
// jd_runs on the score. It runs the writer over the assessment shape
// rather than through the model, because the golden set stores gate
// sides, not per-requirement verdicts, and the pipeline needs a model
// host; v_dual_write_agreement does the same check on every real run.
func TestDualWriteAgrees(t *testing.T) {
	r, ctx := testRepo(t)
	// The v2 score the pipeline stored, which the judgment rows must
	// reproduce: r1 met (3), r2 unresolved, so partial (2 * 0.5), r3
	// not_met and nice, so outside the denominator: 4 / 5.
	const score = 0.8
	f := newFixture(t, r, ctx, score)
	w := NewWriter(nil, r)

	conf := 0.9
	in := RunInput{
		SubmissionID: f.submissionID, Title: "Controls Engineer (test)\nsecond line ignored",
		RunID: f.runID, Attempt: 1, Score: score, Model: "test-model",
		Prompts:       map[string]string{"jd_requirements": "v5:abcd", "requirement_judge": "v13:ef01"},
		JudgePromptID: "requirement_judge", JudgePromptVersion: 13,
		Embedder: "nomic", CorpusFingerprint: "abc123",
		Drafts:             drafts(t, r, ctx, reqs()),
		RequirementsSource: SourceExtraction,
		Judgments: []JudgmentInput{
			{LocalID: "r1", Verdict: "met", EvidenceChunkIDs: []int64{f.chunkIDs[0]},
				Determination: DetMet, Basis: BasisDirect, Uncertainty: []string{}, Rationale: "Eight years of PLC work are stated.",
				ReportedFacts: json.RawMessage(`{"stated_span_years": 8}`), Confidence: &conf, DecisionID: 0},
			{LocalID: "r2", Verdict: "partial", EvidenceChunkIDs: []int64{f.chunkIDs[1]},
				Determination: DetUnresolved, Basis: BasisVocabulary, Uncertainty: []string{UncEquivalenceUnconfirmed}, Rationale: "Designed the SCADA layer; the platform name is not stated."},
			{LocalID: "r3", Verdict: "unmet",
				Determination: DetNotMet, Basis: BasisNotAddressed, Uncertainty: []string{}, Rationale: "No front-end development work was found."},
		},
		Offered: map[string][]Chunk{
			"r1": {{ID: f.chunkIDs[0], Text: "Programmed Allen-Bradley PLCs for eight years across three plants."}},
			"r2": {{ID: f.chunkIDs[1], Text: "Designed the Ignition SCADA layer and its tag model."}},
			"r3": {{ID: f.chunkIDs[2], Text: "Led FEL-3 for a $60M expansion through to approval."}},
		},
	}
	if err := w.Record(ctx, in); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// The role, by submission.
	var roleID, title string
	if err := r.pool.QueryRow(ctx, `SELECT id::text, title FROM roles WHERE submission_id = $1`, f.submissionID).Scan(&roleID, &title); err != nil {
		t.Fatalf("role: %v", err)
	}
	if title != "Controls Engineer (test)" {
		t.Errorf("role title = %q; want the first line of the hint", title)
	}

	// Requirements agree with the assessment, requirement by requirement.
	rows, err := r.pool.Query(ctx, `
    SELECT local_id, version, text, source_quote, category, weight, type, status, extraction->>'type_rule', vocab_version, schema_version
    FROM requirements WHERE role_id = $1::uuid ORDER BY local_id`, roleID)
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		localID, text, quote, category, typ, status, typeRule, vocab, schema string
		version, weight                                                      int
	}
	var got []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.localID, &x.version, &x.text, &x.quote, &x.category, &x.weight, &x.typ, &x.status, &x.typeRule, &x.vocab, &x.schema); err != nil {
			t.Fatal(err)
		}
		got = append(got, x)
	}
	rows.Close()
	want := reqs()
	if len(got) != len(want) {
		t.Fatalf("requirements rows = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, x := range got {
		w := want[i]
		if x.localID != w.LocalID || x.text != w.Text || x.quote != w.SourceQuote || x.category != w.Category || x.weight != w.Weight {
			t.Errorf("requirement %s disagrees with the assessment: got %+v, want %+v", w.LocalID, x, w)
		}
		if x.version != 1 || x.status != "active" || x.typeRule != TypeRule || x.vocab != VocabVersion || x.schema != RequirementSchema {
			t.Errorf("requirement %s: version/status/provenance wrong: %+v", w.LocalID, x)
		}
	}
	if got[0].typ != "experience" || got[1].typ != "tool" {
		t.Errorf("heuristic types: r1=%s (want experience), r2=%s (want tool)", got[0].typ, got[1].typ)
	}

	// The match record carries formula v4's three numbers over the
	// judgment rows and lists the exact requirement versions. r1 is a
	// years item with no equivalency defined, so it is judged, shown and
	// left out of the numbers (FR-HF-23); r2 is required and unresolved
	// (2); r3 is preferred and not met, so it counts at a quarter of its
	// weight (0.25). Nothing evidenced counts, 2 of 2.25 unresolved,
	// 0.25 of 2.25 unmet. The v2 score the gate reads still counts r1.
	var evidenced, unresolved, unmet, eqShare float64
	var reqIDs []string
	var formula string
	var kb, versions []byte
	var supersedes *string
	if err := r.pool.QueryRow(ctx, `
    SELECT m.evidenced, m.unresolved, m.unmet, m.coverage_equivalent_share, m.requirement_ids::text[], m.formula_version, m.keyword_baseline, m.versions, m.supersedes_run_id::text
    FROM match_records m WHERE m.run_id = $1::uuid`, f.runID).
		Scan(&evidenced, &unresolved, &unmet, &eqShare, &reqIDs, &formula, &kb, &versions, &supersedes); err != nil {
		t.Fatalf("match record: %v", err)
	}
	if near := func(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }; !near(evidenced, 0) || !near(unresolved, 2/2.25) || !near(unmet, 0.25/2.25) || eqShare != 0 {
		t.Errorf("coverage = %v / %v / %v (equivalent share %v), want 0 / 0.889 / 0.111 (0)", evidenced, unresolved, unmet, eqShare)
	}
	var r1Included, r2Included bool
	var r1Class, r3Class, r2Notes, r1Standard, r1Template string
	var r1Thresholds []byte
	if err := r.pool.QueryRow(ctx, `
    SELECT a.included, b.included, a.class, c.class, b.equivalence_notes, a.evidence_standard, a.evidence_standard_template, a.thresholds
    FROM requirements a, requirements b, requirements c
    WHERE a.role_id = $1::uuid AND b.role_id = a.role_id AND c.role_id = a.role_id
      AND a.local_id = 'r1' AND b.local_id = 'r2' AND c.local_id = 'r3'`, roleID).
		Scan(&r1Included, &r2Included, &r1Class, &r3Class, &r2Notes, &r1Standard, &r1Template, &r1Thresholds); err != nil {
		t.Fatalf("drafted fields: %v", err)
	}
	if r1Included || !r2Included || r1Class != ClassRequired || r3Class != ClassPreferred {
		t.Errorf("classes and inclusion: r1 included=%v class=%s, r2 included=%v, r3 class=%s", r1Included, r1Class, r2Included, r3Class)
	}
	if min := ParseThresholds(r1Thresholds).MinYears; min == nil || *min != 5 {
		t.Errorf("r1 thresholds = %s, want min_years 5 read from the text", r1Thresholds)
	}
	if r1Template != "experience@"+TemplateVersion || !strings.Contains(r1Standard, "at least 5 years") {
		t.Errorf("r1 evidence standard = %q (%s), want the experience template with the threshold", r1Standard, r1Template)
	}
	if !strings.Contains(r2Notes, "SCADA") {
		t.Errorf("r2 equivalence notes = %q, want the catalog's SCADA entries", r2Notes)
	}
	if len(reqIDs) != 3 || formula != FormulaV4 || supersedes != nil {
		t.Errorf("record: requirement_ids=%v formula=%s supersedes=%v", reqIDs, formula, supersedes)
	}

	// The judgment rows: one per requirement, the derived verdict equal
	// to the verdict the pipeline scored, evidence pointing at the items
	// made from the cited chunks, the provenance counted by kind.
	type jrow struct {
		localID, determination, basis, derived, promptFP string
		uncertainty, evidence                            []string
		provenance                                       []byte
		confidence                                       *float64
		promptVersion                                    int
	}
	jrows := map[string]jrow{}
	jq, err := r.pool.Query(ctx, `
    SELECT requirement_local_id, determination, basis, derived_verdict, prompt_fingerprint, uncertainty, evidence_ids::text[], provenance_summary, confidence, prompt_version
    FROM judgments WHERE run_id = $1::uuid`, f.runID)
	if err != nil {
		t.Fatal(err)
	}
	for jq.Next() {
		var x jrow
		if err := jq.Scan(&x.localID, &x.determination, &x.basis, &x.derived, &x.promptFP, &x.uncertainty, &x.evidence, &x.provenance, &x.confidence, &x.promptVersion); err != nil {
			t.Fatal(err)
		}
		jrows[x.localID] = x
	}
	jq.Close()
	if len(jrows) != 3 {
		t.Fatalf("judgment rows = %d, want 3", len(jrows))
	}
	for _, j := range in.Judgments {
		x := jrows[j.LocalID]
		if x.determination != j.Determination || x.basis != j.Basis || x.derived != j.Verdict {
			t.Errorf("%s: row %s/%s derived %s, input %s/%s verdict %s", j.LocalID, x.determination, x.basis, x.derived, j.Determination, j.Basis, j.Verdict)
		}
		if x.promptFP != "v13:ef01" || x.promptVersion != 13 {
			t.Errorf("%s: prompt provenance %s v%d", j.LocalID, x.promptFP, x.promptVersion)
		}
	}
	if jrows["r2"].uncertainty[0] != UncEquivalenceUnconfirmed || len(jrows["r3"].uncertainty) != 0 {
		t.Errorf("uncertainty: r2=%v r3=%v", jrows["r2"].uncertainty, jrows["r3"].uncertainty)
	}
	// confidence is a real (float32) column, so compare within its precision.
	if c := jrows["r1"].confidence; c == nil || *c-conf > 1e-6 || conf-*c > 1e-6 {
		t.Errorf("r1 confidence = %v, want %v", c, conf)
	}
	var evID string
	if err := r.pool.QueryRow(ctx, `SELECT id::text FROM evidence_items WHERE chunk_id = $1`, f.chunkIDs[0]).Scan(&evID); err != nil {
		t.Fatal(err)
	}
	if len(jrows["r1"].evidence) != 1 || jrows["r1"].evidence[0] != evID || len(jrows["r3"].evidence) != 0 {
		t.Errorf("evidence ids: r1=%v (want %s) r3=%v", jrows["r1"].evidence, evID, jrows["r3"].evidence)
	}
	var prov map[string]int
	if err := json.Unmarshal(jrows["r1"].provenance, &prov); err != nil || prov["corpus_document"] != 1 {
		t.Errorf("r1 provenance_summary = %s (err %v), want one corpus_document", jrows["r1"].provenance, err)
	}
	var baseline struct {
		Rule  string `json:"rule"`
		Items map[string]struct {
			Found bool     `json:"found"`
			Terms []string `json:"terms"`
		} `json:"items"`
	}
	if err := json.Unmarshal(kb, &baseline); err != nil {
		t.Fatalf("keyword_baseline: %v", err)
	}
	if baseline.Rule != KeywordRule || len(baseline.Items) != 3 {
		t.Errorf("keyword baseline = %+v", baseline)
	}
	if !baseline.Items["r2"].Found {
		t.Errorf("r2 (Ignition SCADA against a chunk naming Ignition SCADA) should be found by the keyword filter: %+v", baseline.Items["r2"])
	}
	if baseline.Items["r3"].Found {
		t.Errorf("r3 (front-end development against FEL-3) must not be found by the keyword filter; that gap is the point: %+v", baseline.Items["r3"])
	}
	var v map[string]any
	if err := json.Unmarshal(versions, &v); err != nil || v["formula"] != FormulaV4 || v["rules"] != RulesVersion || v["inclusion_rule"] != InclusionRule || v["requirements_source"] != SourceExtraction || v["vocab"] != VocabVersion || v["keyword_rule"] != KeywordRule {
		t.Errorf("versions snapshot = %s (err %v)", versions, err)
	}

	// Evidence items exist for the cited chunks only, lazily.
	var cited int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM evidence_items WHERE chunk_id = ANY($1)`, f.chunkIDs).Scan(&cited); err != nil {
		t.Fatal(err)
	}
	if cited != 2 {
		t.Errorf("evidence items for cited chunks = %d, want 2 (the third chunk was offered, not cited)", cited)
	}
	var kind, visibility, candidate string
	if err := r.pool.QueryRow(ctx, `SELECT source_kind, visibility, candidate_id::text FROM evidence_items WHERE chunk_id = $1`, f.chunkIDs[0]).Scan(&kind, &visibility, &candidate); err != nil {
		t.Fatal(err)
	}
	if kind != "corpus_document" || visibility != "private" || candidate != OwnerCandidateID {
		t.Errorf("evidence item: kind=%s visibility=%s candidate=%s", kind, visibility, candidate)
	}

	// The production check agrees too.
	var agrees bool
	if err := r.pool.QueryRow(ctx, `SELECT agrees FROM v_dual_write_agreement WHERE run_id = $1::uuid`, f.runID).Scan(&agrees); err != nil {
		t.Fatalf("v_dual_write_agreement: %v", err)
	}
	// requirement_count on jd_runs is 0 in this fixture, so the view's
	// count comparison fails by construction; set it as the scorer does
	// and read again.
	if _, err := r.pool.Exec(ctx, `UPDATE jd_runs SET requirement_count = 3 WHERE run_id = $1::uuid`, f.runID); err != nil {
		t.Fatal(err)
	}
	var recomputed float64
	if err := r.pool.QueryRow(ctx, `SELECT agrees, judgments_v2_score FROM v_dual_write_agreement WHERE run_id = $1::uuid`, f.runID).Scan(&agrees, &recomputed); err != nil || !agrees {
		t.Errorf("v_dual_write_agreement.agrees = %v (err %v), want true; v2 recomputed from the judgment rows = %v", agrees, err, recomputed)
	}
	// And it notices when they disagree: a run whose stored score the
	// rows cannot reproduce is flagged, not smoothed over.
	if _, err := r.pool.Exec(ctx, `UPDATE jd_runs SET match_score = 0.95 WHERE run_id = $1::uuid`, f.runID); err != nil {
		t.Fatal(err)
	}
	if err := r.pool.QueryRow(ctx, `SELECT agrees FROM v_dual_write_agreement WHERE run_id = $1::uuid`, f.runID).Scan(&agrees); err != nil || agrees {
		t.Errorf("v_dual_write_agreement.agrees = %v (err %v) after the score was changed, want false", agrees, err)
	}
	if _, err := r.pool.Exec(ctx, `UPDATE jd_runs SET match_score = $2 WHERE run_id = $1::uuid`, f.runID, score); err != nil {
		t.Fatal(err)
	}

	// A second run with one requirement reworded: r2 gets version 2
	// with supersedes set, the old row is retired, r1 and r3 keep their
	// rows, and the new record cites the new version.
	var run2 string
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO jd_runs (run_id, submission_id, attempt, status, match_score, score_formula, finished_at)
    VALUES (gen_random_uuid(), $1, 2, 'ready', 0.7, 'v2-nice-bonus', now()) RETURNING run_id::text`, f.submissionID).Scan(&run2); err != nil {
		t.Fatal(err)
	}
	in2 := in
	in2.RunID, in2.Attempt, in2.Score = run2, 2, 0.7
	reworded := reqs()
	reworded[1].Text = "Hands-on experience with the Ignition SCADA platform"
	in2.Drafts = drafts(t, r, ctx, reworded)
	if err := w.Record(ctx, in2); err != nil {
		t.Fatalf("Record (second run): %v", err)
	}
	var v2 int
	var supersedesID *string
	var oldStatus string
	if err := r.pool.QueryRow(ctx, `
    SELECT q.version, q.supersedes::text, o.status
    FROM requirements q LEFT JOIN requirements o ON o.id = q.supersedes
    WHERE q.role_id = $1::uuid AND q.local_id = 'r2' AND q.status = 'active'`, roleID).Scan(&v2, &supersedesID, &oldStatus); err != nil {
		t.Fatalf("r2 after rewording: %v", err)
	}
	if v2 != 2 || supersedesID == nil || oldStatus != "retired" {
		t.Errorf("r2: version=%d supersedes=%v old status=%s; want 2, set, retired", v2, supersedesID, oldStatus)
	}
	var active int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM requirements WHERE role_id = $1::uuid AND status = 'active'`, roleID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 3 {
		t.Errorf("active requirements after the second run = %d, want 3", active)
	}
	// r1 and r3 were unchanged (r3 with nil parties on input and {} in
	// the row) and must keep version 1: a new version for an identical
	// extraction would make every run a rewrite.
	for _, localID := range []string{"r1", "r3"} {
		var version, total int
		if err := r.pool.QueryRow(ctx, `
      SELECT max(version), count(*) FROM requirements WHERE role_id = $1::uuid AND local_id = $2`, roleID, localID).Scan(&version, &total); err != nil {
			t.Fatal(err)
		}
		if version != 1 || total != 1 {
			t.Errorf("%s was re-versioned by an identical extraction: max version %d, %d rows", localID, version, total)
		}
	}
	var prev string
	var ids2 []string
	if err := r.pool.QueryRow(ctx, `SELECT supersedes_run_id::text, requirement_ids::text[] FROM match_records WHERE run_id = $1::uuid`, run2).Scan(&prev, &ids2); err != nil {
		t.Fatal(err)
	}
	if prev != f.runID {
		t.Errorf("second record supersedes %q, want the first run %q", prev, f.runID)
	}
	var cites2 bool
	for _, id := range ids2 {
		var localID string
		var version int
		if err := r.pool.QueryRow(ctx, `SELECT local_id, version FROM requirements WHERE id = $1::uuid`, id).Scan(&localID, &version); err != nil {
			t.Fatal(err)
		}
		if localID == "r2" && version == 2 {
			cites2 = true
		}
	}
	if !cites2 {
		t.Errorf("the second record does not cite r2 version 2: %v", ids2)
	}
	var r2Version int
	if err := r.pool.QueryRow(ctx, `
    SELECT q.version FROM judgments j JOIN requirements q ON q.id = j.requirement_id
    WHERE j.run_id = $1::uuid AND j.requirement_local_id = 'r2'`, run2).Scan(&r2Version); err != nil || r2Version != 2 {
		t.Errorf("the second run's r2 judgment cites version %d (err %v), want 2: a judgment names exactly what was judged", r2Version, err)
	}
	var secondRows int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM judgments WHERE run_id = $1::uuid`, run2).Scan(&secondRows); err != nil || secondRows != 3 {
		t.Errorf("second run judgment rows = %d (err %v), want 3", secondRows, err)
	}
	// Writing the same run again replaces its rows rather than failing
	// on the unique index or doubling them.
	if err := w.Record(ctx, in2); err != nil {
		t.Fatalf("Record (second run, again): %v", err)
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM judgments WHERE run_id = $1::uuid`, run2).Scan(&secondRows); err != nil || secondRows != 3 {
		t.Errorf("after a repeated persist judgment rows = %d (err %v), want 3", secondRows, err)
	}
}

// TestWrittenCodesAreRegistered is the control 14 §1 names in place of
// a CHECK constraint: every vocabulary code written to the requirement
// and judgment tables is one the registry knows. A CHECK would make a
// vocabulary change a migration; this makes it an INSERT.
func TestWrittenCodesAreRegistered(t *testing.T) {
	r, ctx := testRepo(t)
	f := newFixture(t, r, ctx, 0.5)
	in := RunInput{SubmissionID: f.submissionID, Title: "Controls Engineer (test)", RunID: f.runID, Attempt: 1, Score: 0.5,
		Drafts: drafts(t, r, ctx, reqs()), Offered: map[string][]Chunk{}}
	if err := NewWriter(nil, r).Record(ctx, in); err != nil {
		t.Fatalf("Record: %v", err)
	}
	reg, err := r.LoadVocab(ctx)
	if err != nil {
		t.Fatalf("LoadVocab: %v", err)
	}
	if len(reg.Codes(VocabBasis)) != 12 || len(reg.Codes(VocabDetermination)) != 4 || len(reg.Codes(VocabUncertainty)) != 8 || len(reg.Codes(VocabRequirementType)) != 9 {
		t.Errorf("vocabulary 1.0 seed: basis=%d determination=%d uncertainty=%d types=%d",
			len(reg.Codes(VocabBasis)), len(reg.Codes(VocabDetermination)), len(reg.Codes(VocabUncertainty)), len(reg.Codes(VocabRequirementType)))
	}
	check := func(vocab, query string) {
		rows, err := r.pool.Query(ctx, query)
		if err != nil {
			t.Fatalf("%s: %v", vocab, err)
		}
		defer rows.Close()
		for rows.Next() {
			var code string
			if err := rows.Scan(&code); err != nil {
				t.Fatal(err)
			}
			if !reg.Known(vocab, code) {
				t.Errorf("%s code %q was written but is not in vocab_codes", vocab, code)
			}
		}
	}
	check(VocabRequirementType, `SELECT DISTINCT type FROM requirements`)
	check(VocabRequirementClass, `SELECT DISTINCT class FROM requirements`)
	check(VocabScopeUnit, `SELECT DISTINCT thresholds->'min_scope'->>'unit' FROM requirements WHERE thresholds ? 'min_scope'`)
	check(VocabCredentialLevel, `SELECT DISTINCT thresholds->>'credential_level' FROM requirements WHERE thresholds ? 'credential_level'`)
	check(VocabRelationshipKind, `SELECT DISTINCT thresholds->>'relationship_kind' FROM requirements WHERE thresholds ? 'relationship_kind'`)
	check(VocabDetermination, `SELECT DISTINCT determination FROM judgments`)
	check(VocabBasis, `SELECT DISTINCT basis FROM judgments`)
	check(VocabBasis, `SELECT DISTINCT effective_basis FROM judgments`)
	check(VocabUncertainty, `SELECT DISTINCT unnest(uncertainty) FROM judgments`)
	check(VocabCatalogKind, `SELECT DISTINCT kind FROM equivalence_catalog`)
	check(VocabCatalogStrength, `SELECT DISTINCT strength FROM equivalence_catalog`)
	for vocab, codes := range map[string][]string{
		VocabRequirementType:  RequirementTypes,
		VocabRequirementClass: RequirementClasses,
		VocabScopeUnit:        ScopeUnits,
		VocabCredentialLevel:  CredentialLevels,
		VocabRelationshipKind: RelationshipKinds,
	} {
		for _, c := range codes {
			if !reg.Known(vocab, c) {
				t.Errorf("compiled-in %s code %q is not in the database registry", vocab, c)
			}
		}
		if got := len(reg.Codes(vocab)); got != len(codes) {
			t.Errorf("%s: the registry has %d live codes, this build knows %d", vocab, got, len(codes))
		}
	}
}
