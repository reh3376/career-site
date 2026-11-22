package handlers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	sidecarv1 "github.com/reh3376/career-site/services/api/gen/career/sidecar/v1"
	"github.com/reh3376/career-site/services/api/internal/dtreport"
	"github.com/reh3376/career-site/services/api/internal/email"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// A store that keeps the report service's state in memory, with the
// one property the real one gets from a unique index: the automatic
// email can be claimed once.
type fakeDTStore struct {
	mu sync.Mutex

	status   string
	wants    bool
	addr     string
	inputErr error

	reports  []users.DTReportBuild
	attempts int
	perm     bool
	lastErr  string

	claimed   bool
	outcome   string
	sentVer   int
	adminSent []int
	due       []int64
}

func (f *fakeDTStore) DTReportInput(context.Context, int64) (*dtreport.Input, error) {
	if f.inputErr != nil {
		return nil, f.inputErr
	}
	one := 1
	run := dtreport.Run{Key: "0f3c9a2e-0000-4000-8000-000000000000", AttemptNo: &one, Usable: true,
		Started: time.Date(2026, 11, 19, 2, 55, 0, 0, time.UTC), Finished: time.Date(2026, 11, 19, 3, 7, 0, 0, time.UTC),
		InstrumentVersion: "v4-m6000-q25000-r20000", ItemSetVersion: "pool-2026-10-07"}
	for i := 1; i <= 30; i++ {
		run.Answers = append(run.Answers, dtreport.Answer{Position: i, BlockNo: (i-1)/6 + 1, Load: "d3", Family: "arithmetic",
			ItemCode: "A1", Outcome: "correct", Confidence: 70, LatencyMs: 12000, Options: 4})
	}
	return &dtreport.Input{Run: run, MinPeers: users.DTReportMinPeers, ConfidentThreshold: 70,
		Email: dtreport.EmailFigures{Reading: "no_decline", FatigueHeld: true}}, nil
}

func (f *fakeDTStore) DTReportSave(_ context.Context, _ int64, b users.DTReportBuild) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reports = append(f.reports, b)
	f.attempts++
	return len(f.reports), nil
}

func (f *fakeDTStore) DTReportFailed(_ context.Context, _ int64, cause error, permanent bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts++
	f.perm = f.perm || permanent
	f.lastErr = cause.Error()
	return nil
}

func (f *fakeDTStore) DTReportsDue(context.Context, time.Duration, int) ([]int64, error) {
	return f.due, nil
}

func (f *fakeDTStore) DTReportStateByID(_ context.Context, id int64) (*users.DTReportState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := &users.DTReportState{SessionID: id, SessionKey: "0f3c9a2e-0000-4000-8000-000000000000", Status: f.status,
		StartedAt: time.Date(2026, 11, 19, 2, 55, 0, 0, time.UTC), Attempts: f.attempts, Permanent: f.perm, LastError: f.lastErr,
		CanSend: f.wants && f.addr != ""}
	if n := len(f.reports); n > 0 {
		st.Report = &users.DTReportRecord{Version: n, Compared: f.reports[n-1].Summary.Compared}
	}
	return st, nil
}

func (f *fakeDTStore) DTReportPDF(_ context.Context, _ int64, owner bool) ([]byte, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := len(f.reports)
	if n == 0 {
		return nil, 0, errors.New("no report")
	}
	if owner {
		return f.reports[n-1].OwnerPDF, n, nil
	}
	return f.reports[n-1].ParticipantPDF, n, nil
}

func (f *fakeDTStore) DTResultEmailClaim(context.Context, int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claimed {
		return false, nil
	}
	f.claimed, f.outcome = true, "claimed"
	return true, nil
}

func (f *fakeDTStore) DTResultEmailSettle(_ context.Context, _ int64, outcome string, version int, _ error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.outcome, f.sentVer = outcome, version
	return nil
}

func (f *fakeDTStore) DTResultEmailRecordAdmin(_ context.Context, _, _ int64, version int, cause error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if cause == nil {
		f.adminSent = append(f.adminSent, version)
	}
	return nil
}

func (f *fakeDTStore) DTWantsResults(context.Context, int64) (bool, string, error) {
	return f.wants && f.addr != "", f.addr, nil
}

func (f *fakeDTStore) DTResultFor(context.Context, int64) (*users.DTResult, error) {
	return &users.DTResult{Correct: 12, Total: 30, EarlyAccuracy: 33, EarlyConfidence: 54, HardAccuracy: 50, HardConfidence: 33,
		Block1Accuracy: 33, Block5Accuracy: 33}, nil
}

type fakeRenderer struct {
	err   error
	calls int
	docs  []string
}

func (r *fakeRenderer) RenderDecisionTestReport(_ context.Context, doc, _ string) (*sidecarv1.RenderDecisionTestReportResponse, error) {
	r.calls++
	r.docs = append(r.docs, doc)
	if r.err != nil {
		return nil, r.err
	}
	return &sidecarv1.RenderDecisionTestReportResponse{ParticipantPdf: []byte("%PDF-participant"), OwnerPdf: []byte("%PDF-owner"),
		ParticipantPages: 6, OwnerPages: 9, Engine: "fake"}, nil
}

