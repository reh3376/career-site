package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/email"
	"github.com/reh3376/career-site/services/api/internal/events"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// The decision test (docs/fsd-decision-test.md).
//
// Everything this handler does is shaped by one rule: the client learns
// nothing about correctness. Questions go out without their key,
// answers come back and are graded here, and the response says "stored"
// and nothing else. A running score is never sent, because a score the
// browser holds is a score the participant can read.
//
// That is not UI politeness. Confidence is a dependent variable, and a
// participant who finds out they were wrong stops giving an
// uncontaminated rating for everything after it.
type DecisionTest struct {
	careerv1connect.UnimplementedDecisionTestServiceHandler

	log   *slog.Logger
	users *users.Repo
	auth  *Auth

	events *events.Writer // product event stream; nil is silent

	// Results go out to participants who asked for them. Nil leaves the
	// test working and the emails unsent, which is the right failure:
	// somebody mid-run must not be stopped because a mailer is down.
	email   email.Provider
	from    string
	siteURL string
}

// NewDecisionTest wires the handler. auth may be nil: the test is
// public and a signed-in member is simply recognised when present.
func NewDecisionTest(log *slog.Logger, repo *users.Repo, auth *Auth) *DecisionTest {
	return &DecisionTest{log: log, users: repo, auth: auth}
}

// SetMailer wires result emails. Optional: without it the test runs and
// nobody is written to.
func (h *DecisionTest) SetMailer(p email.Provider, from, siteURL string) {
	h.email, h.from, h.siteURL = p, from, siteURL
}

// SetEvents wires the product event stream. Optional, like every other
// handler's: a run must not fail because analytics is down.
func (h *DecisionTest) SetEvents(w *events.Writer) { h.events = w }

// itemSetVersion identifies which items, in which order. Bumped
// whenever the bank changes, so a reworded item does not silently pool
// with its predecessor.
const (
	// Bumped on 2026-10-05 with migration 00052: three base-rate items
	// retired, one conjunction and two syllogisms added, and the
	// presentation order interleaved so every block carries the same
	// family mix. A run before that is a different instrument and must
	// not pool with one after it.
	itemSetVersion = "items-2026-10-05"
	keyVersion     = "key-1"
)

// StartSession opens a run and returns the practice block.
func (h *DecisionTest) StartSession(
	ctx context.Context, req *connect.Request[v1.StartSessionRequest],
) (*connect.Response[v1.StartSessionResponse], error) {
	if h.users == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("decision test is not configured"))
	}
	m := req.Msg

	intake := users.DTIntake{}
	if in := m.GetIntake(); in != nil {
		intake = users.DTIntake{
			DisplayName:  strings.TrimSpace(in.GetDisplayName()),
			AgeRange:     strings.TrimSpace(in.GetAgeRange()),
			Education:    strings.TrimSpace(in.GetEducation()),
			Occupation:   strings.TrimSpace(in.GetOccupation()),
			Email:        strings.TrimSpace(in.GetEmail()),
			WantsResults: in.GetWantsResults(),
		}
	}

	cond := users.DTConditions{AudioMode: "sound", DeviceClass: "unknown"}
	if c := m.GetConditions(); c != nil {
		if v := c.GetAudioMode(); v == "sound" || v == "visual" {
			cond.AudioMode = v
		}
		switch c.GetDeviceClass() {
		case "desktop", "tablet", "phone":
			cond.DeviceClass = c.GetDeviceClass()
		}
		cond.TapCheckPassed = c.GetTapCheckPassed()
		cond.BaselineRTMs = int(c.GetBaselineRtMs())
		cond.BaselineRTSDMs = int(c.GetBaselineRtSdMs())
	}

	// A member is recognised when signed in; nobody is required to be,
	// and a lookup failure is not an error here. An anonymous run is the
	// normal case, so this degrades to anonymous rather than refusing.
	var userID *int64
	if h.auth != nil {
		if u, err := h.auth.LookupSessionUser(ctx, req); err == nil && u != nil {
			id := u.ID
			userID = &id
		}
	}
	visitorKey := cookieValue(req.Header().Get("Cookie"), AnonCookieName)

	// Timings are owner-editable (/admin/decision-test), and the
	// instrument version is derived from them, so a run always records
	// the timings it was taken under.
	timings := h.users.DTGetSettings(ctx)

	sess, err := h.users.StartDecisionTest(ctx, intake, cond, m.GetSynthetic(),
		visitorKey, userID, timings.InstrumentVersion(), itemSetVersion, keyVersion)
	if err != nil {
		h.log.Error("decision test: start", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("could not start the test"))
	}

	// The practice block carries the tick, so a participant hears it
	// before anything is scored, and it is where a muted device is
	// caught rather than at question nine.
	practice, err := h.practiceBlock(ctx, sess.ID)
	if err != nil {
		h.log.Error("decision test: practice block", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("could not prepare the test"))
	}

	h.log.Info("decision test started",
		slog.String("session", sess.PublicID),
		slog.Bool("synthetic", sess.Synthetic),
		slog.String("audio_mode", cond.AudioMode))

	var eventUser int64
	if userID != nil {
		eventUser = *userID
	}
	h.events.Emit(ctx, requestEvent(req, "dtest.started", eventUser, map[string]any{
		"device_class":     cond.DeviceClass,
		"audio_mode":       cond.AudioMode,
		"tap_check_passed": cond.TapCheckPassed,
		"is_repeat":        sess.IsRepeat,
		"synthetic":        sess.Synthetic,
	}))

	return connect.NewResponse(&v1.StartSessionResponse{
		SessionKey:    sess.PublicID,
		Practice:      practice,
		BlockCount:    users.DTBlockCount,
		QuestionCount: users.DTQuestionCount,
		Timings: &v1.Timings{
			MemoriseMs: int32(timings.MemoriseMs),
			QuestionMs: int32(timings.QuestionMs),
			RecallMs:   int32(timings.RecallMs),
		},
	}), nil
}

