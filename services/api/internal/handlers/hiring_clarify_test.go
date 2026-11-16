package handlers

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/hiring"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// What a reader of a record is sent about a follow-up: the question and
// the candidate's answer to every reader, the ids to the hiring side
// only, and never a fit while the candidate has been asked something.
func TestFollowUpProtoByAudience(t *testing.T) {
	at := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	j := hiring.RecordJudgment{
		ID: "j2", RequirementID: "q2", LocalID: "r2",
		Determination: hiring.DetMetEquivalent, Basis: hiring.BasisAdjacentDomain,
		EffectiveDetermination: hiring.DetMetEquivalent, EffectiveBasis: hiring.BasisAdjacentDomain,
		SelfAttested: true,
		Evidence: []hiring.RecordEvidence{
			{ID: "e9", SourceKind: hiring.SourceCandidateResponse, Statement: "The candidate's own answer."},
			{ID: "e1", SourceKind: "corpus_document", Title: "A private note", ChunkID: 41},
		},
		FollowUp: &hiring.RecordFollowUp{
			ItemID: "item-1", RequestID: "req-1", Status: hiring.ItemAnswered, Question: "Which of your roles?",
			Statement: "The candidate's own answer.", AnsweredAt: &at, AskedAttempt: 1, AskedBy: hiring.AskedBySystem,
		},
	}
	member := judgmentProto(j, recordAudience{})
	if !member.SelfAttested || member.FollowUp.GetStatement() == "" || member.FollowUp.GetQuestion() == "" || member.FollowUp.GetAskedAttempt() != 1 ||
		member.FollowUp.GetRequestId() != "" || member.FollowUp.GetItemId() != "" {
		t.Errorf("a member is sent the question and the answer and no ids: %+v", member.FollowUp)
	}
	if member.Evidence[0].Statement == "" || member.Evidence[1].Statement != "" || member.Evidence[1].Title != "" {
		t.Errorf("the answer's citation carries its statement; a private document's carries nothing: %+v", member.Evidence)
	}
	admin := judgmentProto(j, recordAudience{admin: true})
	if admin.FollowUp.GetRequestId() != "req-1" || admin.FollowUp.GetItemId() != "item-1" {
		t.Errorf("the hiring side is sent the ids: %+v", admin.FollowUp)
	}

	limit := at.Add(72 * time.Hour)
	score := 0.9
	rec := hiring.Record{
		RunID: "run", RunStatus: hiring.RunAwaitingCandidate, FormulaVersion: hiring.FormulaV4, HasNumbers: true,
		Fit: "strong", MatchScore: &score,
		Requirements: []hiring.Requirement{{ID: "q2", LocalID: "r2", Weight: 3, Class: hiring.ClassRequired, Included: true}},
		Judgments:    []hiring.RecordJudgment{j},
		Clarification: &hiring.RecordClarification{
			RequestID: "req-1", Status: hiring.RequestSent, Held: true, Asked: 2, Pending: 1, Answered: 1,
			GeneratedAt: at, ExpiresAt: &limit, AskedAttempt: 1,
		},
	}
	for _, aud := range []recordAudience{{}, {admin: true}} {
		out := recordProto(rec, "A role", "", aud)
		if !out.AwaitingCandidate || out.Fit != "" || out.MatchScore != nil {
			t.Errorf("no reader is sent a fit while the candidate has been asked something (admin=%v): fit=%q score=%v", aud.admin, out.Fit, out.MatchScore)
		}
		c := out.Clarification
		if c == nil || c.Status != hiring.RequestSent || !c.Held || c.Asked != 2 || c.Pending != 1 || c.Answered != 1 || c.ExpiresAt == nil {
			t.Fatalf("the questionnaire on the record (admin=%v): %+v", aud.admin, c)
		}
		if (c.RequestId != "") != aud.admin {
			t.Errorf("the questionnaire's id is the hiring side's (admin=%v): %q", aud.admin, c.RequestId)
		}
	}
}

// closingRuns stands in for the scorer: it records the questionnaires
// whose closing run was asked for.
type closingRuns struct {
	mu   sync.Mutex
	seen []string
	done chan struct{}
}

func (c *closingRuns) CloseClarification(_ context.Context, q hiring.ClarificationRequest, _ int64) {
	c.mu.Lock()
	c.seen = append(c.seen, q.ID)
	c.mu.Unlock()
	c.done <- struct{}{}
}

