package jd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/reh3376/career-site/services/api/internal/corpusscope"
	"github.com/reh3376/career-site/services/api/internal/hiring"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// clarify.go is the pipeline's half of the clarification loop (Phase 0
// step 7; FSD FR-HF-13, 14, 22 and 24). The questions, the answers and
// their evidence live in internal/hiring; what is here is what a run
// does about them:
//
//   - a full run that leaves a requirement unresolved in a way the
//     framework asks about unprompted writes the questions and rests at
//     awaiting_candidate with no fit (holdForCandidate);
//   - when the candidate has answered, or the time limit has passed, a
//     new run judges only the requirements an answer speaks to, carries
//     every other judgment forward as it was, and then goes through the
//     gate like any run (CloseClarification, judgeOptions);
//   - every later run of the posting has the standing answers beside
//     the documents, because an answer is evidence about a requirement
//     version and not about a run.

// Run triggers and the resting status of the loop.
const (
	// TriggerClarification: the candidate answered. The run judges the
	// requirements the answers speak to and carries the rest.
	TriggerClarification = "clarification"
	// TriggerExpiry: the time limit passed with questions open. The run
	// issues the fit from what is evidenced, with whatever was answered.
	TriggerExpiry = "expiry"
	// StatusAwaitingCandidate is where a submission and its run rest
	// while questions are open and the fit is withheld (FR-HF-22).
	StatusAwaitingCandidate = hiring.RunAwaitingCandidate
)

// ReminderLead is how long before the limit the candidate is reminded.
const ReminderLead = 24 * time.Hour

// maxClosingRuns bounds how many times the run that closes one request
// is started on its own: a restart resumes it, and a request whose run
// has already failed this many times is left for a person to re-score.
const maxClosingRuns = 3

// clarifyScope narrows a run to the requirements a closed questionnaire
// speaks to.
type clarifyScope struct {
	requestID string
	// prevRunID is the run that asked; its judgments are what is
	// carried.
	prevRunID string
	// reason is how the request closed: answered or expired.
	reason string
}

// runCarry is what a run did not send to the judge, by requirement id
// within the role: the judgments it carried forward as they were, and
// the judgments each row settled in code was settled from.
type runCarry struct {
	carried map[string]hiring.CarriedJudgment
	settled map[string]hiring.CarriedJudgment
}

// model names the model behind the judgments a run kept, for a run
// that sent nothing to the judge.
func (rc runCarry) model() string {
	for _, c := range rc.carried {
		if c.Model != "" {
			return c.Model
		}
	}
	for _, c := range rc.settled {
		if c.Model != "" {
			return c.Model
		}
	}
	return ""
}

// judgedAgain counts the judgments of an assessment that were not
// carried forward unchanged.
func (a *Assessment) judgedAgain() int {
	if a == nil {
		return 0
	}
	n := 0
	for _, j := range a.Judgments {
		if !j.Carried {
			n++
		}
	}
	return n
}

// carriedJudgment is a stored judgment in the form the judging pass and
// the assessment use.
func carriedJudgment(c hiring.CarriedJudgment) Judgment {
	j := Judgment{
		RequirementID:      c.LocalID,
		Verdict:            hiring.DeriveVerdict(c.Determination),
		Determination:      c.Determination,
		Basis:              c.Basis,
		Uncertainty:        append([]string{}, c.Uncertainty...),
		Type:               c.RequirementType,
		EvidenceIDs:        append([]int64{}, c.ChunkIDs...),
		Rationale:          c.Rationale,
		Confidence:         c.Confidence,
		NotAssessed:        c.RequirementType == hiring.TypeBehavioral,
		AnswerEvidence:     append([]string{}, c.AnswerEvidenceIDs...),
		ClarificationItems: append([]string{}, c.ClarificationItemIDs...),
		Carried:            true,
	}
	if len(c.ReportedFacts) > 0 {
		var f hiring.ReportedFacts
		if json.Unmarshal(c.ReportedFacts, &f) == nil {
			j.ReportedFacts = &f
			j.StatedSpanYears, j.PartiesEvidenced = f.StatedSpanYears, f.PartiesEvidenced
		}
	}
	if len(c.RuleAdjustments) > 0 {
		_ = json.Unmarshal(c.RuleAdjustments, &j.RuleAdjustments)
	}
	if len(c.RationaleIssues) > 0 {
		_ = json.Unmarshal(c.RationaleIssues, &j.RationaleIssues)
	}
	j.Adjusted = adjustmentReasons(j.RuleAdjustments)
	return j
}

