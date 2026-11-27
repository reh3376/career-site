package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	sidecarv1 "github.com/reh3376/career-site/services/api/gen/career/sidecar/v1"
	"github.com/reh3376/career-site/services/api/internal/dtreport"
	"github.com/reh3376/career-site/services/api/internal/email"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// The decision test's results report, from a finished run to a PDF in
// the participant's inbox (FSD FR-DT-21 to 24).
//
// Three steps, each owned somewhere else: the users package reads the
// rows, dtreport works out what the report says, and the sidecar draws
// it. This file is the order they happen in, and the two promises that
// order has to keep:
//
// A participant is written to once. The run screen calls FinishSession
// twice, and before this every participant who asked for results was
// emailed twice. The send is now claimed in the database first, under a
// unique index, so a second caller finds it taken.
//
// A participant who asked is always answered. The renderer is another
// service and can be down. A build is retried a few times, minutes
// apart, and if it still has not produced a report the results go out
// as they always did, in the body of the email, without the PDF.

// DTReportRenderer draws a report document. The sidecar client is the
// production implementation.
type DTReportRenderer interface {
	RenderDecisionTestReport(ctx context.Context, documentJSON, traceID string) (*sidecarv1.RenderDecisionTestReportResponse, error)
}

// dtReportStore is what the service needs from the database. An
// interface so the order of operations can be tested without one.
type dtReportStore interface {
	DTReportInput(ctx context.Context, sessionID int64) (*dtreport.Input, error)
	DTReportSave(ctx context.Context, sessionID int64, b users.DTReportBuild) (int, error)
	DTReportFailed(ctx context.Context, sessionID int64, cause error, permanent bool) error
	DTReportsDue(ctx context.Context, settle time.Duration, limit int) ([]int64, error)
	DTReportStateByID(ctx context.Context, sessionID int64) (*users.DTReportState, error)
	DTReportPDF(ctx context.Context, sessionID int64, owner bool) ([]byte, int, error)
	DTResultEmailClaim(ctx context.Context, sessionID int64) (bool, error)
	DTResultEmailSettle(ctx context.Context, sessionID int64, outcome string, reportVersion int, cause error) error
	DTResultEmailRecordAdmin(ctx context.Context, sessionID, adminID int64, reportVersion int, cause error) error
	DTWantsResults(ctx context.Context, sessionID int64) (bool, string, error)
	DTResultFor(ctx context.Context, sessionID int64) (*users.DTResult, error)
}

// How long a closed run is left alone before the sweep builds its
// report. The run screen closes a run and then asks how the participant
// held the number; the report quotes the answer. Somebody who answers
// triggers the build themselves, at once. This is for the ones who shut
// the tab on the question.
const dtReportSettle = 2 * time.Minute

// How many runs one sweep takes on. The first sweep after a deploy
// meets every run that finished before reports existed, and working
// through them five a minute is kinder to the box than all at once.
const dtReportSweepBatch = 5

// DTReports builds, keeps and sends the reports.
type DTReports struct {
	log      *slog.Logger
	store    dtReportStore
	renderer DTReportRenderer

	email   email.Provider
	from    string
	siteURL string

	// One run at a time. Builds are rare and the renderer draws one
	// document at a time anyway; what this buys is that the sweep and a
	// participant's own trigger cannot both build the same run.
	mu sync.Mutex
}

// NewDTReports wires the service. A nil renderer leaves results going
// out by email alone, which is the right failure for a deployment
// without the sidecar: nobody waits on a report that cannot be made.
func NewDTReports(log *slog.Logger, repo *users.Repo, renderer DTReportRenderer) *DTReports {
	s := &DTReports{log: log, renderer: renderer}
	if repo != nil {
		s.store = repo
	}
	return s
}

// SetMailer wires the results email. Without it reports are still built
// and nobody is written to.
func (s *DTReports) SetMailer(p email.Provider, from, siteURL string) {
	s.email, s.from, s.siteURL = p, from, siteURL
}

// Process brings one finished run up to date: its report built if it
// has none, and its results email sent if one is owed.
//
// Safe to call any number of times, from the request that finished the
// run and from the sweep alike. Detached from any request: it takes its
// own time budget.
func (s *DTReports) Process(sessionID int64) {
	if s == nil || s.store == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	st, err := s.store.DTReportStateByID(ctx, sessionID)
	if err != nil {
		s.log.Warn("decision test report: state", slog.Int64("session_id", sessionID), slog.String("error", err.Error()))
		return
	}
	if st.Status != "completed" {
		return
	}
	if st.Report == nil && !st.Exhausted() {
		if _, err := s.build(ctx, sessionID, "system"); err != nil {
			s.log.Warn("decision test report: build",
				slog.Int64("session_id", sessionID), slog.String("error", err.Error()))
		}
		if st, err = s.store.DTReportStateByID(ctx, sessionID); err != nil {
			s.log.Warn("decision test report: state after build",
				slog.Int64("session_id", sessionID), slog.String("error", err.Error()))
			return
		}
	}
	// The email waits for the report, but not for ever: once the build
	// has been given up on, the results go without it.
	if st.Report != nil || st.Exhausted() || s.renderer == nil {
		s.deliver(ctx, st)
	}
}