type capturingMailer struct {
	mu   sync.Mutex
	sent []email.Message
	err  error
}

func (m *capturingMailer) Name() string { return "capture" }
func (m *capturingMailer) Send(_ context.Context, msg email.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, msg)
	return nil
}

func newDTReportsForTest(store *fakeDTStore, r DTReportRenderer, m email.Provider) *DTReports {
	s := &DTReports{log: slog.New(slog.NewTextHandler(io.Discard, nil)), store: store, renderer: r}
	if m != nil {
		s.SetMailer(m, "Test <test@example.com>", "https://example.com")
	}
	return s
}

// The defect this replaced: the run screen calls FinishSession twice,
// and every participant who asked for results was emailed twice.
func TestTheResultsEmailGoesOnce(t *testing.T) {
	store := &fakeDTStore{status: "completed", wants: true, addr: "p@example.com"}
	renderer, mailer := &fakeRenderer{}, &capturingMailer{}
	s := newDTReportsForTest(store, renderer, mailer)

	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() { defer wg.Done(); s.Process(1) }()
	}
	wg.Wait()
	s.Process(1)

	if len(mailer.sent) != 1 {
		t.Fatalf("the participant was emailed %d times", len(mailer.sent))
	}
	if renderer.calls != 1 || len(store.reports) != 1 {
		t.Fatalf("the report was built %d times and stored %d times", renderer.calls, len(store.reports))
	}
	msg := mailer.sent[0]
	if msg.To != "p@example.com" || msg.Kind != "decision_test_result" {
		t.Fatalf("message: to=%q kind=%q", msg.To, msg.Kind)
	}
	if len(msg.Attachments) != 1 {
		t.Fatalf("%d attachments", len(msg.Attachments))
	}
	att := msg.Attachments[0]
	// The participant's copy and never the owner's, under a name that
	// identifies the run.
	if string(att.Data) != "%PDF-participant" || att.ContentType != "application/pdf" {
		t.Fatalf("attachment: %q %q", att.Data, att.ContentType)
	}
	if att.Filename != "decision-test-results-0f3c9a2e-0000-4000-8000-000000000000.pdf" {
		t.Fatalf("filename: %q", att.Filename)
	}
	if !strings.Contains(msg.TextBody, "The attached PDF goes further") || !strings.Contains(msg.HTMLBody, "The attached PDF goes further") {
		t.Fatal("the body does not mention the attachment")
	}
	if !strings.Contains(msg.TextBody, "You answered 12 of 30 correctly.") {
		t.Fatal("the body lost the results it has always carried")
	}
	if store.outcome != "sent" || store.sentVer != 1 {
		t.Fatalf("settled as %q version %d", store.outcome, store.sentVer)
	}
}

func TestNothingIsSentToSomebodyWhoDidNotAsk(t *testing.T) {
	for _, c := range []struct {
		name  string
		wants bool
		addr  string
	}{{"did not ask", false, "p@example.com"}, {"left no address", true, ""}} {
		t.Run(c.name, func(t *testing.T) {
			store := &fakeDTStore{status: "completed", wants: c.wants, addr: c.addr}
			mailer := &capturingMailer{}
			s := newDTReportsForTest(store, &fakeRenderer{}, mailer)
			s.Process(1)
			if len(mailer.sent) != 0 {
				t.Fatal("an email went to somebody who did not ask for one")
			}
			// The report is still built: the owner's copy is wanted for
			// every run, whoever asked for what.
			if len(store.reports) != 1 {
				t.Fatalf("%d reports", len(store.reports))
			}
			if store.outcome != "not_wanted" {
				t.Fatalf("settled as %q", store.outcome)
			}
			if err := s.Send(context.Background(), 1, 7); !errors.Is(err, ErrDTReportNotWanted) {
				t.Fatalf("a send from the console should be refused too: %v", err)
			}
		})
	}
}

