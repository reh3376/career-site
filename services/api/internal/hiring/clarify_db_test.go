package hiring

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestClarificationRows walks the loop's rows (FR-HF-13, 14, 22): a
// questionnaire generated from a run's unresolved judgments, an answer
// that becomes evidence, the questionnaire closed, an answer withdrawn,
// a time limit reached, and a posting's open questions superseded by a
// new run.
func TestClarificationRows(t *testing.T) {
	r, ctx := testRepo(t)
	now := time.Now().UTC().Truncate(time.Second)

	// One of the candidate's own roles, as the corpus holds it after
	// ingest keeps a role file's front matter.
	var roleDoc int64
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO corpus_documents (source_kind, source_path, title, visibility, meta)
    VALUES ('role', 'test/' || gen_random_uuid()::text, 'Director of Engineering (test)', 'public',
            '{"organization": "Test Distilling", "start": "2017", "end": "2022", "order": "20"}'::jsonb)
    RETURNING id`).Scan(&roleDoc); err != nil {
		t.Fatalf("make role document: %v", err)
	}
	t.Cleanup(func() { _, _ = r.pool.Exec(ctx, `DELETE FROM corpus_documents WHERE id = $1`, roleDoc) })
	engagements, err := r.Engagements(ctx)
	if err != nil {
		t.Fatalf("Engagements: %v", err)
	}
	var mine Engagement
	for _, e := range engagements {
		if e.Title == "Director of Engineering (test)" {
			mine = e
		}
	}
	if mine.Ref == "" || mine.Kind != "role" || mine.Organization != "Test Distilling" || mine.Start != 2017 || mine.End != 2022 ||
		mine.Label() != "Director of Engineering (test), Test Distilling, 2017 to 2022" {
		t.Fatalf("the role as an engagement: %+v", mine)
	}

	f := newFixture(t, r, ctx, 0.7)
	t.Cleanup(func() {
		_, _ = r.pool.Exec(ctx, `DELETE FROM decision_log WHERE ref_kind = 'jd_submission' AND ref_id = $1`, f.submissionID)
	})
	recordRun(t, r, ctx, f, f.runID, 1)
	rec, err := r.GetRecord(ctx, f.runID)
	if err != nil {
		t.Fatalf("GetRecord: %v", err)
	}
	r2 := judgmentByLocal(t, rec, "r2")

	// Unprompted, the run's one lexical unresolved row is asked about:
	// r2, adjacent domain, equivalence unconfirmed, as a pointer to the
	// candidate's own work.
	limit := now.Add(72 * time.Hour)
	req, added, err := r.GenerateClarification(ctx, GenerateInput{RunID: f.runID, Held: true, ExpiresAt: &limit})
	if err != nil {
		t.Fatalf("GenerateClarification: %v", err)
	}
	if added != 1 || len(req.Items) != 1 || !req.Open() || !req.Held || req.Round != 1 || req.SubmissionID != f.submissionID ||
		req.ExpiresAt == nil || req.Pending() != 1 || req.AskedBy != AskedBySystem || req.Attempt != 1 {
		t.Fatalf("the request as generated: added=%d %+v", added, req)
	}
	it := req.Items[0]
	if it.LocalID != "r2" || it.JudgmentID != r2.ID || it.RequirementID != r2.RequirementID || it.UncertaintyCode != UncEquivalenceUnconfirmed ||
		it.AnswerType != AnswerEvidencePointer || it.TemplateVersion != QuestionTemplateVersion || it.Answer.Status != ItemPending ||
		it.QuestionText != QuestionText(AnswerEvidencePointer, "outcome") || it.SourceQuote != "front-end loading" {
		t.Fatalf("the item as generated: %+v", it)
	}
	offered := false
	for _, o := range it.Fields[0].Options {
		if o.Value == mine.Ref && o.Source == OptionOwnCorpus && o.Start == 2017 {
			offered = true
		}
	}
	if !offered {
		t.Errorf("the candidate's own role must be among the choices: %+v", it.Fields[0].Options)
	}
	var open, asked int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM v_clarification_open WHERE request_id = $1::uuid`, req.ID).Scan(&open); err != nil || open != 1 {
		t.Errorf("v_clarification_open: %d (err %v)", open, err)
	}
	if err := r.pool.QueryRow(ctx, `
    SELECT count(*) FROM decision_log WHERE run_id = $1::uuid AND kind = $2 AND output->>'action' = 'asked' AND reviewed_at IS NOT NULL`,
		f.runID, DecisionKindClarification).Scan(&asked); err != nil || asked != 1 {
		t.Errorf("a question is logged like a decision: %d rows (err %v)", asked, err)
	}
	// Asked once: neither a second unprompted pass nor the candidate's
	// own request adds a second question about the same row.
	if _, _, err := r.GenerateClarification(ctx, GenerateInput{RunID: f.runID, Held: true}); !errors.Is(err, ErrNothingToAsk) {
		t.Errorf("a second unprompted pass: %v, want nothing to ask", err)
	}
	if _, _, err := r.GenerateClarification(ctx, GenerateInput{RunID: f.runID, All: true, AskedBy: AskedByCandidate}); !errors.Is(err, ErrNothingToAsk) {
		t.Errorf("the candidate's own request with nothing left: %v, want nothing to ask", err)
	}
	if _, _, err := r.GenerateClarification(ctx, GenerateInput{RunID: "00000000-0000-0000-0000-00000000dead"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("a run with no record: %v, want not found", err)
	}

	// Sending before every question is dealt with is refused.
	if _, err := r.SubmitClarificationRequest(ctx, req.ID, now); !errors.Is(err, ErrInvalidAnswer) {
		t.Errorf("sending with a question pending: %v, want invalid", err)
	}
	// A pointer with nothing said is not an answer.
	if _, err := r.SaveClarificationAnswer(ctx, it.ID, Answer{Status: ItemAnswered, Values: map[string]any{"engagements": []string{mine.Ref}}}, now); !errors.Is(err, ErrInvalidAnswer) {
		t.Errorf("a pointer with no description: %v, want invalid", err)
	}
	saved, err := r.SaveClarificationAnswer(ctx, it.ID, Answer{Status: ItemAnswered,
		Values: map[string]any{"engagements": []string{mine.Ref}}, FreeText: "Led FEL-1 to FEL-3 for the expansion."}, now)
	if err != nil {
		t.Fatalf("SaveClarificationAnswer: %v", err)
	}
	if saved.Answer.Status != ItemAnswered || saved.EvidenceItemID == "" {
		t.Fatalf("the item as answered: %+v", saved)
	}
	var kind, text, visibility string
	if err := r.pool.QueryRow(ctx, `
    SELECT source_kind, text, visibility FROM evidence_items WHERE id = $1::uuid AND clarification_item_id = $2::uuid AND withdrawn_at IS NULL`,
		saved.EvidenceItemID, it.ID).Scan(&kind, &text, &visibility); err != nil {
		t.Fatalf("the answer's evidence item: %v", err)
	}
	if kind != SourceCandidateResponse || visibility != "employer_scoped" || !strings.HasPrefix(text, "The candidate's own answer to a follow-up question") ||
		!strings.Contains(text, "Director of Engineering (test), Test Distilling, 2017 to 2022") || !strings.Contains(text, "The candidate adds: Led FEL-1 to FEL-3 for the expansion.") {
		t.Errorf("the evidence item: kind=%s visibility=%s text=%q", kind, visibility, text)
	}
	// Saved again, the answer and its evidence are replaced, not doubled.
	again, err := r.SaveClarificationAnswer(ctx, it.ID, Answer{Status: ItemAnswered,
		Values: map[string]any{"engagements": []string{mine.Ref}}, FreeText: "Owned front-end loading through FEL-3."}, now)
	if err != nil || again.EvidenceItemID != saved.EvidenceItemID {
		t.Fatalf("saving again must keep the one evidence item: %s then %s (err %v)", saved.EvidenceItemID, again.EvidenceItemID, err)
	}
	standing, err := r.StandingAnswers(ctx, []string{r2.RequirementID, "00000000-0000-0000-0000-00000000dead"})
	if err != nil || len(standing) != 1 {
		t.Fatalf("StandingAnswers: %d (err %v)", len(standing), err)
	}
	if a := standing[r2.RequirementID]; a.ItemID != it.ID || a.AnswerType != AnswerEvidencePointer || a.EvidenceItemID != saved.EvidenceItemID ||
		!strings.Contains(a.Statement, "Owned front-end loading through FEL-3.") || a.Answer.Status != ItemAnswered || len(a.Fields) == 0 {
		t.Errorf("the standing answer: %+v", a)
	}

	sent, err := r.SubmitClarificationRequest(ctx, req.ID, now)
	if err != nil || sent.Status != RequestAnswered || sent.AnsweredAt == nil || sent.Open() {
		t.Fatalf("SubmitClarificationRequest: %+v (err %v)", sent, err)
	}
	if _, err := r.SaveClarificationAnswer(ctx, it.ID, Answer{Status: ItemDeclined}, now); !errors.Is(err, ErrClarificationClosed) {
		t.Errorf("an answer after the questions were sent: %v, want closed", err)
	}
	if _, _, err := r.GenerateClarification(ctx, GenerateInput{RunID: f.runID, All: true}); !errors.Is(err, ErrNothingToAsk) && !errors.Is(err, ErrClarificationClosed) {
		t.Errorf("a second round on a closed request: %v", err)
	}
	if err := r.CloseClarificationRequest(ctx, req.ID, ClosedAnswered, f.runID, now); err != nil {
		t.Fatalf("CloseClarificationRequest: %v", err)
	}
	closed, _ := r.GetClarificationRequest(ctx, req.ID)
	if closed.ClosedAt == nil || closed.ClosedReason != ClosedAnswered || closed.ClosingRunID != f.runID || closed.Items[0].RejudgeRunID != f.runID {
		t.Errorf("the request as closed: %+v", closed)
	}
	if byRun, ok, err := r.ClarificationForRun(ctx, f.runID); err != nil || !ok || byRun.ID != req.ID {
		t.Errorf("ClarificationForRun: ok=%v err=%v", ok, err)
	}

	// Withdrawn: the evidence is soft-deleted and a judgment that cited
	// it is stale.
	if _, err := r.pool.Exec(ctx, `UPDATE judgments SET evidence_ids = evidence_ids || $2::uuid WHERE id = $1::uuid`, r2.ID, saved.EvidenceItemID); err != nil {
		t.Fatal(err)
	}
	gone, stale, err := r.WithdrawClarificationAnswer(ctx, it.ID, now)
	if err != nil || gone.Answer.Status != ItemWithdrawn || stale != 1 {
		t.Fatalf("WithdrawClarificationAnswer: %+v stale=%d err=%v", gone.Answer, stale, err)
	}
	rec, _ = r.GetRecord(ctx, f.runID)
	if !judgmentByLocal(t, rec, "r2").Stale {
		t.Error("a judgment that cited a withdrawn answer must be stale")
	}
	if standing, _ := r.StandingAnswers(ctx, []string{r2.RequirementID}); len(standing) != 0 {
		t.Errorf("a withdrawn answer is not a standing one: %+v", standing)
	}
	if _, _, err := r.WithdrawClarificationAnswer(ctx, it.ID, now); !errors.Is(err, ErrInvalidAnswer) {
		t.Errorf("withdrawing twice: %v, want invalid", err)
	}

	// A second posting: the two ways out, the reminder and the limit.
	g := newFixture(t, r, ctx, 0.7)
	t.Cleanup(func() {
		_, _ = r.pool.Exec(ctx, `DELETE FROM decision_log WHERE ref_kind = 'jd_submission' AND ref_id = $1`, g.submissionID)
	})
	recordRun(t, r, ctx, g, g.runID, 1)
	soon := now.Add(72 * time.Hour)
	held, _, err := r.GenerateClarification(ctx, GenerateInput{RunID: g.runID, Held: true, ExpiresAt: &soon})
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	// The same requirement wording is the same requirement version only
	// within a role; this posting has its own and is asked afresh.
	na, err := r.SaveClarificationAnswer(ctx, held.Items[0].ID, Answer{Status: ItemNotApplicable}, now)
	if err != nil || na.EvidenceItemID == "" {
		t.Fatalf("\"I have not done this\" is a statement and becomes evidence: %+v err=%v", na, err)
	}
	declined, err := r.SaveClarificationAnswer(ctx, held.Items[0].ID, Answer{Status: ItemDeclined}, now)
	if err != nil || declined.EvidenceItemID != "" {
		t.Fatalf("a declined item carries no evidence: %+v err=%v", declined, err)
	}
	var withdrawn bool
	if err := r.pool.QueryRow(ctx, `SELECT withdrawn_at IS NOT NULL FROM evidence_items WHERE id = $1::uuid`, na.EvidenceItemID).Scan(&withdrawn); err != nil || !withdrawn {
		t.Errorf("declining after answering withdraws the earlier statement: %v (err %v)", withdrawn, err)
	}
	if _, err := r.SaveClarificationAnswer(ctx, held.Items[0].ID, Answer{Status: ItemPending}, now); !errors.Is(err, ErrInvalidAnswer) {
		t.Errorf("an answer cannot be saved back to pending: %v", err)
	}
	// Reminded once, a day before the limit, and only while a question
	// is still pending.
	dayBefore := soon.Add(-23 * time.Hour)
	heldIn := func(list []ClarificationRequest) int {
		n := 0
		for _, q := range list {
			if q.ID == held.ID {
				n++
			}
		}
		return n
	}
	if got, err := r.ClarificationReminders(ctx, dayBefore, 24*time.Hour); err != nil || heldIn(got) != 0 {
		t.Errorf("no reminder for a request with nothing pending: %d (err %v)", heldIn(got), err)
	}
	if _, err := r.pool.Exec(ctx, `UPDATE clarification_items SET answer_status = 'pending' WHERE id = $1::uuid`, held.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	if early, _ := r.ClarificationReminders(ctx, now, 24*time.Hour); heldIn(early) != 0 {
		t.Errorf("no reminder three days before the limit: %d", heldIn(early))
	}
	reminded, err := r.ClarificationReminders(ctx, dayBefore, 24*time.Hour)
	if err != nil || heldIn(reminded) != 1 {
		t.Fatalf("ClarificationReminders: %d of %d (err %v)", heldIn(reminded), len(reminded), err)
	}
	if again, _ := r.ClarificationReminders(ctx, dayBefore, 24*time.Hour); heldIn(again) != 0 {
		t.Errorf("a request is reminded once: %d", heldIn(again))
	}
	// Not yet due, then due.
	if due, _ := r.ExpireClarificationRequests(ctx, now); len(due) != 0 {
		t.Errorf("a request inside its limit must not expire: %d", len(due))
	}
	due, err := r.ExpireClarificationRequests(ctx, soon.Add(time.Minute))
	if err != nil {
		t.Fatalf("ExpireClarificationRequests: %v", err)
	}
	// Found by id: the call closes every request past its limit, and a
	// shared database may hold someone else's.
	var mineDue *ClarificationRequest
	for i := range due {
		if due[i].ID == held.ID {
			mineDue = &due[i]
		}
	}
	if mineDue == nil || mineDue.RunID != g.runID || mineDue.SubmissionID != g.submissionID {
		t.Fatalf("ExpireClarificationRequests: %+v", due)
	}
	expired, _ := r.GetClarificationRequest(ctx, held.ID)
	if expired.Status != RequestExpired || expired.ClosedReason != ClosedExpired || expired.Open() {
		t.Errorf("the request after its limit: %+v", expired)
	}

	// A third posting: a new full run supersedes the open questions.
	h := newFixture(t, r, ctx, 0.7)
	t.Cleanup(func() {
		_, _ = r.pool.Exec(ctx, `DELETE FROM decision_log WHERE ref_kind = 'jd_submission' AND ref_id = $1`, h.submissionID)
	})
	recordRun(t, r, ctx, h, h.runID, 1)
	openReq, _, err := r.GenerateClarification(ctx, GenerateInput{RunID: h.runID, All: true, AskedBy: AskedByCandidate})
	if err != nil || openReq.Held || openReq.AskedBy != AskedByCandidate {
		t.Fatalf("a request the candidate asked for is not held: %+v (err %v)", openReq, err)
	}
	if n, err := r.SupersedeClarifications(ctx, h.submissionID, now); err != nil || n != 1 {
		t.Fatalf("SupersedeClarifications: %d (err %v)", n, err)
	}
	superseded, _ := r.GetClarificationRequest(ctx, openReq.ID)
	if superseded.Status != RequestExpired || superseded.ClosedReason != ClosedSuperseded || superseded.ClosedAt == nil {
		t.Errorf("the request after a new run: %+v", superseded)
	}
	list, counts, err := r.ListClarificationRequests(ctx, false, 50)
	if err != nil || len(list) < 3 || counts[req.ID][0] != 1 {
		t.Errorf("ListClarificationRequests: %d requests, counts %v (err %v)", len(list), counts[req.ID], err)
	}
	if onlyOpen, _, _ := r.ListClarificationRequests(ctx, true, 50); len(onlyOpen) != 0 {
		for _, q := range onlyOpen {
			if q.ID == req.ID || q.ID == held.ID || q.ID == openReq.ID {
				t.Errorf("a closed request listed as open: %s", q.ID)
			}
		}
	}
}

// TestClarificationClosesIntoARun is step 7's exit criterion at the row
// level (13 §2; FR-HF-14, FR-HF-22): one unresolved judgment resolved
// by an answer, visible in the record with its provenance. A run rests
// with a question open; the answer closes into a second run that
// judges that one requirement again and carries every other judgment
// forward as it was; and the record of each run says where the
// questions stand.
func TestClarificationClosesIntoARun(t *testing.T) {
	r, ctx := testRepo(t)
	now := time.Now().UTC().Truncate(time.Second)
	f := newFixture(t, r, ctx, 0.7)
	t.Cleanup(func() {
		_, _ = r.pool.Exec(ctx, `DELETE FROM decision_log WHERE ref_kind = 'jd_submission' AND ref_id = $1`, f.submissionID)
	})
	newRun := func(attempt int, trigger string) string {
		t.Helper()
		var id string
		if err := r.pool.QueryRow(ctx, `
      INSERT INTO jd_runs (run_id, submission_id, attempt, trigger, status, match_score, score_formula, model, prompts, finished_at)
      VALUES (gen_random_uuid(), $1, $2, $3, 'ready', 0.8, 'v2-nice-bonus', 'test-model', '{}'::jsonb, now())
      RETURNING run_id::text`, f.submissionID, attempt, trigger).Scan(&id); err != nil {
			t.Fatalf("make run %d: %v", attempt, err)
		}
		return id
	}
	// One of the candidate's own roles for a question to offer; a fresh
	// database has none.
	var roleDoc int64
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO corpus_documents (source_kind, source_path, title, visibility, meta)
    VALUES ('role', 'test/' || gen_random_uuid()::text, 'Director of Engineering (closing test)', 'public',
            '{"organization": "Test Distilling", "start": "2017", "end": "2022"}'::jsonb)
    RETURNING id`).Scan(&roleDoc); err != nil {
		t.Fatalf("make role document: %v", err)
	}
	t.Cleanup(func() { _, _ = r.pool.Exec(ctx, `DELETE FROM corpus_documents WHERE id = $1`, roleDoc) })
	roleRef := "corpus_document:" + strconv.FormatInt(roleDoc, 10)
	recordRun(t, r, ctx, f, f.runID, 1)
	if _, err := r.pool.Exec(ctx, `UPDATE jd_runs SET status = $2, match_score = NULL WHERE run_id = $1::uuid`, f.runID, RunAwaitingCandidate); err != nil {
		t.Fatal(err)
	}
	limit := now.Add(72 * time.Hour)
	req, _, err := r.GenerateClarification(ctx, GenerateInput{RunID: f.runID, Held: true, ExpiresAt: &limit})
	if err != nil {
		t.Fatalf("GenerateClarification: %v", err)
	}
	item := req.Items[0]

	// The record while the question is open: no fit, and r2 says what
	// was asked.
	first, err := r.GetRecord(ctx, f.runID)
	if err != nil {
		t.Fatalf("GetRecord: %v", err)
	}
	if !first.AwaitingCandidate() || first.Clarification == nil {
		t.Fatalf("the record must rest awaiting the candidate: status=%s clarification=%+v", first.RunStatus, first.Clarification)
	}
	if c := first.Clarification; c.RequestID != req.ID || c.Status != RequestSent || !c.Held || c.Asked != 1 || c.Pending != 1 || c.Answered != 0 ||
		c.AskedAttempt != 1 || c.ClosingAttempt != 0 || c.ClosedByThisRun || c.ExpiresAt == nil {
		t.Errorf("the open questionnaire on the record: %+v", c)
	}
	open := judgmentByLocal(t, first, "r2").FollowUp
	if open == nil || open.Status != ItemPending || open.ItemID != item.ID || open.Question != item.QuestionText || open.AskedAttempt != 1 || open.Statement != "" {
		t.Fatalf("r2's follow-up while open: %+v", open)
	}
	if judgmentByLocal(t, first, "r1").FollowUp != nil {
		t.Error("a row nobody asked about has no follow-up")
	}

	// Answered and submitted: the request is owed a run. The choice is
	// the role made for this test, which the question offered.
	saved, err := r.SaveClarificationAnswer(ctx, item.ID, Answer{Status: ItemAnswered,
		Values: map[string]any{"engagements": []string{roleRef}}, FreeText: "Led FEL-1 to FEL-3 for the expansion."}, now)
	if err != nil {
		t.Fatalf("SaveClarificationAnswer: %v", err)
	}
	if _, err := r.SubmitClarificationRequest(ctx, req.ID, now); err != nil {
		t.Fatalf("SubmitClarificationRequest: %v", err)
	}
	owed := func() bool {
		list, err := r.ClarificationsOwedARun(ctx, 3)
		if err != nil {
			t.Fatalf("ClarificationsOwedARun: %v", err)
		}
		for _, q := range list {
			if q.ID == req.ID {
				return true
			}
		}
		return false
	}
	if !owed() {
		t.Error("an answered request with no closing run is owed one")
	}

	// The closing run: r2 judged again with the answer, the rest
	// carried.
	carry, err := r.CarryableJudgments(ctx, f.runID)
	if err != nil || len(carry) != 4 {
		t.Fatalf("CarryableJudgments: %d (err %v)", len(carry), err)
	}
	if c := carry["r2"]; c.RequirementType != "outcome" || !c.KeywordKnown || len(c.ChunkIDs) != 1 || c.PromptVersion != 14 || c.Model != "test-model" {
		t.Errorf("r2 as carried: %+v", c)
	}
	standing, err := r.StandingAnswers(ctx, []string{carry["r2"].RequirementID})
	if err != nil || standing[carry["r2"].RequirementID].ItemID != item.ID {
		t.Fatalf("StandingAnswers: %+v (err %v)", standing, err)
	}
	roleID, ok, err := r.RoleForSubmission(ctx, f.submissionID)
	if err != nil || !ok {
		t.Fatalf("RoleForSubmission: ok=%v err=%v", ok, err)
	}
	rows, err := r.ActiveRequirements(ctx, roleID)
	if err != nil {
		t.Fatalf("ActiveRequirements: %v", err)
	}
	run2 := newRun(2, "clarification")
	c1, c3, c4 := carry["r1"], carry["r3"], carry["r4"]
	w := NewWriter(nil, r)
	if err := w.Record(ctx, RunInput{
		SubmissionID: f.submissionID, Title: "Controls Engineer (test)",
		RunID: run2, Attempt: 2, Score: 0.8, Model: "judge-two",
		Prompts:       map[string]string{"jd_requirements": "v5:abcd", "requirement_judge": "v15:9999"},
		JudgePromptID: "requirement_judge", JudgePromptVersion: 15,
		Rows: rows, RequirementsSource: SourceRows,
		Judgments: []JudgmentInput{
			{LocalID: "r1", Determination: c1.Determination, Basis: c1.Basis, Carried: &c1},
			{LocalID: "r2", Verdict: "met", EvidenceChunkIDs: []int64{f.chunkIDs[2]},
				Determination: DetMetEquivalent, Basis: BasisAdjacentDomain, Uncertainty: []string{},
				Rationale:       "The candidate states they led FEL-1 to FEL-3, which is front-end loading.",
				EvidenceItemIDs: []string{saved.EvidenceItemID}, ClarificationItemIDs: []string{item.ID}},
			{LocalID: "r3", Determination: c3.Determination, Basis: c3.Basis, Carried: &c3},
			{LocalID: "r4", Determination: c4.Determination, Basis: c4.Basis, NotAssessed: true, Carried: &c4},
		},
		Offered: map[string][]Chunk{"r2": {{ID: f.chunkIDs[2], Text: "Led FEL-3 for a $60M expansion through to approval."}}},
	}); err != nil {
		t.Fatalf("Record (closing run): %v", err)
	}
	if err := r.CloseClarificationRequest(ctx, req.ID, ClosedAnswered, run2, now); err != nil {
		t.Fatalf("CloseClarificationRequest: %v", err)
	}
	if owed() {
		t.Error("a closed request is owed nothing")
	}
	// Closing twice changes nothing: the first closing stands.
	if err := r.CloseClarificationRequest(ctx, req.ID, ClosedExpired, f.runID, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if again, _ := r.GetClarificationRequest(ctx, req.ID); again.ClosedReason != ClosedAnswered || again.ClosingRunID != run2 {
		t.Errorf("a second closing must not replace the first: %+v", again)
	}

	second, err := r.GetRecord(ctx, run2)
	if err != nil {
		t.Fatalf("GetRecord (closing run): %v", err)
	}
	if second.AwaitingCandidate() {
		t.Error("the closing run carries a fit")
	}
	if c := second.Clarification; c == nil || !c.ClosedByThisRun || c.ClosedReason != ClosedAnswered || c.AskedAttempt != 1 || c.ClosingAttempt != 2 ||
		c.Asked != 1 || c.Answered != 1 || c.Pending != 0 {
		t.Errorf("the questionnaire on the closing run's record: %+v", second.Clarification)
	}
	// r2: resolved by the answer, and the record shows it.
	j2 := judgmentByLocal(t, second, "r2")
	if j2.Determination != DetMetEquivalent || !j2.SelfAttested || j2.FollowUp == nil || j2.FollowUp.Status != ItemAnswered ||
		!strings.Contains(j2.FollowUp.Statement, "Led FEL-1 to FEL-3 for the expansion.") || j2.FollowUp.AnsweredAt == nil {
		t.Fatalf("r2 on the closing run: %+v follow-up %+v", j2, j2.FollowUp)
	}
	var answers, documents int
	for _, e := range j2.Evidence {
		switch e.SourceKind {
		case SourceCandidateResponse:
			answers++
			if !strings.Contains(e.Statement, "Led FEL-1 to FEL-3") {
				t.Errorf("the answer's citation carries its statement: %+v", e)
			}
		default:
			documents++
			if e.Statement != "" {
				t.Errorf("a document's text is never part of a citation: %+v", e)
			}
		}
	}
	if answers != 1 || documents != 1 {
		t.Errorf("r2 cites the answer beside the document: %d answers, %d documents", answers, documents)
	}
	if !(second.Coverage.SelfAttestedShare > 0) || second.Coverage.Unresolved != 0 {
		t.Errorf("the numbers after the answer: %+v", second.Coverage)
	}
	// r1, r3, r4: carried as they were, by the judge that made them.
	for _, local := range []string{"r1", "r3", "r4"} {
		was, is := judgmentByLocal(t, first, local), judgmentByLocal(t, second, local)
		if is.Determination != was.Determination || is.Basis != was.Basis || is.Rationale != was.Rationale ||
			len(is.Evidence) != len(was.Evidence) || is.KeywordKnown != was.KeywordKnown || is.KeywordFound != was.KeywordFound || is.SelfAttested {
			t.Errorf("%s must be carried unchanged: was %+v, is %+v", local, was, is)
		}
		var supersedes, model string
		var promptVersion int
		if err := r.pool.QueryRow(ctx, `SELECT COALESCE(supersedes::text, ''), model, prompt_version FROM judgments WHERE id = $1::uuid`, is.ID).
			Scan(&supersedes, &model, &promptVersion); err != nil {
			t.Fatal(err)
		}
		if supersedes != was.ID || model != "test-model" || promptVersion != 14 {
			t.Errorf("%s: supersedes=%s (want %s) model=%s prompt v%d; a carried row keeps the judge that made it", local, supersedes, was.ID, model, promptVersion)
		}
	}
	var supersedes, model string
	var consumed []string
	if err := r.pool.QueryRow(ctx, `SELECT COALESCE(supersedes::text, ''), model, clarification_item_ids::text[] FROM judgments WHERE id = $1::uuid`, j2.ID).
		Scan(&supersedes, &model, &consumed); err != nil {
		t.Fatal(err)
	}
	if supersedes != judgmentByLocal(t, first, "r2").ID || model != "judge-two" || len(consumed) != 1 || consumed[0] != item.ID {
		t.Errorf("r2's row: supersedes=%s model=%s consumed=%v", supersedes, model, consumed)
	}
	var after string
	var n int
	if err := r.pool.QueryRow(ctx, `
    SELECT determination_after, items FROM v_clarification_outcomes o
    WHERE uncertainty_code = $1 AND answer_status = 'answered' AND determination_after = 'met_equivalent'`, UncEquivalenceUnconfirmed).Scan(&after, &n); err != nil || n < 1 {
		t.Errorf("v_clarification_outcomes: %s x%d (err %v)", after, n, err)
	}
	// The run that asked is history now, and still says what came of
	// its question.
	if was, _ := r.GetRecord(ctx, f.runID); was.Clarification == nil || was.Clarification.ClosedByThisRun || was.Clarification.ClosingAttempt != 2 ||
		judgmentByLocal(t, was, "r2").FollowUp.Status != ItemAnswered {
		t.Errorf("the asking run's record after the close: %+v", was.Clarification)
	}

	// A second posting: "I have not done this" settles r2 in code, on
	// the judgment the model had made.
	g := newFixture(t, r, ctx, 0.7)
	t.Cleanup(func() {
		_, _ = r.pool.Exec(ctx, `DELETE FROM decision_log WHERE ref_kind = 'jd_submission' AND ref_id = $1`, g.submissionID)
	})
	recordRun(t, r, ctx, g, g.runID, 1)
	greq, _, err := r.GenerateClarification(ctx, GenerateInput{RunID: g.runID, Held: true, ExpiresAt: &limit})
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	na, err := r.SaveClarificationAnswer(ctx, greq.Items[0].ID, Answer{Status: ItemNotApplicable}, now)
	if err != nil {
		t.Fatalf("not applicable: %v", err)
	}
	gcarry, err := r.CarryableJudgments(ctx, g.runID)
	if err != nil {
		t.Fatal(err)
	}
	groleID, _, _ := r.RoleForSubmission(ctx, g.submissionID)
	grows, _ := r.ActiveRequirements(ctx, groleID)
	var grun2 string
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO jd_runs (run_id, submission_id, attempt, trigger, status, match_score, score_formula, model, prompts, finished_at)
    VALUES (gen_random_uuid(), $1, 2, 'clarification', 'below_threshold', 0.4, 'v2-nice-bonus', 'test-model', '{}'::jsonb, now())
    RETURNING run_id::text`, g.submissionID).Scan(&grun2); err != nil {
		t.Fatal(err)
	}
	g1, g2, g3, g4 := gcarry["r1"], gcarry["r2"], gcarry["r3"], gcarry["r4"]
	verdict, adj := NotApplicableVerdict(g2.Basis)
	adjs, _ := json.Marshal([]RuleAdjustment{adj})
	if err := w.Record(ctx, RunInput{
		SubmissionID: g.submissionID, Title: "Controls Engineer (test)",
		RunID: grun2, Attempt: 2, Score: 0.4, Model: "judge-two",
		Prompts:       map[string]string{"requirement_judge": "v15:9999"},
		JudgePromptID: "requirement_judge", JudgePromptVersion: 15,
		Rows: grows, RequirementsSource: SourceRows,
		Judgments: []JudgmentInput{
			{LocalID: "r1", Determination: g1.Determination, Basis: g1.Basis, Carried: &g1},
			{LocalID: "r2", Verdict: "unmet", Determination: verdict.Determination, Basis: verdict.Basis, Uncertainty: []string{},
				Rationale: g2.Rationale, RuleAdjustments: adjs,
				EvidenceItemIDs: []string{na.EvidenceItemID}, ClarificationItemIDs: []string{na.ID}, SettledFrom: &g2},
			{LocalID: "r3", Determination: g3.Determination, Basis: g3.Basis, Carried: &g3},
			{LocalID: "r4", Determination: g4.Determination, Basis: g4.Basis, NotAssessed: true, Carried: &g4},
		},
	}); err != nil {
		t.Fatalf("Record (settled in code): %v", err)
	}
	settled, err := r.GetRecord(ctx, grun2)
	if err != nil {
		t.Fatalf("GetRecord (settled in code): %v", err)
	}
	s2 := judgmentByLocal(t, settled, "r2")
	if s2.Determination != DetNotMet || s2.Basis != BasisNotAddressed || !s2.SelfAttested || len(s2.Evidence) != 1 ||
		s2.Evidence[0].SourceKind != SourceCandidateResponse || s2.FollowUp == nil || s2.FollowUp.Status != ItemNotApplicable ||
		len(s2.RuleAdjustments) != 1 || s2.RuleAdjustments[0].Rule != RuleClarification || s2.KeywordKnown != judgmentByLocal(t, first, "r2").KeywordKnown {
		t.Fatalf("r2 settled on \"I have not done this\": %+v evidence %+v follow-up %+v", s2, s2.Evidence, s2.FollowUp)
	}
	var settledModel string
	var settledPrompt int
	if err := r.pool.QueryRow(ctx, `SELECT model, prompt_version FROM judgments WHERE id = $1::uuid`, s2.ID).Scan(&settledModel, &settledPrompt); err != nil {
		t.Fatal(err)
	}
	if settledModel != "test-model" || settledPrompt != 14 {
		t.Errorf("a row settled in code keeps the judge whose output it stands on: %s v%d", settledModel, settledPrompt)
	}

	// A third posting: a question that lapsed is not asked again
	// unprompted, and is asked again when the candidate asks.
	h := newFixture(t, r, ctx, 0.7)
	t.Cleanup(func() {
		_, _ = r.pool.Exec(ctx, `DELETE FROM decision_log WHERE ref_kind = 'jd_submission' AND ref_id = $1`, h.submissionID)
	})
	recordRun(t, r, ctx, h, h.runID, 1)
	soon := now.Add(time.Hour)
	lapsing, _, err := r.GenerateClarification(ctx, GenerateInput{RunID: h.runID, Held: true, ExpiresAt: &soon})
	if err != nil {
		t.Fatalf("third request: %v", err)
	}
	if due, err := r.ExpireClarificationRequests(ctx, soon.Add(time.Minute)); err != nil || len(due) == 0 {
		t.Fatalf("ExpireClarificationRequests: %d (err %v)", len(due), err)
	}
	lapsed, _ := r.GetRecord(ctx, h.runID)
	if f := judgmentByLocal(t, lapsed, "r2").FollowUp; f == nil || f.Status != FollowUpUnanswered || f.ItemID != lapsing.Items[0].ID {
		t.Errorf("a question that closed with no answer reads as unanswered: %+v", f)
	}
	var hrun2 string
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO jd_runs (run_id, submission_id, attempt, trigger, status, match_score, score_formula, model, prompts, finished_at)
    VALUES (gen_random_uuid(), $1, 2, 'rescore', 'ready', 0.7, 'v2-nice-bonus', 'test-model', '{}'::jsonb, now())
    RETURNING run_id::text`, h.submissionID).Scan(&hrun2); err != nil {
		t.Fatal(err)
	}
	recordRun(t, r, ctx, h, hrun2, 2)
	if _, _, err := r.GenerateClarification(ctx, GenerateInput{RunID: hrun2, Held: true, ExpiresAt: &limit}); !errors.Is(err, ErrNothingToAsk) {
		t.Errorf("a lapsed question asked again unprompted: %v, want nothing to ask", err)
	}
	asked, added, err := r.GenerateClarification(ctx, GenerateInput{RunID: hrun2, All: true, AskedBy: AskedByCandidate})
	if err != nil || added != 1 || asked.Items[0].LocalID != "r2" {
		t.Errorf("the candidate's own request asks it again: added=%d err=%v", added, err)
	}

	// A fourth posting: a question a new run closed before it could be
	// answered did not lapse. The record says what happened to it, the
	// new run asks it again unprompted, and a limit of one day gets no
	// reminder, which would arrive with the questions.
	k := newFixture(t, r, ctx, 0.7)
	t.Cleanup(func() {
		_, _ = r.pool.Exec(ctx, `DELETE FROM decision_log WHERE ref_kind = 'jd_submission' AND ref_id = $1`, k.submissionID)
	})
	recordRun(t, r, ctx, k, k.runID, 1)
	oneDay := now.Add(24 * time.Hour)
	short, _, err := r.GenerateClarification(ctx, GenerateInput{RunID: k.runID, Held: true, ExpiresAt: &oneDay})
	if err != nil {
		t.Fatalf("fourth request: %v", err)
	}
	early, err := r.ClarificationReminders(ctx, now.Add(time.Hour), 24*time.Hour)
	if err != nil {
		t.Fatalf("ClarificationReminders: %v", err)
	}
	for _, q := range early {
		if q.ID == short.ID {
			t.Error("a one-day limit must not be reminded a day before it")
		}
	}
	if n, err := r.SupersedeClarifications(ctx, k.submissionID, now); err != nil || n != 1 {
		t.Fatalf("SupersedeClarifications: %d (err %v)", n, err)
	}
	replaced, _ := r.GetRecord(ctx, k.runID)
	if f := judgmentByLocal(t, replaced, "r2").FollowUp; f == nil || f.Status != FollowUpSuperseded {
		t.Errorf("a question closed by a new run reads as superseded: %+v", f)
	}
	var krun2 string
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO jd_runs (run_id, submission_id, attempt, trigger, status, match_score, score_formula, model, prompts, finished_at)
    VALUES (gen_random_uuid(), $1, 2, 'rescore', 'ready', 0.7, 'v2-nice-bonus', 'test-model', '{}'::jsonb, now())
    RETURNING run_id::text`, k.submissionID).Scan(&krun2); err != nil {
		t.Fatal(err)
	}
	recordRun(t, r, ctx, k, krun2, 2)
	again, added, err := r.GenerateClarification(ctx, GenerateInput{RunID: krun2, Held: true, ExpiresAt: &limit})
	if err != nil || added != 1 || again.ID == short.ID || again.Items[0].LocalID != "r2" {
		t.Errorf("the new run asks the superseded question again: added=%d err=%v", added, err)
	}
}