// GetBlock serves one block: the number to hold and its six questions.
//
// One at a time, so the client never holds the whole test, and so the
// digits for a later block do not exist in the browser before the
// participant reaches it.
func (h *DecisionTest) GetBlock(
	ctx context.Context, req *connect.Request[v1.GetBlockRequest],
) (*connect.Response[v1.GetBlockResponse], error) {
	sess, err := h.session(ctx, req.Msg.GetSessionKey())
	if err != nil {
		return nil, err
	}
	blockNo := int(req.Msg.GetBlockNo())

	items, err := h.users.DTScoredItems(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("could not load the questions"))
	}
	from := (blockNo - 1) * users.DTQuestionsPerBlock
	to := from + users.DTQuestionsPerBlock
	if to > len(items) {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("the item bank holds %d scored items, which is not enough for block %d", len(items), blockNo))
	}

	block, err := h.users.DTBuildBlock(ctx, blockNo, items[from:to])
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// Record what was shown before serving it, so the recall is graded
	// against the server's record rather than against whatever the
	// client later claims it was asked to hold.
	if err := h.users.DTRecordBlockDigits(ctx, sess.ID, block); err != nil {
		h.log.Error("decision test: record digits", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("could not prepare the block"))
	}

	return connect.NewResponse(&v1.GetBlockResponse{Block: toProtoBlock(block)}), nil
}

// SubmitAnswer grades one answer and stores it. The response carries no
// verdict, deliberately.
func (h *DecisionTest) SubmitAnswer(
	ctx context.Context, req *connect.Request[v1.SubmitAnswerRequest],
) (*connect.Response[v1.SubmitAnswerResponse], error) {
	sess, err := h.session(ctx, req.Msg.GetSessionKey())
	if err != nil {
		return nil, err
	}
	m := req.Msg
	blockNo := int(m.GetBlockNo())
	pos := int(m.GetPositionOverall())

	items, err := h.users.DTScoredItems(ctx)
	if err != nil || pos < 1 || pos > len(items) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("question %d is not part of this test", pos))
	}
	item := items[pos-1]

	if err := h.users.DTSaveAnswer(ctx, sess.ID, blockNo, pos,
		int(m.GetChosenIndex()), int(m.GetLatencyMs()), int(m.GetConfidence()), item); err != nil {
		h.log.Error("decision test: save answer", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("could not record the answer"))
	}
	return connect.NewResponse(&v1.SubmitAnswerResponse{Stored: true}), nil
}