// The renderer is another service. While it is down the email waits,
// and when the build is given up on the results go without the PDF.
func TestARunIsAnsweredWhenTheReportCannotBeBuilt(t *testing.T) {
	store := &fakeDTStore{status: "completed", wants: true, addr: "p@example.com"}
	renderer, mailer := &fakeRenderer{err: errors.New("unavailable")}, &capturingMailer{}
	s := newDTReportsForTest(store, renderer, mailer)

	for i := 1; i < users.DTReportMaxAttempts; i++ {
		s.Process(1)
		if len(mailer.sent) != 0 {
			t.Fatalf("attempt %d: the email went before the build was given up on", i)
		}
		if store.claimed {
			t.Fatalf("attempt %d: the email was claimed while a retry was still owed", i)
		}
	}
	s.Process(1)
	if len(mailer.sent) != 1 {
		t.Fatalf("after %d failed builds the participant should have their results: %d sent", users.DTReportMaxAttempts, len(mailer.sent))
	}
	msg := mailer.sent[0]
	if len(msg.Attachments) != 0 || strings.Contains(msg.TextBody, "The attached PDF") {
		t.Fatal("an email with no report claims to carry one")
	}
	if store.outcome != "sent" || store.sentVer != 0 {
		t.Fatalf("settled as %q version %d", store.outcome, store.sentVer)
	}
	// Nothing more is tried, and nothing more is sent.
	s.Process(1)
	if renderer.calls != users.DTReportMaxAttempts || len(mailer.sent) != 1 {
		t.Fatalf("builds=%d sent=%d", renderer.calls, len(mailer.sent))
	}

	// The renderer comes back and the owner rebuilds from the console:
	// a report now exists and can be sent, and still nothing went twice
	// on its own.
	renderer.err = nil
	if v, err := s.Generate(context.Background(), 1, "admin:7"); err != nil || v != 1 {
		t.Fatalf("generate: %d %v", v, err)
	}
	if err := s.Send(context.Background(), 1, 7); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 2 || len(mailer.sent[1].Attachments) != 1 || mailer.sent[1].TriggeredBy != "admin:7" {
		t.Fatalf("console send: %d sent", len(mailer.sent))
	}
	if len(store.adminSent) != 1 || store.adminSent[0] != 1 {
		t.Fatalf("console send not recorded: %v", store.adminSent)
	}
}

func TestAnIncompleteRunIsNotRetried(t *testing.T) {
	store := &fakeDTStore{status: "completed", wants: true, addr: "p@example.com", inputErr: users.ErrDTRunIncomplete}
	renderer, mailer := &fakeRenderer{}, &capturingMailer{}
	s := newDTReportsForTest(store, renderer, mailer)
	s.Process(1)
	if !store.perm || store.attempts != 1 || renderer.calls != 0 {
		t.Fatalf("permanent=%v attempts=%d renders=%d", store.perm, store.attempts, renderer.calls)
	}
	// Given up on at once, so the results go at once, without a report.
	if len(mailer.sent) != 1 || len(mailer.sent[0].Attachments) != 0 {
		t.Fatalf("sent=%d", len(mailer.sent))
	}
}

func TestARunningSessionIsLeftAlone(t *testing.T) {
	store := &fakeDTStore{status: "running", wants: true, addr: "p@example.com"}
	renderer, mailer := &fakeRenderer{}, &capturingMailer{}
	newDTReportsForTest(store, renderer, mailer).Process(1)
	if renderer.calls != 0 || len(mailer.sent) != 0 || store.claimed {
		t.Fatal("a run that has not finished was reported on")
	}
}

func TestConsoleActions(t *testing.T) {
	store := &fakeDTStore{status: "completed", wants: true, addr: "p@example.com"}
	renderer, mailer := &fakeRenderer{}, &capturingMailer{}
	s := newDTReportsForTest(store, renderer, mailer)

	if err := s.Send(context.Background(), 1, 7); !errors.Is(err, ErrDTReportMissing) {
		t.Fatalf("a send with no report: %v", err)
	}
	// A rebuild is a new version, and it sends nothing.
	for want := 1; want <= 2; want++ {
		v, err := s.Generate(context.Background(), 1, "admin:7")
		if err != nil || v != want {
			t.Fatalf("generate: %d %v", v, err)
		}
	}
	if len(mailer.sent) != 0 || store.claimed {
		t.Fatal("a rebuild from the console sent an email")
	}
	if store.reports[1].TriggeredBy != "admin:7" {
		t.Fatalf("triggered by %q", store.reports[1].TriggeredBy)
	}
	// The document handed to the renderer is the engine's, at the
	// schema the sidecar draws.
	if !strings.Contains(renderer.docs[0], `"schema":1`) || !strings.Contains(renderer.docs[0], `"template":"dt-report-1"`) {
		t.Fatal("the renderer was not handed the report document")
	}

	// The sweep works through whatever is due.
	store.due = []int64{1}
	if err := s.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].TriggeredBy != email.TriggeredBySystem {
		t.Fatalf("the sweep should settle the owed email once: %d", len(mailer.sent))
	}

	// A service with nothing behind it does nothing, quietly: this is
	// what a handler constructed in a test holds.
	var none *DTReports
	none.Process(1)
	if err := none.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	NewDTReports(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil).Process(1)
}

func TestReportFilenames(t *testing.T) {
	// The whole run id, so a file names exactly one run.
	const key = "0f3c9a2e-7b1d-4c5e-9a60-3d2f8e1b4c7a"
	if got := users.DTReportFilename(key, false); got != "decision-test-results-"+key+".pdf" {
		t.Fatal(got)
	}
	if got := users.DTReportFilename(key, true); got != "decision-test-results-"+key+"-owner-copy.pdf" {
		t.Fatal(got)
	}
}
