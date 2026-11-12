package handlers

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/hiring"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// What a member is sent and what an admin is sent differ only around a
// citation and around an override: no private title, no passage id, no
// reviewer's name or reason, no decision-log id.
func TestJudgmentProtoByAudience(t *testing.T) {
	at := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	j := hiring.RecordJudgment{
		ID: "j1", RequirementID: "q1", LocalID: "r1",
		Determination: hiring.DetUnresolved, Basis: hiring.BasisAdjacentDomain, Uncertainty: []string{hiring.UncEquivalenceUnconfirmed},
		EffectiveDetermination: hiring.DetMetEquivalent, EffectiveBasis: hiring.BasisAdjacentDomain,
		Override: &hiring.RecordOverride{
			Determination: hiring.DetMetEquivalent, Basis: hiring.BasisAdjacentDomain, Role: hiring.RoleHiringManager,
			Reason: "the reviewer's own note", By: 7, ByDisplay: "A Reviewer", At: at,
		},
		Evidence: []hiring.RecordEvidence{
			{ID: "e1", SourceKind: "corpus_document", Title: "A private note", ChunkID: 41, Public: false},
			{ID: "e2", SourceKind: "corpus_document", Title: "A published article", ChunkID: 42, Public: true},
		},
		Facts:      hiring.ReportedFacts{StatedSpanYears: 4, StatedScope: &hiring.Scope{Unit: "usd", Value: 60e6}},
		GapClass:   hiring.GapLexical,
		DecisionID: 991,
		Earlier:    &hiring.EarlierOverride{Attempt: 1, Determination: hiring.DetMet, Role: hiring.RoleRecruiter, At: at, SameVersion: true},
	}
	admin := judgmentProto(j, recordAudience{admin: true})
	if admin.Evidence[0].Title != "A private note" || admin.Evidence[0].ChunkId != 41 || admin.Override.Reason == "" ||
		admin.Override.ByDisplay != "A Reviewer" || admin.DecisionId != 991 || admin.EarlierOverride.GetAttempt() != 1 {
		t.Errorf("an admin is sent the citation and the override whole: %+v", admin)
	}
	member := judgmentProto(j, recordAudience{})
	if member.Evidence[0].Title != "" || member.Evidence[0].ChunkId != 0 || member.Evidence[0].SourceKind != "corpus_document" {
		t.Errorf("a member must not be sent a private document's title or passage: %+v", member.Evidence[0])
	}
	if member.Evidence[1].Title != "A published article" || member.Evidence[1].ChunkId != 0 {
		t.Errorf("a member is sent a public document's title and no passage id: %+v", member.Evidence[1])
	}
	if o := member.Override; o == nil || o.Reason != "" || o.ByDisplay != "" || o.Role != hiring.RoleHiringManager ||
		o.Determination != hiring.DetMetEquivalent || o.At == nil {
		t.Errorf("a member is told a person set the verdict and in what role, not who or why: %+v", member.Override)
	}
	if member.DecisionId != 0 || member.EarlierOverride != nil {
		t.Errorf("a member is sent no decision-log id and no earlier override: %+v", member)
	}
	// Both readers get the same judgment.
	if member.EffectiveDetermination != admin.EffectiveDetermination || member.Determination != hiring.DetUnresolved ||
		member.GapClass != hiring.GapLexical || member.ReportedFacts.StatedSpanYears != 4 || member.ReportedFacts.StatedScope.GetUnit() != "usd" {
		t.Errorf("the judgment itself must not differ by reader: %+v", member)
	}
}

type recordFixture struct {
	submissionID                 int64
	adminID, ownerID, strangerID int64
	runID                        string
}