func adjustmentReasons(adjs []hiring.RuleAdjustment) string {
	reasons := make([]string, 0, len(adjs))
	for _, a := range adjs {
		reasons = append(reasons, a.Reason)
	}
	return strings.Join(reasons, "; ")
}

// structuredAnswer reports whether an answer type carries a fact the
// code rules compare.
func structuredAnswer(answerType string) bool {
	switch answerType {
	case hiring.AnswerSpan, hiring.AnswerMagnitude, hiring.AnswerCredential, hiring.AnswerRelationship, hiring.AnswerInvolvement:
		return true
	}
	return false
}

// answerEvidence is a standing answer as the judging pass uses it.
func answerEvidence(ans hiring.StoredAnswer, now time.Time) AnswerEvidence {
	ae := AnswerEvidence{ItemID: ans.ItemID, EvidenceItemID: ans.EvidenceItemID, Statement: ans.Statement}
	if ans.Answer.Status == hiring.ItemNotApplicable {
		ae.NotApplicable = true
		return ae
	}
	if structuredAnswer(ans.AnswerType) {
		ae.FactsFix = func(f hiring.ReportedFacts) hiring.ReportedFacts {
			return hiring.AnswerFacts(ans.AnswerType, ans.Fields, ans.Answer, f, now)
		}
	}
	return ae
}

// settleOnAnswer settles in code a judgment the run before left
// unresolved, from the candidate's answer, when nothing remains for a
// model to judge: "I have not done this" is not met, not addressed; a
// stated number, credential or party that a code rule was missing is
// put to the same rule again (hiring.Resettle). ok is false when the
// answer is one only a judge can weigh, and the caller sends the
// requirement to the judge with the answer beside its evidence.
func settleOnAnswer(q hiring.Requirement, c hiring.CarriedJudgment, ans hiring.StoredAnswer, now time.Time) (Judgment, bool) {
	if c.Determination != hiring.DetUnresolved || c.RequirementType == hiring.TypeBehavioral {
		return Judgment{}, false
	}
	j := carriedJudgment(c)
	j.Carried = false
	var facts hiring.ReportedFacts
	if j.ReportedFacts != nil {
		facts = *j.ReportedFacts
	}
	var verdict hiring.Verdict
	if ans.Answer.Status == hiring.ItemNotApplicable {
		v, adj := hiring.NotApplicableVerdict(c.Basis)
		verdict = v
		j.RuleAdjustments = append(j.RuleAdjustments, adj)
	} else {
		facts = hiring.AnswerFacts(ans.AnswerType, ans.Fields, ans.Answer, facts, now)
		r := FromRow(q)
		v, adjs, ok := hiring.Resettle(hiring.RuleInput{
			Text: r.Text, Type: r.Type, NamedParties: r.NamedParties, Thresholds: r.Thresholds,
		}, ans.AnswerType, j.RuleAdjustments, c.Basis, facts, now)
		if !ok {
			return Judgment{}, false
		}
		verdict, j.RuleAdjustments = v, adjs
	}
	if verdict.Uncertainty == nil {
		verdict.Uncertainty = []string{}
	}
	j.Determination, j.Basis, j.Uncertainty = verdict.Determination, verdict.Basis, verdict.Uncertainty
	j.Verdict = hiring.DeriveVerdict(verdict.Determination)
	j.Adjusted = adjustmentReasons(j.RuleAdjustments)
	factsCopy := facts
	j.ReportedFacts = &factsCopy
	j.StatedSpanYears, j.PartiesEvidenced = facts.StatedSpanYears, facts.PartiesEvidenced
	if verdict.Determination == hiring.DetNotMet && verdict.Basis == hiring.BasisNotAddressed {
		// As everywhere: a judgment that nothing addresses cites no
		// passage. What it rests on is the candidate's statement.
		j.EvidenceIDs = nil
	}
	if !slices.Contains(j.AnswerEvidence, ans.EvidenceItemID) {
		j.AnswerEvidence = append(j.AnswerEvidence, ans.EvidenceItemID)
	}
	if !slices.Contains(j.ClarificationItems, ans.ItemID) {
		j.ClarificationItems = append(j.ClarificationItems, ans.ItemID)
	}
	return j, true
}

