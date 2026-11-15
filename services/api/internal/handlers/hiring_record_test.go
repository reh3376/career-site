package handlers

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/corpusscope"
	"github.com/reh3376/career-site/services/api/internal/hiring"
	"github.com/reh3376/career-site/services/api/internal/jd"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// What a member is sent: no private title, no passage id, no
// decision-log id, and no override at all. An override reaching the
// member's rendering is dropped even if the record was not made
// model-only first.
func TestJudgmentProtoByAudience(t *testing.T) {
	at := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	j := hiring.RecordJudgment{
		ID: "j1", RequirementID: "q1", LocalID: "r1",
		Determination: hiring.DetUnresolved, Basis: hiring.BasisAdjacentDomain, Uncertainty: []string{hiring.UncEquivalenceUnconfirmed},
		EffectiveDetermination: hiring.DetMetEquivalent, EffectiveBasis: hiring.BasisAdjacentDomain,
		Override: &hiring.RecordOverride{
			Determination: hiring.DetMetEquivalent, Basis: hiring.BasisAdjacentDomain, Role: hiring.RoleHiringManager,
			Reason: "the reviewer's own note", By: 7, ByDisplay: "A Reviewer", At: at,
			CarriedFromAttempt: 2, Recheck: true, ModelDeterminationThen: hiring.DetNotMet, ModelBasisThen: hiring.BasisNotAddressed,
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
		admin.Override.ByDisplay != "A Reviewer" || admin.DecisionId != 991 || admin.EarlierOverride.GetAttempt() != 1 ||
		admin.Override.CarriedFromAttempt != 2 || !admin.Override.Recheck || admin.Override.ModelDeterminationThen != hiring.DetNotMet {
		t.Errorf("an admin is sent the citation and the override whole: %+v", admin)
	}
	member := judgmentProto(j, recordAudience{})
	if member.Evidence[0].Title != "" || member.Evidence[0].ChunkId != 0 || member.Evidence[0].SourceKind != "corpus_document" {
		t.Errorf("a member must not be sent a private document's title or passage: %+v", member.Evidence[0])
	}
	if member.Evidence[1].Title != "A published article" || member.Evidence[1].ChunkId != 0 {
		t.Errorf("a member is sent a public document's title and no passage id: %+v", member.Evidence[1])
	}
	if member.Override != nil || member.DecisionId != 0 || member.EarlierOverride != nil {
		t.Errorf("a member is sent no override, no decision-log id and no earlier override: %+v", member)
	}
	if member.Determination != hiring.DetUnresolved || member.ReportedFacts.StatedSpanYears != 4 || member.ReportedFacts.StatedScope.GetUnit() != "usd" {
		t.Errorf("the model's judgment itself must not differ by reader: %+v", member)
	}

	// The record as a whole: a member's is the model's own, whatever
	// overrides the record that was passed in carries.
	rec := hiring.Record{
		RunID: "run", FormulaVersion: hiring.FormulaV4, PreferredFraction: 0.25, HasNumbers: true, OverrideCount: 1,
		Coverage:     hiring.Coverage{Evidenced: 1},
		Requirements: []hiring.Requirement{{ID: "q1", LocalID: "r1", Weight: 3, Class: hiring.ClassRequired, Included: true, EditedByDisplay: "An Editor", EditReason: "the editor's note"}},
		Judgments:    []hiring.RecordJudgment{j},
	}
	forMember := recordProto(rec, "A role", "", recordAudience{})
	mj := forMember.Judgments[0]
	if forMember.OverrideCount != 0 || forMember.Coverage.Evidenced != 0 || forMember.Coverage.Unresolved != 1 ||
		mj.Override != nil || mj.EffectiveDetermination != hiring.DetUnresolved || mj.EffectiveBasis != hiring.BasisAdjacentDomain {
		t.Errorf("a member's record is the model's: count=%d coverage=%+v judgment=%+v", forMember.OverrideCount, forMember.Coverage, mj)
	}
	if q := forMember.Requirements[0]; q.EditedByDisplay != "" || q.EditReason != "" {
		t.Errorf("a member is not sent who edited a requirement or why: %+v", q)
	}
	forAdmin := recordProto(rec, "A role", "", recordAudience{admin: true})
	if forAdmin.OverrideCount != 1 || forAdmin.Coverage.Evidenced != 1 || forAdmin.Judgments[0].EffectiveDetermination != hiring.DetMetEquivalent ||
		forAdmin.Requirements[0].EditReason == "" {
		t.Errorf("an admin's record is the record as it stands: %+v", forAdmin)
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
	h.SetBands(jd.NewBandsStore(slog.New(slog.NewTextHandler(io.Discard, nil)), h.users, jd.DefaultBands(0.7)))
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
	// The fit after review rides beside the run's own: both required
	// rows now earn their weight, and the run's score has not moved.
	if after.GetReviewedScore() != 1 || after.ReviewedFit != jd.DefaultBands(0.7).Category(1) || after.GetMatchScore() != 0.7 || after.Fit != "strong" {
		t.Errorf("fit after review: reviewed=%v %q, run=%v %q", after.GetReviewedScore(), after.ReviewedFit, after.GetMatchScore(), after.Fit)
	}
	// The member who submitted the posting is sent the record as the
	// model left it: the model's verdict, the model's numbers, and no
	// sign that anyone overrode anything.
	mine, err = get(as(f.ownerID, users.RoleMember), &v1.GetMatchRecordRequest{SubmissionId: sid})
	if err != nil {
		t.Fatalf("the owner's read after the override: %v", err)
	}
	m := mine.Record.Judgments[1]
	if m.EffectiveDetermination != "unresolved" || m.EffectiveBasis != "adjacent_domain" || m.Override != nil ||
		mine.Record.OverrideCount != 0 || mine.Record.Coverage.Evidenced != 0.4 || mine.Record.Coverage.Unresolved != 0.6 ||
		mine.Record.ReviewedScore != nil || mine.Record.ReviewedFit != "" {
		t.Errorf("the member's record must not move with an override: %+v count=%d coverage=%+v", m, mine.Record.OverrideCount, mine.Record.Coverage)
	}
	// An admin can ask for exactly that, to see what a recruiter sees.
	asMember, err := get(adminCtx, &v1.GetMatchRecordRequest{SubmissionId: sid, AsMember: true})
	if err != nil {
		t.Fatalf("the admin's read as a member: %v", err)
	}
	am := asMember.Record.Judgments[1]
	if am.Override != nil || am.EffectiveDetermination != "unresolved" || am.Evidence[0].Title != "" || am.Evidence[0].ChunkId != 0 ||
		asMember.Record.OverrideCount != 0 || asMember.Record.Coverage.Evidenced != 0.4 || asMember.CanOverride || len(asMember.OverrideChoices) != 0 ||
		asMember.Record.ReviewedScore != nil {
		t.Errorf("an admin reading as a member must get the member's record: %+v can=%v", am, asMember.CanOverride)
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

// fakeResumes stands in for the scorer: it records what it was asked to
// write and finishes the row when the test lets it, so the request, the
// busy refusal and the finished row can each be looked at.
type fakeResumes struct {
	repo    *hiring.Repo
	called  chan [2]string
	release chan struct{}
}

func (f *fakeResumes) CanWriteResume() bool { return true }

func (f *fakeResumes) WriteReviewResume(ctx context.Context, submissionID int64, runID, jdText string, hints prompts.Hints) {
	f.called <- [2]string{runID, hints.Role + "|" + strconv.Itoa(int(corpusscope.FromContext(ctx)))}
	<-f.release
	_ = f.repo.FinishReviewResume(ctx, runID, []byte(`{"experience":[]}`), "# Written after review", "test-model", "resume_tailor", 7)
	_ = f.repo.SetReviewResumePDF(ctx, runID, []byte("%PDF-1.7 after review"), 2)
	f.called <- [2]string{"done", ""}
}

// The résumé after review through the RPCs (owner's decision,
// 2026-11-13): offered only once a person's verdicts lift a run that
// finished below the gate to the gate, written for the owner alone,
// and never part of what the member is sent.
func TestReviewResumeAgainstPostgres(t *testing.T) {
	h, pool, ctx := hiringStack(t)
	// The gate is whatever is in force: a stored setting wins over the
	// fallback, so the test reads it and does not assume one. The run is
	// marked as having finished below it, which is what the offer reads.
	bands := jd.NewBandsStore(slog.New(slog.NewTextHandler(io.Discard, nil)), h.users, jd.DefaultBands(0.8))
	h.SetBands(bands)
	gate := bands.Get(ctx).Strong
	if gate <= 0.4 || gate > 1 {
		t.Skipf("the gate in force here is %.2f; this test needs one between 0.40 and 1.00", gate)
	}
	f := newRecordFixture(t, h, pool, ctx)
	if _, err := pool.Exec(ctx, `UPDATE jd_runs SET status = 'below_threshold', fit = 'possible', retrieval_scope = 'all' WHERE run_id = $1::uuid`, f.runID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE jd_submissions SET status = 'below_threshold' WHERE id = $1`, f.submissionID); err != nil {
		t.Fatal(err)
	}
	sid := strconv.FormatInt(f.submissionID, 10)
	adminCtx := WithCaller(ctx, &users.User{ID: f.adminID, Role: users.RoleAdmin})
	ownerCtx := WithCaller(ctx, &users.User{ID: f.ownerID, Role: users.RoleMember})
	read := func(c context.Context) *v1.GetMatchRecordResponse {
		resp, err := h.GetMatchRecord(c, connect.NewRequest(&v1.GetMatchRecordRequest{SubmissionId: sid}))
		if err != nil {
			t.Fatalf("GetMatchRecord: %v", err)
		}
		return resp.Msg
	}
	write := func() (*v1.WriteReviewResumeResponse, error) {
		resp, err := h.WriteReviewResume(adminCtx, connect.NewRequest(&v1.WriteReviewResumeRequest{RunId: f.runID}))
		if err != nil {
			return nil, err
		}
		return resp.Msg, nil
	}
	override := func(judgmentID, determination, basis string) {
		t.Helper()
		if _, err := h.OverrideJudgment(adminCtx, connect.NewRequest(&v1.OverrideJudgmentRequest{
			JudgmentId: judgmentID, Determination: determination, Basis: basis, Reason: "test " + determination, Role: "hiring_manager",
		})); err != nil {
			t.Fatalf("override to %s: %v", determination, err)
		}
	}

	// With no writer wired nothing is offered, and nothing can be asked.
	first := read(adminCtx)
	r2 := first.Record.Judgments[1].Id
	if first.CanWriteReviewResume || first.ReviewResumeNote != "" || first.ReviewResume != nil {
		t.Errorf("a record with no override is offered nothing and told nothing: %+v", first)
	}
	if _, err := write(); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("asking with no override: code %v, want failed_precondition", connect.CodeOf(err))
	}
	override(r2, "not_met", "scope_shortfall")
	if got := read(adminCtx); got.CanWriteReviewResume || got.ReviewResumeNote != "The résumé writer is not available here." {
		t.Errorf("with no writer wired: can=%v note=%q", got.CanWriteReviewResume, got.ReviewResumeNote)
	}

	fake := &fakeResumes{repo: h.repo, called: make(chan [2]string, 2), release: make(chan struct{})}
	h.SetReviewResumes(fake)

	// A review that leaves the score below the gate is offered nothing,
	// and told why: r1 met (2) of 5.
	wantNote := fmt.Sprintf("The score after review, 0.40, is below the résumé gate of %.2f, so no résumé is offered.", gate)
	if got := read(adminCtx); got.CanWriteReviewResume || got.ReviewResumeNote != wantNote {
		t.Errorf("below the gate after review: can=%v note=%q", got.CanWriteReviewResume, got.ReviewResumeNote)
	}
	if _, err := write(); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("asking below the gate: code %v, want failed_precondition", connect.CodeOf(err))
	}
	for name, c := range map[string]struct {
		run  string
		code connect.Code
	}{
		"a run id that is not a uuid": {"x", connect.CodeInvalidArgument},
		"a run with no record":        {"00000000-0000-0000-0000-00000000dead", connect.CodeNotFound},
	} {
		if _, err := h.WriteReviewResume(adminCtx, connect.NewRequest(&v1.WriteReviewResumeRequest{RunId: c.run})); connect.CodeOf(err) != c.code {
			t.Errorf("%s: code %v, want %v", name, connect.CodeOf(err), c.code)
		}
	}

	// The person's verdict lifts it: both required rows now earn their
	// weight, 1.00, which is at or above any gate.
	override(r2, "met_equivalent", "adjacent_domain")
	offered := read(adminCtx)
	if !offered.CanWriteReviewResume || offered.ReviewResumeNote != "" || offered.Record.GetReviewedScore() != 1 {
		t.Fatalf("at the gate after review: can=%v note=%q reviewed=%v", offered.CanWriteReviewResume, offered.ReviewResumeNote, offered.Record.GetReviewedScore())
	}
	// The member is told nothing of it, before or after.
	if mine := read(ownerCtx); mine.CanWriteReviewResume || mine.ReviewResumeNote != "" || mine.ReviewResume != nil {
		t.Errorf("a member's response must not name a résumé after review: %+v", mine)
	}

	asked, err := write()
	if err != nil {
		t.Fatalf("WriteReviewResume: %v", err)
	}
	if rr := asked.ReviewResume; rr == nil || rr.Status != hiring.ReviewResumeWriting || rr.RunId != f.runID || rr.ReviewedScore != 1 || rr.OverrideCount != 1 || rr.Markdown != "" {
		t.Errorf("the row as asked for: %+v", asked.ReviewResume)
	}
	select {
	case call := <-fake.called:
		// Written for the run asked about, under the scope that run read.
		if call[0] != f.runID || call[1] != "Controls Engineer|"+strconv.Itoa(int(corpusscope.All)) {
			t.Errorf("the writer was called with %v", call)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the writer was never started")
	}
	writing := read(adminCtx)
	if writing.ReviewResume.GetStatus() != hiring.ReviewResumeWriting || writing.CanWriteReviewResume {
		t.Errorf("while it is being written: status=%q can=%v", writing.ReviewResume.GetStatus(), writing.CanWriteReviewResume)
	}
	if _, err := write(); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("asking again while one is being written: code %v, want failed_precondition", connect.CodeOf(err))
	}
	close(fake.release)
	select {
	case <-fake.called:
	case <-time.After(5 * time.Second):
		t.Fatal("the writer never finished")
	}

	ready := read(adminCtx)
	rr := ready.ReviewResume
	if rr == nil || rr.Status != hiring.ReviewResumeReady || rr.Markdown != "# Written after review" || rr.FinishedAt == nil ||
		rr.DownloadPath != ReviewResumeDownloadPath(f.runID) || rr.Model != "test-model" || !ready.CanWriteReviewResume {
		t.Errorf("the résumé when ready, as the admin reads it: %+v can=%v", rr, ready.CanWriteReviewResume)
	}
	// The member's side of the posting has not moved: no résumé on the
	// submission, the status as the run left it, and nothing in the
	// record's response.
	if mine := read(ownerCtx); mine.ReviewResume != nil || mine.CanWriteReviewResume {
		t.Errorf("a member must never be sent the résumé after review: %+v", mine.ReviewResume)
	}
	jdh := NewJd(slog.New(slog.NewTextHandler(io.Discard, nil)), h.users, nil, nil, 0, nil)
	jdh.SetHiring(h.repo)
	result, err := jdh.GetJdResult(ownerCtx, connect.NewRequest(&v1.GetJdResultRequest{SubmissionId: sid}))
	if err != nil {
		t.Fatalf("GetJdResult: %v", err)
	}
	if result.Msg.ResumeMarkdown != "" || result.Msg.GeneratedResumeUrl != "" || result.Msg.Status != v1.JdStatus_JD_STATUS_BELOW_THRESHOLD {
		t.Errorf("the member's result must be as the run left it: status=%v markdown=%q url=%q", result.Msg.Status, result.Msg.ResumeMarkdown, result.Msg.GeneratedResumeUrl)
	}
	// The PDF route answers an admin's session and nothing else; with no
	// session lookup wired it is a plain not-found.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, ReviewResumeDownloadPath(f.runID), nil)
	req.SetPathValue("file", f.runID+".pdf")
	jdh.ServeReviewResumePDF(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("the PDF without an admin session: %d, want 404", rec.Code)
	}

	// A run that wrote a résumé of its own is offered none after review.
	if _, err := pool.Exec(ctx, `UPDATE jd_runs SET status = 'ready', resume_generated = true WHERE run_id = $1::uuid`, f.runID); err != nil {
		t.Fatal(err)
	}
	if got := read(adminCtx); got.CanWriteReviewResume || got.ReviewResumeNote != "This run wrote a résumé of its own, so none is written after review." {
		t.Errorf("a run with its own résumé: can=%v note=%q", got.CanWriteReviewResume, got.ReviewResumeNote)
	}
}