// SubmitRecall stores the digits returned at the end of a block.
//
// A missed recall is a data point rather than a failure, so nothing
// here refuses, retries or tells the participant anything.
func (h *DecisionTest) SubmitRecall(
	ctx context.Context, req *connect.Request[v1.SubmitRecallRequest],
) (*connect.Response[v1.SubmitRecallResponse], error) {
	sess, err := h.session(ctx, req.Msg.GetSessionKey())
	if err != nil {
		return nil, err
	}
	blockNo := int(req.Msg.GetBlockNo())
	outcome, held, err := h.users.DTSaveRecall(ctx, sess.ID, blockNo,
		strings.TrimSpace(req.Msg.GetDigits()), int(req.Msg.GetLatencyMs()))
	if err != nil {
		h.log.Error("decision test: save recall", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("could not record the recall"))
	}

	// The recall closes the block, so this is where one block's worth of
	// progress is recorded. It is the only trace an abandoned run leaves
	// of how far it got: dt_sessions would say nothing but "running".
	// A tally failure is logged and dropped rather than returned, because
	// an event is not worth failing a recall over.
	if correct, expired, err := h.users.DTBlockTally(ctx, sess.ID, blockNo); err != nil {
		h.log.Warn("decision test: block tally", slog.String("error", err.Error()))
	} else {
		h.events.Emit(ctx, requestEvent(req, "dtest.block_finished", 0, map[string]any{
			"block_no":       blockNo,
			"load":           users.DTBlockLoad(blockNo),
			"correct":        correct,
			"expired":        expired,
			"recall_outcome": outcome,
			"digits_held":    held,
		}))
	}
	return connect.NewResponse(&v1.SubmitRecallResponse{Stored: true}), nil
}

// FinishSession closes the run and returns the only figure a
// participant ever sees: how many of the thirty were correct, never
// item by item.
func (h *DecisionTest) FinishSession(
	ctx context.Context, req *connect.Request[v1.FinishSessionRequest],
) (*connect.Response[v1.FinishSessionResponse], error) {
	sess, err := h.session(ctx, req.Msg.GetSessionKey())
	if err != nil {
		return nil, err
	}
	strategy := ""
	switch req.Msg.GetRecallStrategy() {
	case "encode", "defer":
		strategy = req.Msg.GetRecallStrategy()
	}
	correct, total, err := h.users.DTFinish(ctx, sess.ID, strategy)
	if err != nil {
		h.log.Error("decision test: finish", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("could not close the session"))
	}
	h.log.Info("decision test finished",
		slog.String("session", sess.PublicID), slog.Int("correct", correct), slog.Int("total", total))

	if fin, err := h.users.DTSessionSummary(ctx, sess.ID); err != nil {
		h.log.Warn("decision test: summary for the event", slog.String("error", err.Error()))
	} else {
		h.events.Emit(ctx, requestEvent(req, "dtest.finished", 0, map[string]any{
			"correct":       correct,
			"answered":      total,
			"expired":       fin.Expired,
			"duration_s":    fin.DurationS,
			"wants_results": fin.WantsResults,
		}))
	}

	// Sent after the response, not before it. A participant who has just
	// given fifteen minutes should see the thank-you immediately; an
	// email provider having a slow afternoon is not their problem, and
	// a failed send must not turn a completed run into an error.
	go h.sendResult(sess.ID)
	return connect.NewResponse(&v1.FinishSessionResponse{
		Correct: int32(correct), Total: int32(total),
	}), nil
}

// session resolves the handle and refuses a run that is already closed.
func (h *DecisionTest) session(ctx context.Context, key string) (*users.DTSession, error) {
	if h.users == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("decision test is not configured"))
	}
	if key == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("no session"))
	}
	s, err := h.users.DTSessionByKey(ctx, key)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no such session"))
	}
	// A completed run cannot be added to. There is no resume (FSD §5b):
	// resuming would poison the timing control the instrument depends
	// on, since a question answered after a four-minute interruption is
	// not the same question and nothing in the data would say so.
	if s.Status != "running" {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("this session is already %s", s.Status))
	}
	return s, nil
}