// judgeOptions decides, for the rows a run is about to judge, which go
// to the judge and with what. Every run gives the judge the candidate's
// standing answers. A run that closes a questionnaire (scope set) also
// carries forward every judgment of the run that asked whose
// requirement no new answer speaks to, and settles in code the ones an
// answer settles without a judge (FR-HF-14: re-judging is scoped to the
// affected requirements and carries the rest). A requirement whose
// version has changed since that run is judged like any other.
func (s *Scorer) judgeOptions(ctx context.Context, rows []hiring.Requirement, scope *clarifyScope) (JudgeOptions, runCarry, error) {
	var opts JudgeOptions
	var rc runCarry
	if s.hiring == nil || s.hiring.Repo() == nil || len(rows) == 0 {
		return opts, rc, nil
	}
	repo := s.hiring.Repo()
	ids := make([]string, 0, len(rows))
	for _, q := range rows {
		ids = append(ids, q.ID)
	}
	answers, err := repo.StandingAnswers(ctx, ids)
	if err != nil {
		return opts, rc, fmt.Errorf("answers: %w", err)
	}
	var prev map[string]hiring.CarriedJudgment
	if scope != nil {
		prev, err = repo.CarryableJudgments(ctx, scope.prevRunID)
		if err != nil {
			return opts, rc, fmt.Errorf("carried judgments: %w", err)
		}
	}
	if len(answers) == 0 && len(prev) == 0 {
		return opts, rc, nil
	}
	now := time.Now()
	opts = JudgeOptions{Carry: map[string]Judgment{}, CarryDecisions: map[string]int64{}, Answers: map[string]AnswerEvidence{}}
	rc = runCarry{carried: map[string]hiring.CarriedJudgment{}, settled: map[string]hiring.CarriedJudgment{}}
	for _, q := range rows {
		ans, has := answers[q.ID]
		c, was := prev[q.LocalID]
		was = was && c.RequirementID == q.ID
		if was {
			// consumed: the earlier judgment was already made with this
			// answer on the table.
			consumed := has && slices.Contains(c.ClarificationItemIDs, ans.ItemID)
			switch {
			case !has && len(c.ClarificationItemIDs) > 0:
				// The answer it stood on has been withdrawn: judged
				// afresh, without it.
			case !has || consumed:
				opts.Carry[q.LocalID] = carriedJudgment(c)
				opts.CarryDecisions[q.LocalID] = c.DecisionID
				rc.carried[q.LocalID] = c
				continue
			default:
				if j, ok := settleOnAnswer(q, c, ans, now); ok {
					opts.Carry[q.LocalID] = j
					opts.CarryDecisions[q.LocalID] = c.DecisionID
					rc.settled[q.LocalID] = c
					continue
				}
			}
		}
		if has {
			opts.Answers[q.LocalID] = answerEvidence(ans, now)
		}
	}
	return opts, rc, nil
}