func newRecordFixture(t *testing.T, h *Hiring, pool *pgxpool.Pool, ctx context.Context) recordFixture {
	t.Helper()
	var f recordFixture
	mk := func(role string) int64 {
		var id int64
		if err := pool.QueryRow(ctx, `
      INSERT INTO users (email, name, status, role)
      VALUES ('rec-' || gen_random_uuid()::text || '@example.test', 'Test ' || $1::text, 'active', $1::user_role) RETURNING id`, role).Scan(&id); err != nil {
			t.Fatalf("make %s: %v", role, err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id) })
		return id
	}
	f.adminID, f.ownerID, f.strangerID = mk("admin"), mk("member"), mk("member")
	if err := pool.QueryRow(ctx, `
    INSERT INTO jd_submissions (source_kind, jd_text, jd_hash, text_head, status, user_id, role_hint, employer_hint)
    VALUES ('paste', 'Controls Engineer. Ignition SCADA. Front-end loading.', gen_random_uuid()::text::bytea, 'Controls Engineer', 'ready', $1, 'Controls Engineer', 'Acme Beverages')
    RETURNING id`, f.ownerID).Scan(&f.submissionID); err != nil {
		t.Fatalf("make submission: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM decision_log WHERE ref_kind = 'jd_submission' AND ref_id = $1`, f.submissionID)
		_, _ = pool.Exec(ctx, `DELETE FROM roles WHERE submission_id = $1`, f.submissionID)
		_, _ = pool.Exec(ctx, `DELETE FROM jd_submissions WHERE id = $1`, f.submissionID)
	})
	if err := pool.QueryRow(ctx, `
    INSERT INTO jd_runs (run_id, submission_id, attempt, status, match_score, score_formula, model, prompts, fit, finished_at)
    VALUES (gen_random_uuid(), $1, 1, 'ready', 0.7, 'v2-nice-bonus', 'test-model', '{}'::jsonb, 'strong', now())
    RETURNING run_id::text`, f.submissionID).Scan(&f.runID); err != nil {
		t.Fatalf("make run: %v", err)
	}
	// One private document and one public one, a passage each.
	chunk := func(title, visibility, text string) int64 {
		var docID, id int64
		if err := pool.QueryRow(ctx, `
      INSERT INTO corpus_documents (source_kind, source_path, title, visibility)
      VALUES ('career_note', 'test/' || gen_random_uuid()::text, $1, $2) RETURNING id`, title, visibility).Scan(&docID); err != nil {
			t.Fatalf("make document: %v", err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM corpus_documents WHERE id = $1`, docID) })
		if err := pool.QueryRow(ctx, `INSERT INTO corpus_chunks (document_id, chunk_index, text) VALUES ($1, 0, $2) RETURNING id`, docID, text).Scan(&id); err != nil {
			t.Fatalf("make chunk: %v", err)
		}
		return id
	}
	private := chunk("Private project file", users.VisibilityCorpusOnly, "Led FEL-3 for a $60M expansion through to approval.")
	public := chunk("Published article on SCADA", users.VisibilityPublic, "Designed the Ignition SCADA layer and its tag model.")

	catalog, err := h.repo.LoadCatalog(ctx)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	var drafts []hiring.Requirement
	for _, in := range []hiring.RequirementInput{
		{LocalID: "r1", Text: "Experience with Ignition SCADA", SourceQuote: "Ignition SCADA", Category: "must", Weight: 2, Type: "tool"},
		{LocalID: "r2", Text: "Lead front-end loading of capital projects", SourceQuote: "Front-end loading", Category: "must", Weight: 3, Type: "outcome"},
		{LocalID: "r3", Text: "Thrives in ambiguity and communicates clearly", Category: "must", Weight: 1, Type: hiring.TypeBehavioral},
	} {
		drafts = append(drafts, hiring.Draft(in, catalog, hiring.Provenance{PromptID: "jd_requirements", PromptFingerprint: "v5:abcd", Model: "test-model"}))
	}
	w := hiring.NewWriter(slog.New(slog.NewTextHandler(io.Discard, nil)), h.repo)
	if err := w.Record(ctx, hiring.RunInput{
		SubmissionID: f.submissionID, Title: "Controls Engineer", RunID: f.runID, Attempt: 1, Score: 0.7, Model: "test-model",
		Prompts:       map[string]string{"requirement_judge": "v14:ef01"},
		JudgePromptID: "requirement_judge", JudgePromptVersion: 14,
		Drafts: drafts, RequirementsSource: hiring.SourceExtraction,
		Judgments: []hiring.JudgmentInput{
			{LocalID: "r1", Verdict: "met", EvidenceChunkIDs: []int64{public}, Determination: hiring.DetMet, Basis: hiring.BasisDirect, Rationale: "Designed the Ignition layer."},
			{LocalID: "r2", Verdict: "partial", EvidenceChunkIDs: []int64{private}, Determination: hiring.DetUnresolved, Basis: hiring.BasisAdjacentDomain,
				Uncertainty: []string{hiring.UncEquivalenceUnconfirmed}, Rationale: "Led FEL-3; the posting's term is not used."},
			{LocalID: "r3", Verdict: "partial", NotAssessed: true, Determination: hiring.DetUnresolved, Basis: hiring.BasisNotAddressed,
				Uncertainty: []string{hiring.UncEvidenceAmbiguous}, Rationale: "Not assessed."},
		},
		Offered: map[string][]hiring.Chunk{
			"r1": {{ID: public, Text: "Designed the Ignition SCADA layer and its tag model."}},
			"r2": {{ID: private, Text: "Led FEL-3 for a $60M expansion through to approval."}},
		},
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	return f
}

// The record and the override through the RPCs: who may read, what
// each reader is sent, what an override does and what it refuses.
func TestMatchRecordAgainstPostgres(t *testing.T) {
	h, pool, ctx := hiringStack(t)
	f := newRecordFixture(t, h, pool, ctx)
	sid := strconv.FormatInt(f.submissionID, 10)
	as := func(id int64, role users.Role) context.Context {
		return WithCaller(ctx, &users.User{ID: id, Role: role})
	}
	get := func(c context.Context, req *v1.GetMatchRecordRequest) (*v1.GetMatchRecordResponse, error) {
		resp, err := h.GetMatchRecord(c, connect.NewRequest(req))
		if err != nil {
			return nil, err
		}
		return resp.Msg, nil
	}

	// Who may read.
	if _, err := get(as(f.strangerID, users.RoleMember), &v1.GetMatchRecordRequest{SubmissionId: sid}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("another member's read: code %v, want not_found", connect.CodeOf(err))
	}
	for name, c := range map[string]struct {
		req  *v1.GetMatchRecordRequest
		code connect.Code
	}{
		"a submission id that is not a number": {&v1.GetMatchRecordRequest{SubmissionId: "abc"}, connect.CodeInvalidArgument},
		"a submission that is absent":          {&v1.GetMatchRecordRequest{SubmissionId: "999999999"}, connect.CodeNotFound},
		"a run id that is not a uuid":          {&v1.GetMatchRecordRequest{SubmissionId: sid, RunId: "x"}, connect.CodeInvalidArgument},
		"a run with no record":                 {&v1.GetMatchRecordRequest{SubmissionId: sid, RunId: "00000000-0000-0000-0000-00000000dead"}, connect.CodeNotFound},
	} {
		if _, err := get(as(f.adminID, users.RoleAdmin), c.req); connect.CodeOf(err) != c.code {
			t.Errorf("%s: code %v, want %v", name, connect.CodeOf(err), c.code)
		}
	}

	// The member who submitted the posting.
	mine, err := get(as(f.ownerID, users.RoleMember), &v1.GetMatchRecordRequest{SubmissionId: sid})
	if err != nil {
		t.Fatalf("the owner's read: %v", err)
	}
	rec := mine.Record
	if rec == nil || rec.RunId != f.runID || rec.SubmissionId != sid || rec.RoleTitle != "Controls Engineer" || rec.Employer != "Acme Beverages" ||
		rec.Attempt != 1 || rec.Superseded || rec.Fit != "strong" || rec.GetMatchScore() != 0.7 || len(rec.Requirements) != 3 || len(rec.Judgments) != 3 {
		t.Fatalf("the record as the owner reads it: %+v", rec)
	}
	// r1 (2, met) and r2 (3, unresolved); r3 is a working-style row.
	if c := rec.Coverage; c == nil || c.FormulaVersion != hiring.FormulaV4 || c.Evidenced != 0.4 || c.Unresolved != 0.6 || c.Unmet != 0 || c.PreferredFraction != hiring.DefaultPreferredFraction {
		t.Errorf("coverage: %+v", rec.Coverage)
	}
	if mine.CanOverride || len(mine.OverrideChoices) != 0 {
		t.Error("a member is offered no override")
	}
	if len(mine.Determinations) != 4 || len(mine.Bases) != 12 || len(mine.Uncertainties) != 8 || len(mine.Types) != 9 ||
		len(mine.Classes) != 3 || len(mine.SourceKinds) != 4 || len(mine.OverrideRoles) != 3 {
		t.Errorf("vocabularies: det=%d bases=%d unc=%d types=%d classes=%d kinds=%d roles=%d", len(mine.Determinations), len(mine.Bases),
			len(mine.Uncertainties), len(mine.Types), len(mine.Classes), len(mine.SourceKinds), len(mine.OverrideRoles))
	}
	r1, r2, r3 := rec.Judgments[0], rec.Judgments[1], rec.Judgments[2]
	if r1.Evidence[0].Title != "Published article on SCADA" || r1.Evidence[0].ChunkId != 0 {
		t.Errorf("r1 cites a public document; the member sees its title: %+v", r1.Evidence[0])
	}
	if r2.Evidence[0].Title != "" || r2.Evidence[0].ChunkId != 0 || r2.Evidence[0].SourceKind != "corpus_document" {
		t.Errorf("r2 cites a private document; the member sees the kind only: %+v", r2.Evidence[0])
	}
	if r2.GapClass != hiring.GapLexical || !r2.KeywordKnown || !r3.NotAssessed || r2.DecisionId != 0 {
		t.Errorf("r2 and r3 as the member reads them: %+v %+v", r2, r3)
	}

	// An admin: the citations whole, and the override offered.
	adminCtx := as(f.adminID, users.RoleAdmin)
	theirs, err := get(adminCtx, &v1.GetMatchRecordRequest{SubmissionId: sid, RunId: f.runID})
	if err != nil {
		t.Fatalf("the admin's read: %v", err)
	}
	if !theirs.CanOverride || len(theirs.OverrideChoices) != 4 || theirs.OverrideChoices[0].Determination != hiring.DetMet ||
		len(theirs.OverrideChoices[0].Bases) != 1 || theirs.Record.Judgments[1].Evidence[0].Title != "Private project file" ||
		theirs.Record.Judgments[1].Evidence[0].ChunkId == 0 {
		t.Errorf("the admin's record: can=%v choices=%+v evidence=%+v", theirs.CanOverride, theirs.OverrideChoices, theirs.Record.Judgments[1].Evidence)
	}

	// Overrides and their refusals.
	for name, c := range map[string]struct {
		req  *v1.OverrideJudgmentRequest
		code connect.Code
	}{
		"a judgment id that is not a uuid": {&v1.OverrideJudgmentRequest{JudgmentId: "x", Determination: "met", Basis: "direct", Reason: "r", Role: "hiring_manager"}, connect.CodeInvalidArgument},
		"a judgment that is absent":        {&v1.OverrideJudgmentRequest{JudgmentId: "00000000-0000-0000-0000-00000000dead", Determination: "met", Basis: "direct", Reason: "r", Role: "hiring_manager"}, connect.CodeNotFound},
		"a pair that contradicts itself":   {&v1.OverrideJudgmentRequest{JudgmentId: r2.Id, Determination: "met", Basis: "adjacent_domain", Reason: "r", Role: "hiring_manager"}, connect.CodeInvalidArgument},
		"the reserved role":                {&v1.OverrideJudgmentRequest{JudgmentId: r2.Id, Determination: "met", Basis: "direct", Reason: "r", Role: "candidate_dispute"}, connect.CodeInvalidArgument},
		"a working-style row":              {&v1.OverrideJudgmentRequest{JudgmentId: r3.Id, Determination: "met", Basis: "direct", Reason: "r", Role: "hiring_manager"}, connect.CodeFailedPrecondition},
		"nothing to withdraw":              {&v1.OverrideJudgmentRequest{JudgmentId: r2.Id, Withdraw: true, Reason: "r"}, connect.CodeFailedPrecondition},
	} {
		if _, err := h.OverrideJudgment(adminCtx, connect.NewRequest(c.req)); connect.CodeOf(err) != c.code {
			t.Errorf("%s: code %v, want %v (%v)", name, connect.CodeOf(err), c.code, err)
		}
	}
	done, err := h.OverrideJudgment(adminCtx, connect.NewRequest(&v1.OverrideJudgmentRequest{
		JudgmentId: r2.Id, Determination: "met_equivalent", Basis: "adjacent_domain",
		Reason: "FEL-3 is front-end loading under another name.", Role: "hiring_manager",
	}))
	if err != nil {
		t.Fatalf("OverrideJudgment: %v", err)
	}
	after := done.Msg.Record
	if after == nil || after.OverrideCount != 1 || after.Coverage.Evidenced != 1 || after.Coverage.Unresolved != 0 || after.Coverage.EquivalentShare != 0.6 {
		t.Fatalf("the record after the override: %+v", after)
	}
	o := after.Judgments[1]
	if o.Determination != "unresolved" || o.EffectiveDetermination != "met_equivalent" || o.Override.GetReason() == "" || o.Override.GetByDisplay() != "Test admin" {
		t.Errorf("the judgment after the override, as an admin reads it: %+v", o)
	}
	// The member sees the verdict a person set, and that a person set it.
	mine, err = get(as(f.ownerID, users.RoleMember), &v1.GetMatchRecordRequest{SubmissionId: sid})
	if err != nil {
		t.Fatalf("the owner's read after the override: %v", err)
	}
	m := mine.Record.Judgments[1]
	if m.EffectiveDetermination != "met_equivalent" || m.Override == nil || m.Override.Role != "hiring_manager" || m.Override.Reason != "" || m.Override.ByDisplay != "" ||
		mine.Record.OverrideCount != 1 || mine.Record.Coverage.Evidenced != 1 {
		t.Errorf("the member's record after the override: %+v count=%d", m, mine.Record.OverrideCount)
	}

	// The result names the record's run for its two readers, and stops
	// naming it once a later run exists without one.
	jdh := NewJd(slog.New(slog.NewTextHandler(io.Discard, nil)), h.users, nil, nil, 0, nil)
	jdh.SetHiring(h.repo)
	result := func(c context.Context) string {
		resp, err := jdh.GetJdResult(c, connect.NewRequest(&v1.GetJdResultRequest{SubmissionId: sid}))
		if err != nil {
			t.Fatalf("GetJdResult: %v", err)
		}
		return resp.Msg.MatchRecordRunId
	}
	if got := result(as(f.ownerID, users.RoleMember)); got != f.runID {
		t.Errorf("the owner's result names run %q, want %q", got, f.runID)
	}
	if _, err := pool.Exec(ctx, `
    INSERT INTO jd_runs (run_id, submission_id, attempt, trigger, status, model, prompts)
    VALUES (gen_random_uuid(), $1, 2, 'rescore', 'running', 'test-model', '{}'::jsonb)`, f.submissionID); err != nil {
		t.Fatalf("make run 2: %v", err)
	}
	if got := result(as(f.ownerID, users.RoleMember)); got != "" {
		t.Errorf("with a later run going, the result must not name the older record: %q", got)
	}
	// The record itself is still readable, and still takes an override:
	// no later record has replaced it.
	still, err := get(adminCtx, &v1.GetMatchRecordRequest{SubmissionId: sid})
	if err != nil || still.Record.GetRunId() != f.runID || !still.CanOverride {
		t.Errorf("the record while a later run is going: run=%q can=%v err=%v", still.GetRecord().GetRunId(), still.GetCanOverride(), err)
	}
}