// Sweep is the scheduler's job: every run that still needs a report
// built or an email settled.
func (s *DTReports) Sweep(ctx context.Context) error {
	if s == nil || s.store == nil {
		return nil
	}
	ids, err := s.store.DTReportsDue(ctx, dtReportSettle, dtReportSweepBatch)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.Process(id)
	}
	return nil
}

// Generate builds a run's report again as a new version, for the
// console. It sends nothing.
func (s *DTReports) Generate(ctx context.Context, sessionID int64, triggeredBy string) (int, error) {
	if s == nil || s.store == nil {
		return 0, errors.New("reports are not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.build(ctx, sessionID, triggeredBy)
}

// errNoRenderer is the build failing because there is nothing to draw
// with, as opposed to the renderer being reached and failing.
var errNoRenderer = errors.New("no report renderer is configured")

// build makes one version: read, write the document, draw, store.
func (s *DTReports) build(ctx context.Context, sessionID int64, triggeredBy string) (int, error) {
	fail := func(err error, permanent bool) (int, error) {
		if rerr := s.store.DTReportFailed(ctx, sessionID, err, permanent); rerr != nil {
			s.log.Warn("decision test report: record failure", slog.String("error", rerr.Error()))
		}
		return 0, err
	}
	if s.renderer == nil {
		return fail(errNoRenderer, true)
	}
	in, err := s.store.DTReportInput(ctx, sessionID)
	if err != nil {
		// A run closed without all its answers will never have them.
		return fail(err, errors.Is(err, users.ErrDTRunIncomplete))
	}
	doc, figures := dtreport.Build(*in)
	raw, err := json.Marshal(doc)
	if err != nil {
		return fail(fmt.Errorf("encode the report document: %w", err), true)
	}
	started := time.Now()
	res, err := s.renderer.RenderDecisionTestReport(ctx, string(raw), "dt:"+in.Run.ShortID())
	if err != nil {
		// Not permanent, whatever the code: a renderer from an older
		// build refuses a newer document until the deploy finishes, and
		// that is exactly the failure a retry in two minutes fixes.
		return fail(fmt.Errorf("render: %w", err), false)
	}
	if len(res.GetParticipantPdf()) == 0 || len(res.GetOwnerPdf()) == 0 {
		return fail(errors.New("render: the renderer returned an empty file"), false)
	}
	version, err := s.store.DTReportSave(ctx, sessionID, users.DTReportBuild{
		Template: doc.Template, Engine: res.GetEngine(), Summary: figures.Summary(),
		ParticipantPDF: res.GetParticipantPdf(), OwnerPDF: res.GetOwnerPdf(),
		ParticipantPages: int(res.GetParticipantPages()), OwnerPages: int(res.GetOwnerPages()),
		TriggeredBy: triggeredBy,
	})
	if err != nil {
		return fail(err, false)
	}
	s.log.Info("decision test report built",
		slog.String("session", in.Run.ShortID()), slog.Int("version", version),
		slog.Bool("compared", figures.Compared), slog.String("no_ranking_reason", figures.Reason),
		slog.Int("others", figures.Others), slog.Int("participant_kb", len(res.GetParticipantPdf())/1024),
		slog.Duration("render", time.Since(started)))
	return version, nil
}

// deliver sends the one automatic results email a run is owed.
func (s *DTReports) deliver(ctx context.Context, st *users.DTReportState) {
	claimed, err := s.store.DTResultEmailClaim(ctx, st.SessionID)
	if err != nil {
		s.log.Warn("decision test: result email, claim",
			slog.Int64("session_id", st.SessionID), slog.String("error", err.Error()))
		return
	}
	if !claimed {
		return
	}
	settle := func(outcome string, version int, cause error) {
		if err := s.store.DTResultEmailSettle(ctx, st.SessionID, outcome, version, cause); err != nil {
			s.log.Warn("decision test: result email, settle", slog.String("error", err.Error()))
		}
	}
	wants, addr, err := s.store.DTWantsResults(ctx, st.SessionID)
	if err != nil {
		settle("failed", 0, err)
		return
	}
	// Nothing is sent to somebody who did not ask. That is the
	// commitment the intake form makes and the privacy policy states,
	// and it is enforced here rather than trusted to the form.
	if !wants {
		settle("not_wanted", 0, nil)
		return
	}
	// Settled as failed rather than left unclaimed, so a deployment
	// without a mailer does not present the same runs to every sweep for
	// ever, ahead of the ones that could be served.
	if s.email == nil || s.from == "" {
		settle("failed", 0, errors.New("the results email is not configured"))
		return
	}
	version, err := s.send(ctx, st, addr, email.TriggeredBySystem)
	if err != nil {
		s.log.Error("decision test: result email, send",
			slog.String("to_domain", emailDomain(addr)), slog.String("error", err.Error()))
		settle("failed", version, err)
		return
	}
	settle("sent", version, nil)
	s.log.Info("decision test result sent",
		slog.Int64("session_id", st.SessionID), slog.Int("report_version", version))
}

// ErrDTReportNotWanted refuses a send to somebody who did not ask for
// their results, or left no address to send them to.
var ErrDTReportNotWanted = errors.New("the participant did not ask for their results")

// ErrDTReportMissing refuses a send when there is no report to attach.
var ErrDTReportMissing = errors.New("this run has no report yet")

// Send emails the participant the latest version of their report, from
// the console. Unlike the automatic send it can be repeated, and unlike
// it there must be a report: sending the body alone again would only
// repeat what they were sent when the run finished.
func (s *DTReports) Send(ctx context.Context, sessionID, adminID int64) error {
	if s == nil || s.store == nil || s.email == nil || s.from == "" {
		return errors.New("the results email is not configured")
	}
	st, err := s.store.DTReportStateByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if st.Report == nil {
		return ErrDTReportMissing
	}
	wants, addr, err := s.store.DTWantsResults(ctx, sessionID)
	if err != nil {
		return err
	}
	if !wants {
		return ErrDTReportNotWanted
	}
	version, sendErr := s.send(ctx, st, addr, fmt.Sprintf("admin:%d", adminID))
	if err := s.store.DTResultEmailRecordAdmin(ctx, sessionID, adminID, version, sendErr); err != nil {
		s.log.Warn("decision test: record admin send", slog.String("error", err.Error()))
	}
	return sendErr
}

// send composes and sends the results email, with the participant's
// copy of the report attached when there is one. It returns the version
// attached, zero when the email went without.
func (s *DTReports) send(ctx context.Context, st *users.DTReportState, addr, triggeredBy string) (int, error) {
	res, err := s.store.DTResultFor(ctx, st.SessionID)
	if err != nil {
		return 0, fmt.Errorf("figures: %w", err)
	}
	var attachments []email.Attachment
	version, compared := 0, false
	if st.Report != nil {
		pdf, v, err := s.store.DTReportPDF(ctx, st.SessionID, false)
		if err != nil {
			return 0, fmt.Errorf("report: %w", err)
		}
		version, compared = v, st.Report.Compared
		attachments = append(attachments, email.Attachment{
			Filename:    users.DTReportFilename(st.SessionKey, false),
			ContentType: "application/pdf",
			Data:        pdf,
		})
	}

	// The template is handed finished words and a named reading, not
	// signed numbers to branch on. Which true thing a run supports is a
	// decision about the research, and it belongs in DTResult where it
	// is unit-tested, not in a sign comparison inside an email body.
	text, html, err := email.DecisionTestResultTemplate.Render(struct {
		*users.DTResult
		AccuracyPhrase   string
		ConfidencePhrase string
		Reading          string
		FatigueHeld      bool
		SiteURL          string
		// Whether the report is attached, and whether it ranks the
		// participant, so the body says what the attachment is.
		HasReport bool
		Compared  bool
	}{
		DTResult:         res,
		AccuracyPhrase:   res.AccuracyPhrase(),
		ConfidencePhrase: res.ConfidencePhrase(),
		Reading:          res.Reading(),
		FatigueHeld:      res.FatigueHeld(),
		SiteURL:          s.siteURL,
		HasReport:        version > 0,
		Compared:         compared,
	})
	if err != nil {
		return version, fmt.Errorf("render the email: %w", err)
	}
	return version, s.email.Send(ctx, email.Message{
		To: addr, From: s.from,
		// Kind is what makes the delivery log answerable. Without it
		// the row lands as "unknown", and the question a participant
		// will eventually ask ("did my results ever go out?") has no
		// query. Found on the first real send, 2026-10-05, where the
		// email arrived and the audit row said nothing about what it
		// was.
		Kind:        "decision_test_result",
		TriggeredBy: triggeredBy,
		Subject:     "Your results from the decision test",
		TextBody:    text, HTMLBody: html,
		Attachments: attachments,
	})
}