// holdForCandidate writes the questions a finished run owes the
// candidate and reports whether the run now rests with no fit
// (FR-HF-22, FR-HF-24). It asks only when the loop's unprompted half is
// switched on, and never for an evaluation run, whose point is the
// pipeline's own answer. ErrNothingToAsk is the ordinary case and is
// not a hold; any other failure is logged and is not a hold either,
// because a review must not be lost to its follow-up.
func (s *Scorer) holdForCandidate(ctx context.Context, submissionID int64, runID string, evaluation bool) (hiring.ClarificationRequest, bool) {
	if s.hiring == nil || s.hiring.Repo() == nil || s.assessor == nil || evaluation {
		return hiring.ClarificationRequest{}, false
	}
	settings := s.assessor.Settings(ctx)
	if !settings.ClarificationAuto {
		return hiring.ClarificationRequest{}, false
	}
	expires := time.Now().Add(settings.ClarificationLimit())
	q, added, err := s.hiring.Repo().GenerateClarification(ctx, hiring.GenerateInput{
		RunID: runID, Held: true, ExpiresAt: &expires, AskedBy: hiring.AskedBySystem,
	})
	if errors.Is(err, hiring.ErrNothingToAsk) {
		return hiring.ClarificationRequest{}, false
	}
	if err != nil {
		s.log.Warn("jd: the follow-up questions could not be written; the run is given its fit",
			slog.Int64("id", submissionID), slog.String("run", runID), slog.String("error", err.Error()))
		return hiring.ClarificationRequest{}, false
	}
	if added == 0 {
		return hiring.ClarificationRequest{}, false
	}
	s.log.Info("jd: waiting on the candidate", slog.Int64("id", submissionID), slog.String("run", runID),
		slog.String("request", q.ID), slog.Int("questions", added))
	return q, true
}

// CloseClarification starts the run that a closed questionnaire owes
// (FR-HF-14, FR-HF-22): the candidate has answered, or the time limit
// has passed. It returns once the run has finished, so callers start it
// on its own goroutine. adminID is the person whose action closed the
// questions, 0 for the clock.
func (s *Scorer) CloseClarification(ctx context.Context, q hiring.ClarificationRequest, adminID int64) {
	if s.hiring == nil || s.hiring.Repo() == nil || q.ID == "" || q.SubmissionID == 0 {
		return
	}
	// One closing run per request. The clock, the boot-time resume and a
	// submit can each reach here for the same request; the second to
	// arrive while the first is running returns at once, and one that
	// arrives after it has finished finds the request closed.
	if !s.claimClosing(q.ID) {
		s.log.Info("jd: the run closing this questionnaire is already under way", slog.String("request", q.ID))
		return
	}
	defer s.releaseClosing(q.ID)
	if fresh, fErr := s.hiring.Repo().GetClarificationRequest(ctx, q.ID); fErr != nil {
		s.log.Warn("jd: the run closing a questionnaire could not start", slog.String("request", q.ID), slog.String("error", fErr.Error()))
		return
	} else if fresh.ClosedAt != nil || fresh.Status == hiring.RequestSent {
		// Closed already, or (never expected) still taking answers.
		return
	} else {
		q = fresh
	}
	row, err := s.users.GetJdSubmission(ctx, q.SubmissionID)
	if err != nil {
		s.log.Warn("jd: the run closing a questionnaire could not start", slog.String("request", q.ID),
			slog.Int64("id", q.SubmissionID), slog.String("error", err.Error()))
		return
	}
	trigger, reason := TriggerClarification, hiring.ClosedAnswered
	if q.Status == hiring.RequestExpired {
		trigger, reason = TriggerExpiry, hiring.ClosedExpired
	}
	hints := prompts.Hints{Role: row.RoleHint, Employer: row.EmployerHint}
	// The run reads what the run that asked read.
	scope := corpusscope.Public
	if q.RetrievalScope == "all" {
		scope = corpusscope.All
	}
	s.scoreAndPersist(corpusscope.With(ctx, scope), q.SubmissionID, row.JdText, hints, trigger, adminID,
		&clarifyScope{requestID: q.ID, prevRunID: q.RunID, reason: reason})
}

// claimClosing marks a request as having its closing run under way in
// this process and reports whether the caller got the claim.
func (s *Scorer) claimClosing(requestID string) bool {
	s.closingMu.Lock()
	defer s.closingMu.Unlock()
	if s.closing == nil {
		s.closing = map[string]bool{}
	}
	if s.closing[requestID] {
		return false
	}
	s.closing[requestID] = true
	return true
}