// The candidate's side of the loop through the RPCs: questions at the
// candidate's request, an answer checked against what was offered, the
// submit that starts the closing run, and a withdrawal.
func TestClarificationAgainstPostgres(t *testing.T) {
	h, pool, ctx := hiringStack(t)
	runs := &closingRuns{done: make(chan struct{}, 4)}
	h.SetClarifications(runs)
	f := newRecordFixture(t, h, pool, ctx)
	sid := strconv.FormatInt(f.submissionID, 10)
	admin := WithCaller(ctx, &users.User{ID: f.adminID, Role: users.RoleAdmin})
	owner := WithCaller(ctx, &users.User{ID: f.ownerID, Role: users.RoleMember})

	// One of the candidate's own roles to point at.
	var roleDoc int64
	if err := pool.QueryRow(ctx, `
    INSERT INTO corpus_documents (source_kind, source_path, title, visibility, meta)
    VALUES ('role', 'test/' || gen_random_uuid()::text, 'Director of Engineering (handler test)', 'public',
            '{"organization": "Test Distilling", "start": "2017", "end": "2022"}'::jsonb)
    RETURNING id`).Scan(&roleDoc); err != nil {
		t.Fatalf("make role document: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM corpus_documents WHERE id = $1`, roleDoc) })
	roleRef := "corpus_document:" + strconv.FormatInt(roleDoc, 10)

	// Before anything is asked, the record offers one row to ask about:
	// r2. r1 is met and r3 is not judged from documents.
	before, err := h.GetMatchRecord(admin, connect.NewRequest(&v1.GetMatchRecordRequest{SubmissionId: sid}))
	if err != nil || before.Msg.AskableCount != 1 || before.Msg.Record.Clarification != nil || before.Msg.Record.Judgments[1].FollowUp != nil {
		t.Fatalf("the record before a question: askable=%d err=%v", before.Msg.GetAskableCount(), err)
	}

	for name, bad := range map[string]string{"not a number": "abc", "absent": "999999999"} {
		if _, err := h.RequestClarification(admin, connect.NewRequest(&v1.RequestClarificationRequest{SubmissionId: bad})); err == nil {
			t.Errorf("a submission id that is %s must be refused", name)
		}
	}
	asked, err := h.RequestClarification(admin, connect.NewRequest(&v1.RequestClarificationRequest{SubmissionId: sid}))
	if err != nil {
		t.Fatalf("RequestClarification: %v", err)
	}
	q := asked.Msg.Request
	if asked.Msg.Added != 1 || q.Status != hiring.RequestSent || q.Held || q.AskedBy != hiring.AskedByCandidate || q.SubmissionId != sid ||
		q.ItemCount != 1 || q.PendingCount != 1 || len(q.Items) != 1 || q.ExpiresAt != nil || q.RoleTitle != "Controls Engineer" {
		t.Fatalf("the questionnaire as asked: added=%d %+v", asked.Msg.Added, q)
	}
	it := q.Items[0]
	if it.RequirementLocalId != "r2" || it.SourceQuote != "Front-end loading" || it.Question == "" || it.AnswerType != hiring.AnswerEvidencePointer ||
		!it.FreeTextRequired || it.FreeTextMax != hiring.MaxAnswerFreeText || it.Answer.GetStatus() != hiring.ItemPending || len(it.Fields) != 1 {
		t.Fatalf("the question as the candidate is sent it: %+v", it)
	}
	offered := false
	for _, o := range it.Fields[0].Options {
		if o.Value == roleRef && o.Source == hiring.OptionOwnCorpus && strings.Contains(o.Label, "Test Distilling") {
			offered = true
		}
	}
	if !offered {
		t.Errorf("the candidate's own role must be among the choices: %+v", it.Fields[0].Options)
	}
	if _, err := h.RequestClarification(admin, connect.NewRequest(&v1.RequestClarificationRequest{SubmissionId: sid})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("asking again with nothing left: code %v, want failed_precondition", connect.CodeOf(err))
	}

	// The record now: nothing left to ask, the hiring side sees the
	// questionnaire's id, the recruiter the question and no id. The fit
	// stands: the candidate asked, the review did not.
	theirs, _ := h.GetMatchRecord(admin, connect.NewRequest(&v1.GetMatchRecordRequest{SubmissionId: sid}))
	if theirs.Msg.AskableCount != 0 || theirs.Msg.Record.Clarification.GetRequestId() != q.Id || theirs.Msg.Record.AwaitingCandidate ||
		theirs.Msg.Record.Fit != "strong" || theirs.Msg.Record.Judgments[1].FollowUp.GetItemId() != it.Id {
		t.Errorf("the admin's record with a question open: askable=%d %+v", theirs.Msg.AskableCount, theirs.Msg.Record.Clarification)
	}
	mine, _ := h.GetMatchRecord(owner, connect.NewRequest(&v1.GetMatchRecordRequest{SubmissionId: sid}))
	if f := mine.Msg.Record.Judgments[1].FollowUp; f.GetStatus() != hiring.ItemPending || f.GetQuestion() != it.Question || f.GetItemId() != "" ||
		mine.Msg.Record.Clarification.GetRequestId() != "" || mine.Msg.AskableCount != 0 {
		t.Errorf("the recruiter's record with a question open: %+v", f)
	}

	// Answers: checked against what was offered.
	answer := func(a *v1.ClarificationAnswer) (*v1.AnswerClarificationItemResponse, error) {
		resp, err := h.AnswerClarificationItem(admin, connect.NewRequest(&v1.AnswerClarificationItemRequest{ClarificationItemId: it.Id, Answer: a}))
		if err != nil {
			return nil, err
		}
		return resp.Msg, nil
	}
	picked := map[string]*v1.StringList{"engagements": {Values: []string{roleRef}}}
	for name, bad := range map[string]*v1.ClarificationAnswer{
		"a pointer with nothing said":    {Status: hiring.ItemAnswered, Values: picked},
		"a choice that was not offered":  {Status: hiring.ItemAnswered, Values: map[string]*v1.StringList{"engagements": {Values: []string{"corpus_document:0"}}}, FreeText: "x"},
		"a status that is not an answer": {Status: "pending"},
	} {
		if _, err := answer(bad); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: code %v, want invalid_argument", name, connect.CodeOf(err))
		}
	}
	if _, err := h.SubmitClarificationAnswers(admin, connect.NewRequest(&v1.SubmitClarificationAnswersRequest{ClarificationRequestId: q.Id})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("submitting with a question unanswered: code %v, want invalid_argument", connect.CodeOf(err))
	}
	saved, err := answer(&v1.ClarificationAnswer{Status: hiring.ItemAnswered, Values: picked, FreeText: "Led FEL-1 to FEL-3 for the expansion."})
	if err != nil {
		t.Fatalf("AnswerClarificationItem: %v", err)
	}
	if saved.PendingCount != 0 || saved.Item.Answer.Status != hiring.ItemAnswered || saved.Item.AnsweredAt == nil ||
		saved.Item.Answer.Values["engagements"].GetValues()[0] != roleRef ||
		!strings.Contains(saved.Item.Statement, "Test Distilling") || !strings.Contains(saved.Item.Statement, "The candidate adds: Led FEL-1 to FEL-3") {
		t.Fatalf("the answer as saved: %+v", saved.Item)
	}
	got, err := h.GetClarificationRequest(admin, connect.NewRequest(&v1.GetClarificationRequestRequest{ClarificationRequestId: q.Id}))
	if err != nil || got.Msg.Request.PendingCount != 0 || got.Msg.Request.Items[0].Answer.FreeText == "" {
		t.Fatalf("GetClarificationRequest: %+v (err %v)", got, err)
	}
	if _, err := h.GetClarificationRequest(admin, connect.NewRequest(&v1.GetClarificationRequestRequest{ClarificationRequestId: "00000000-0000-0000-0000-00000000dead"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a questionnaire that is absent: code %v, want not_found", connect.CodeOf(err))
	}
	list, err := h.ListClarificationRequests(admin, connect.NewRequest(&v1.ListClarificationRequestsRequest{OpenOnly: true}))
	found := false
	for _, x := range list.Msg.GetRequests() {
		if x.Id == q.Id {
			found = x.ItemCount == 1 && x.PendingCount == 0 && len(x.Items) == 0
		}
	}
	if err != nil || !found {
		t.Errorf("ListClarificationRequests: the open questionnaire with its counts and no questions (err %v)", err)
	}

	// Submitted: closed to answers, the posting marked as scoring, and
	// the closing run asked for once.
	sent, err := h.SubmitClarificationAnswers(admin, connect.NewRequest(&v1.SubmitClarificationAnswersRequest{ClarificationRequestId: q.Id}))
	if err != nil || !sent.Msg.RunStarted || sent.Msg.Request.Status != hiring.RequestAnswered || sent.Msg.Request.AnsweredAt == nil {
		t.Fatalf("SubmitClarificationAnswers: %+v (err %v)", sent, err)
	}
	select {
	case <-runs.done:
	case <-time.After(5 * time.Second):
		t.Fatal("the closing run was never asked for")
	}
	runs.mu.Lock()
	if len(runs.seen) != 1 || runs.seen[0] != q.Id {
		t.Errorf("closing runs asked for: %v", runs.seen)
	}
	runs.mu.Unlock()
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM jd_submissions WHERE id = $1`, f.submissionID).Scan(&status); err != nil || status != "scoring" {
		t.Errorf("the posting after the answers were submitted: %s (err %v)", status, err)
	}
	if _, err := h.SubmitClarificationAnswers(admin, connect.NewRequest(&v1.SubmitClarificationAnswersRequest{ClarificationRequestId: q.Id})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("submitting twice: code %v, want failed_precondition", connect.CodeOf(err))
	}
	if _, err := answer(&v1.ClarificationAnswer{Status: hiring.ItemDeclined}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("an answer after the submit: code %v, want failed_precondition", connect.CodeOf(err))
	}
	// A posting being scored takes no new questions.
	if _, err := h.RequestClarification(admin, connect.NewRequest(&v1.RequestClarificationRequest{SubmissionId: sid})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("asking while the posting is being scored: code %v, want failed_precondition", connect.CodeOf(err))
	}

	// Withdrawn: the statement stops being evidence.
	gone, err := h.WithdrawClarificationAnswer(admin, connect.NewRequest(&v1.WithdrawClarificationAnswerRequest{ClarificationItemId: it.Id}))
	if err != nil || gone.Msg.Item.Answer.Status != hiring.ItemWithdrawn || gone.Msg.Item.Statement != "" {
		t.Fatalf("WithdrawClarificationAnswer: %+v (err %v)", gone, err)
	}
	if _, err := h.WithdrawClarificationAnswer(admin, connect.NewRequest(&v1.WithdrawClarificationAnswerRequest{ClarificationItemId: it.Id})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("withdrawing twice: code %v, want invalid_argument", connect.CodeOf(err))
	}

	// The settings: the switch ships off with three days, a write that
	// sets only the fraction leaves both alone, and the limit is
	// bounded.
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM app_settings WHERE key = $1`, hiring.SettingKey) })
	_, _ = pool.Exec(ctx, `DELETE FROM app_settings WHERE key = $1`, hiring.SettingKey)
	fresh := hiring.NewSettingsStore(h.log, h.users)
	h.settings = fresh
	settings := func() *v1.HiringSettings {
		resp, err := h.GetHiringSettings(admin, connect.NewRequest(&v1.GetHiringSettingsRequest{}))
		if err != nil {
			t.Fatalf("GetHiringSettings: %v", err)
		}
		return resp.Msg.Settings
	}
	if s := settings(); s.GetClarificationAuto() || s.ClarificationDays != hiring.DefaultClarificationDays {
		t.Errorf("the loop ships switched off with a three-day limit: %+v", s)
	}
	on := true
	if _, err := h.SetHiringSettings(admin, connect.NewRequest(&v1.SetHiringSettingsRequest{Settings: &v1.HiringSettings{
		PreferredFraction: 0.25, ClarificationAuto: &on, ClarificationDays: 5}})); err != nil {
		t.Fatalf("SetHiringSettings: %v", err)
	}
	if _, err := h.SetHiringSettings(admin, connect.NewRequest(&v1.SetHiringSettingsRequest{Settings: &v1.HiringSettings{PreferredFraction: 0.5}})); err != nil {
		t.Fatalf("SetHiringSettings (fraction only): %v", err)
	}
	if s := settings(); !s.GetClarificationAuto() || s.ClarificationDays != 5 || s.PreferredFraction != 0.5 {
		t.Errorf("a write of the fraction alone must leave the loop's settings: %+v", s)
	}
	if _, err := h.SetHiringSettings(admin, connect.NewRequest(&v1.SetHiringSettingsRequest{Settings: &v1.HiringSettings{
		PreferredFraction: 0.5, ClarificationDays: 30}})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a limit of thirty days: code %v, want invalid_argument", connect.CodeOf(err))
	}
}