// practiceBlock prepares the unscored warm-up.
//
// The practice items carry lures of their own. A gentle practice block
// would rehearse a different experience from the test and leave the
// first real trap arriving at question one, which is the worst place
// for it.
func (h *DecisionTest) practiceBlock(ctx context.Context, sessionID int64) (*v1.Block, error) {
	items, err := h.users.DTPracticeItems(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.Block{
		BlockNo:   0,
		Load:      "practice",
		Digits:    "",
		Transform: "none",
		Questions: toProtoQuestions(items, 0),
	}, nil
}

// toProtoBlock copies a block onto the wire WITHOUT its key.
//
// This function is the boundary. If an answer index ever appears in a
// response, it will be because something was added here, so adding to
// it deserves a second look.
func toProtoBlock(b *users.DTBlock) *v1.Block {
	return &v1.Block{
		BlockNo:   int32(b.BlockNo),
		Load:      b.Load,
		Digits:    b.Digits,
		Transform: b.Transform,
		Questions: toProtoQuestions(b.Items, b.FirstPosition),
	}
}

// toProtoQuestions copies prompt, reminder and options across and
// leaves CorrectIndex and LureIndex behind.
func toProtoQuestions(items []users.DTItem, firstPosition int) []*v1.Question {
	out := make([]*v1.Question, 0, len(items))
	for i, it := range items {
		pos := 0
		if firstPosition > 0 {
			pos = firstPosition + i
		}
		out = append(out, &v1.Question{
			Code:            it.Code,
			Version:         int32(it.Version),
			Prompt:          it.Prompt,
			Reminder:        it.Reminder,
			Options:         it.Options,
			PositionOverall: int32(pos),
		})
	}
	return out
}

// sendResult mails one participant their figures, if they asked.
//
// Detached from the request, with its own context: the caller's is
// cancelled the moment the response is written.
func (h *DecisionTest) sendResult(sessionID int64) {
	if h.email == nil || h.from == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wants, addr, err := h.users.DTWantsResults(ctx, sessionID)
	if err != nil {
		h.log.Warn("decision test: result email, wants check",
			slog.Int64("session_id", sessionID), slog.String("error", err.Error()))
		return
	}
	// Nothing is sent to somebody who did not ask. That is the
	// commitment the intake form makes and the privacy policy states,
	// and it is enforced here rather than trusted to the form.
	if !wants {
		return
	}

	res, err := h.users.DTResultFor(ctx, sessionID)
	if err != nil {
		h.log.Error("decision test: result email, figures",
			slog.Int64("session_id", sessionID), slog.String("error", err.Error()))
		return
	}

	text, html, err := email.DecisionTestResultTemplate.Render(struct {
		*users.DTResult
		AccuracyDrop   int
		ConfidenceDrop int
		Gap            int
		FatigueHeld    bool
		SiteURL        string
	}{
		DTResult:       res,
		AccuracyDrop:   res.AccuracyDrop(),
		ConfidenceDrop: res.ConfidenceDrop(),
		Gap:            res.Gap(),
		FatigueHeld:    res.FatigueHeld(),
		SiteURL:        h.siteURL,
	})
	if err != nil {
		h.log.Error("decision test: result email, render", slog.String("error", err.Error()))
		return
	}

	if err := h.email.Send(ctx, email.Message{
		To: addr, From: h.from,
		// Kind is what makes the delivery log answerable. Without it
		// the row lands as "unknown", and the question a participant
		// will eventually ask ("did my results ever go out?") has no
		// query. Found on the first real send, 2026-10-05, where the
		// email arrived and the audit row said nothing about what it
		// was.
		Kind:     "decision_test_result",
		Subject:  "Your results from the decision test",
		TextBody: text, HTMLBody: html,
	}); err != nil {
		h.log.Error("decision test: result email, send",
			slog.String("to_domain", emailDomain(addr)), slog.String("error", err.Error()))
		return
	}
	h.log.Info("decision test result sent", slog.Int64("session_id", sessionID))
}

// emailDomain returns just the domain, for logs. The address itself is
// participant data and does not belong in a log line.
func emailDomain(addr string) string {
	if i := strings.LastIndex(addr, "@"); i >= 0 {
		return addr[i+1:]
	}
	return "unknown"
}