func (s *Scorer) releaseClosing(requestID string) {
	s.closingMu.Lock()
	defer s.closingMu.Unlock()
	delete(s.closing, requestID)
}

// SweepClarifications is the loop's clock. It closes every held request
// whose limit has passed and starts the run that issues each fit, and
// it mails the candidate about each request a day from its limit.
func (s *Scorer) SweepClarifications(ctx context.Context) {
	if s.hiring == nil || s.hiring.Repo() == nil {
		return
	}
	repo := s.hiring.Repo()
	now := time.Now()
	due, err := repo.ClarificationReminders(ctx, now, ReminderLead)
	if err != nil {
		s.log.Warn("jd: clarification reminders failed", slog.String("error", err.Error()))
	}
	for _, q := range due {
		s.notifyClarification(ctx, q, ClarificationReminder)
	}
	expired, err := repo.ExpireClarificationRequests(ctx, now)
	if err != nil {
		s.log.Warn("jd: clarification expiry failed", slog.String("error", err.Error()))
		return
	}
	for _, q := range expired {
		full, err := repo.GetClarificationRequest(ctx, q.ID)
		if err != nil {
			s.log.Warn("jd: an expired questionnaire could not be read", slog.String("request", q.ID), slog.String("error", err.Error()))
			continue
		}
		s.log.Info("jd: the time limit passed; issuing the fit from what is evidenced",
			slog.String("request", full.ID), slog.Int64("id", full.SubmissionID), slog.Int("unanswered", full.Pending()))
		// On its own goroutine and outside the tick's deadline: the run
		// takes as long as a run takes, and waits its turn for the
		// pipeline like any other.
		go s.CloseClarification(context.WithoutCancel(ctx), full, 0)
	}
}

// ResumeClarifications starts, once at boot, the closing run of every
// request that is owed one: answered or expired and not yet closed,
// which is what a restart in the middle of such a run leaves behind.
func (s *Scorer) ResumeClarifications(ctx context.Context) {
	if s.hiring == nil || s.hiring.Repo() == nil {
		return
	}
	owed, err := s.hiring.Repo().ClarificationsOwedARun(ctx, maxClosingRuns)
	if err != nil {
		s.log.Warn("jd: could not list the questionnaires owed a run", slog.String("error", err.Error()))
		return
	}
	for _, q := range owed {
		s.log.Info("jd: resuming the run that closes a questionnaire", slog.String("request", q.ID), slog.Int64("id", q.SubmissionID))
		s.CloseClarification(ctx, q, 0)
	}
}

// Kinds of mail the loop sends.
const (
	// ClarificationAsked: questions are waiting (to the candidate) and a
	// review is waiting on them (to the submitter).
	ClarificationAsked = "asked"
	// ClarificationReminder: a day is left (to the candidate).
	ClarificationReminder = "reminder"
)

// ClarificationNotifier is told when questions are written for the
// candidate and when their limit is a day away.
type ClarificationNotifier interface {
	NotifyClarification(ctx context.Context, s *users.JdSubmission, q hiring.ClarificationRequest, kind string) error
}

// notifyClarification mails about a request, best-effort. An evaluation
// submission mails nobody.
func (s *Scorer) notifyClarification(ctx context.Context, q hiring.ClarificationRequest, kind string) {
	n, ok := s.notifier.(ClarificationNotifier)
	if !ok || n == nil || q.SubmissionID == 0 {
		return
	}
	row, err := s.users.GetJdSubmission(ctx, q.SubmissionID)
	if err != nil {
		s.log.Warn("jd: clarification mail skipped, row unreadable", slog.Int64("id", q.SubmissionID), slog.String("error", err.Error()))
		return
	}
	if row.IsEval {
		return
	}
	if err := n.NotifyClarification(ctx, row, q, kind); err != nil {
		s.log.Warn("jd: clarification mail failed", slog.Int64("id", q.SubmissionID), slog.String("kind", kind), slog.String("error", err.Error()))
	}
}
