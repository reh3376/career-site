package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// The decision test's timings, on the admin console.
//
// These were compiled-in constants until the owner had taken the test
// twice and moved them both times. Each move cost a build, a CI run and
// a deploy to change one number, which is the wrong shape for a value
// that is a judgement made from watching people rather than a property
// of the code. Same reasoning as the JD fit bands.
//
// The response carries the derived instrument version and a count of
// sessions already recorded under it, because the cost of changing a
// timing is not the edit, it is that every run before the change becomes
// a different instrument. Showing the count makes that cost visible at
// the moment of deciding rather than at the moment of analysis.

// GetDecisionTestSettings reads the timings.
func (a *Admin) GetDecisionTestSettings(
	ctx context.Context, req *connect.Request[v1.GetDecisionTestSettingsRequest],
) (*connect.Response[v1.GetDecisionTestSettingsResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	s := a.users.DTGetSettings(ctx)
	version := s.InstrumentVersion()
	n, err := a.users.DTSessionsOnVersion(ctx, version)
	if err != nil {
		a.log.Warn("decision test settings: session count", slog.String("error", err.Error()))
	}
	return connect.NewResponse(&v1.GetDecisionTestSettingsResponse{
		MemoriseMs:            int32(s.MemoriseMs),
		QuestionMs:            int32(s.QuestionMs),
		RecallMs:              int32(s.RecallMs),
		InstrumentVersion:     version,
		SessionsOnThisVersion: int32(n),
	}), nil
}

// SetDecisionTestSettings stores them.
func (a *Admin) SetDecisionTestSettings(
	ctx context.Context, req *connect.Request[v1.SetDecisionTestSettingsRequest],
) (*connect.Response[v1.SetDecisionTestSettingsResponse], error) {
	admin, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	s := users.DTSettings{
		MemoriseMs: int(req.Msg.GetMemoriseMs()),
		QuestionMs: int(req.Msg.GetQuestionMs()),
		RecallMs:   int(req.Msg.GetRecallMs()),
	}
	if err := a.users.DTSetSettings(ctx, s, admin.ID); err != nil {
		a.log.Error("decision test settings: save", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("could not save the timings"))
	}
	// Read back rather than echo: the values are clamped on write, so
	// what was asked for and what is in force can differ.
	saved := a.users.DTGetSettings(ctx)
	a.log.Info("decision test timings changed",
		slog.String("instrument_version", saved.InstrumentVersion()),
		slog.Int("memorise_ms", saved.MemoriseMs),
		slog.Int("question_ms", saved.QuestionMs),
		slog.Int("recall_ms", saved.RecallMs))
	return connect.NewResponse(&v1.SetDecisionTestSettingsResponse{
		InstrumentVersion: saved.InstrumentVersion(),
	}), nil
}

// ListDecisionTestRuns returns runs, newest first.
func (a *Admin) ListDecisionTestRuns(
	ctx context.Context, req *connect.Request[v1.ListDecisionTestRunsRequest],
) (*connect.Response[v1.ListDecisionTestRunsResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	runs, err := a.users.DTListRuns(ctx, req.Msg.GetIncludeSynthetic(), req.Msg.GetReviewStatus())
	if err != nil {
		a.log.Error("decision test: list runs", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("could not read the runs"))
	}
	out := make([]*v1.DecisionTestRun, 0, len(runs))
	for _, r := range runs {
		out = append(out, toProtoRun(r))
	}
	// Counted across every real run rather than the filtered page, so
	// "14 left to review" stays true while looking at one status.
	counts, err := a.users.DTReviewSummary(ctx)
	if err != nil {
		a.log.Error("decision test: review summary", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("could not read the runs"))
	}
	return connect.NewResponse(&v1.ListDecisionTestRunsResponse{
		Runs: out,
		Counts: &v1.DecisionTestReviewCounts{
			Total: int32(counts.Total), Unreviewed: int32(counts.Unreviewed),
			Good: int32(counts.Good), Incomplete: int32(counts.Incomplete),
			Hold: int32(counts.Hold), DoNotUse: int32(counts.DoNotUse),
		},
	}), nil
}

// GetDecisionTestRun returns one run in full.
func (a *Admin) GetDecisionTestRun(
	ctx context.Context, req *connect.Request[v1.GetDecisionTestRunRequest],
) (*connect.Response[v1.GetDecisionTestRunResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	run, blocks, answers, err := a.users.DTGetRun(ctx, req.Msg.GetSessionKey())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no such run"))
	}
	// Per-block judgements, keyed by block. A missing entry is a block
	// nobody has looked at, which is why this is a map rather than a
	// field defaulted on the summary: absent and "judged fine" are
	// different states and the surface has to show which.
	reviews, err := a.users.DTBlockReviews(ctx, req.Msg.GetSessionKey())
	if err != nil {
		a.log.Warn("decision test: block reviews", slog.String("error", err.Error()))
		reviews = nil
	}
	pb := make([]*v1.DecisionTestBlock, 0, len(blocks))
	for _, b := range blocks {
		pb = append(pb, &v1.DecisionTestBlock{
			BlockNo: int32(b.BlockNo), Load: b.Load,
			Correct: int32(b.Correct), Total: int32(b.Total),
			Lure: int32(b.Lure), Expired: int32(b.Expired),
			MeanConfidence: int32(b.MeanConfidence), MeanLatencyMs: int32(b.MeanLatencyMs),
			PresentedDigits: b.PresentedDigits, ExpectedDigits: b.ExpectedDigits,
			ResponseDigits: b.ResponseDigits, RecallOutcome: b.RecallOutcome,
			MemoryFailurePct:      int32(b.MemoryFailure * 100),
			ReviewStatus:          reviews[b.BlockNo].Status,
			ReviewNote:            reviews[b.BlockNo].Note,
			DigitsCorrect:         int32(b.DigitsCorrect),
			DigitsHeld:            int32(b.DigitsHeld),
			RecallLatencyMs:       int32(b.RecallLatencyMs),
			MeanLatencyVsBaseline: b.MeanLatencyVsBaseline,
		})
	}
	pa := make([]*v1.DecisionTestAnswer, 0, len(answers))
	for _, x := range answers {
		pa = append(pa, &v1.DecisionTestAnswer{
			Position: int32(x.Position), BlockNo: int32(x.BlockNo),
			ItemCode: x.ItemCode, ItemFamily: x.ItemFamily, Outcome: x.Outcome,
			Confidence: int32(x.Confidence), LatencyMs: int32(x.LatencyMs),
			Prompt: x.Prompt, ChosenText: x.ChosenText, CorrectText: x.CorrectText,
			ChosenIndex: int32(x.ChosenIndex), PositionInBlock: int32(x.PositionInBlock),
			ItemVersion: int32(x.ItemVersion), IsLure: x.IsLure,
			ConfidentlyWrong: x.ConfidentlyWrong, ConfidentlyLured: x.ConfidentlyLured,
			LatencyVsBaseline: x.LatencyVsBaseline, Brier: x.Brier,
		})
	}

	// The curation history. A read failure here must not take the page
	// down: the run and its answers are the point, and a missing audit
	// trail is worth a warning rather than a 500 on the one surface a
	// reviewer uses.
	history, err := a.users.DTReviewHistory(ctx, req.Msg.GetSessionKey())
	if err != nil {
		a.log.Warn("decision test: review history", slog.String("error", err.Error()))
	}
	ph := make([]*v1.DecisionTestReviewEvent, 0, len(history))
	for _, e := range history {
		ph = append(ph, &v1.DecisionTestReviewEvent{
			BlockNo: int32(e.BlockNo), Status: e.Status, Reason: e.Reason,
			Note: e.Note, ReviewedBy: e.By,
			CreatedAt: e.At.UTC().Format(time.RFC3339),
		})
	}

	return connect.NewResponse(&v1.GetDecisionTestRunResponse{
		Run: toProtoRun(*run), Blocks: pb, Answers: pa, ReviewHistory: ph,
	}), nil
}

// ExportDecisionTestRun hands over one run as two CSV files.
//
// Thin, like its neighbour: everything that decides what the files
// contain is in users.BuildDTRunCSV, which is handed exactly what
// DTGetRun gave the page, so the download and the screen cannot
// disagree about a single number.
func (a *Admin) ExportDecisionTestRun(
	ctx context.Context, req *connect.Request[v1.ExportDecisionTestRunRequest],
) (*connect.Response[v1.ExportDecisionTestRunResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	out, err := a.users.DTExportRun(ctx, req.Msg.GetSessionKey(), req.Msg.GetIncludeKey())
	if err != nil {
		a.log.Error("decision test: export run", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no such run"))
	}
	a.log.Info("decision test run exported",
		slog.String("session", req.Msg.GetSessionKey()),
		slog.Bool("include_key", req.Msg.GetIncludeKey()),
		slog.Int("answer_rows", out.AnswerRows))
	return connect.NewResponse(&v1.ExportDecisionTestRunResponse{
		FilenameStem: out.FilenameStem,
		BlocksCsv:    out.BlocksCSV,
		AnswersCsv:   out.AnswersCSV,
		BlockRows:    int32(out.BlockRows),
		AnswerRows:   int32(out.AnswerRows),
	}), nil
}

// ExportDecisionTestData hands over the curated dataset as CSV.
//
// Thin on purpose. Everything that decides what the dataset contains is
// in v_dt_answers and in users.DTExportCSV, so there is no place here
// where a column could be added, filtered or renamed on its way out.
func (a *Admin) ExportDecisionTestData(
	ctx context.Context, req *connect.Request[v1.ExportDecisionTestDataRequest],
) (*connect.Response[v1.ExportDecisionTestDataResponse], error) {
	// This call was shipped without it on 2026-10-05 and the entire
	// dataset was downloadable by anyone, with no session, for about
	// five hours. The proto declares AUTH_LEVEL_ADMIN and that is not
	// enforcement: the comment on the Admin type says so in as many
	// words, because the auth interceptor has not landed and every
	// method here gates itself.
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	if a.users == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("not configured"))
	}
	out, err := a.users.DTExportCSV(ctx, req.Msg.GetIncludeSynthetic(), req.Msg.GetIncludeExcluded())
	if err != nil {
		a.log.Error("decision test: export", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("could not build the export"))
	}
	a.log.Info("decision test exported",
		slog.Int("rows", out.Rows), slog.Int("sessions", out.Sessions),
		slog.Bool("include_synthetic", req.Msg.GetIncludeSynthetic()),
		slog.Bool("include_excluded", req.Msg.GetIncludeExcluded()))
	return connect.NewResponse(&v1.ExportDecisionTestDataResponse{
		Csv: out.CSV, Filename: out.Filename,
		Rows: int32(out.Rows), Sessions: int32(out.Sessions),
	}), nil
}

// ReviewDecisionTestRun records the owner's curation judgement.
//
// Session or block, decided by block_no. The judgement is written to
// the row every query filters on and appended to a history, in one
// transaction, so there is never a verdict without a record of how it
// got there.
func (a *Admin) ReviewDecisionTestRun(
	ctx context.Context, req *connect.Request[v1.ReviewDecisionTestRunRequest],
) (*connect.Response[v1.ReviewDecisionTestRunResponse], error) {
	admin, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	key := req.Msg.GetSessionKey()
	rev := users.DTReview{
		BlockNo: int(req.Msg.GetBlockNo()),
		Status:  req.Msg.GetStatus(),
		Reason:  req.Msg.GetReason(),
		Note:    req.Msg.GetNote(),
	}

	switch {
	case rev.BlockNo == 0:
		err = a.users.DTReviewSession(ctx, key, rev, &admin.ID)
	case rev.Status == "":
		// A block has no stored unreviewed state: no row means nobody
		// has judged it. So clearing one is a delete, and the clearing
		// itself is recorded, because an exclusion that quietly
		// disappears is the thing somebody would most want explained.
		err = a.users.DTClearBlockReview(ctx, key, rev.BlockNo, &admin.ID)
	default:
		err = a.users.DTReviewBlock(ctx, key, rev, &admin.ID)
	}
	if err != nil {
		// A rejected vocabulary is the caller's mistake, not a server
		// fault, and saying which word was refused is the whole value
		// of refusing it.
		a.log.Warn("decision test: review", slog.String("session", key),
			slog.Int("block", rev.BlockNo), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	a.log.Info("decision test reviewed",
		slog.String("session", key), slog.Int("block", rev.BlockNo),
		slog.String("status", rev.Status), slog.String("reason", rev.Reason))

	run, _, _, err := a.users.DTGetRun(ctx, key)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("saved, but could not read the run back"))
	}
	return connect.NewResponse(&v1.ReviewDecisionTestRunResponse{Run: toProtoRun(*run)}), nil
}

// toProtoRun copies a run onto the wire.
//
// gave_email rather than the address itself: the admin list needs to
// know whether results can be sent, not what the address is, and a
// mailing list is not what this surface is for.
func toProtoRun(r users.DTRun) *v1.DecisionTestRun {
	return &v1.DecisionTestRun{
		SessionKey: r.SessionKey, Status: r.Status,
		DisplayName: r.DisplayName, AgeRange: r.AgeRange,
		Education: r.Education, Occupation: r.Occupation, GaveEmail: r.GaveEmail,
		AudioMode: r.AudioMode, DeviceClass: r.DeviceClass,
		TapCheckPassed: r.TapCheckPassed, BaselineRtMs: int32(r.BaselineRTMs),
		IsRepeat: r.IsRepeat, IsSynthetic: r.IsSynthetic,
		InstrumentVersion: r.InstrumentVersion, ItemSetVersion: r.ItemSetVersion,
		Correct: int32(r.Correct), Answered: int32(r.Answered), Expired: int32(r.Expired),
		MeanConfidence: int32(r.MeanConfidence), DurationS: int32(r.DurationS),
		StartedAt:    r.StartedAt.UTC().Format(time.RFC3339),
		ReviewStatus: r.ReviewStatus, ReviewReason: r.ReviewReason,
		ReviewNote: r.ReviewNote, ReviewedAt: rfc3339OrEmpty(r.ReviewedAt),
		BlocksExcluded: int32(r.BlocksExcluded),
		RecallStrategy: r.RecallStrategy, BaselineRtSdMs: int32(r.BaselineRTSDMs),
		RepeatMatchedBy: r.RepeatMatchedBy,
		// attempt_no carries NULL as 0, which is safe because the
		// functions never return 0: the first sitting is 1, so 0 can
		// only mean "no identity could place this run" and cannot be
		// mistaken for "first".
		AttemptNo:              int32(derefOr(r.AttemptNo, 0)),
		AttemptNoStrongest:     int32(derefOr(r.AttemptNoStrongest, 0)),
		AttemptSource:          r.AttemptSource,
		AttemptSourcesDisagree: r.AttemptSourcesDisagree,
		// The deprecated scalars, still populated so nothing that reads
		// them regresses, and still lossy: a zero vanishes from the JSON
		// under implicit presence and reads as "no such link".
		PriorByAccount: int32(derefOr(r.PriorByAccount, 0)),
		PriorByEmail:   int32(derefOr(r.PriorByEmail, 0)),
		PriorByCookie:  int32(derefOr(r.PriorByCookie, 0)),
		// The replacement, where each field has explicit presence and a
		// zero therefore survives. This is what the console reads.
		PriorSittings: &v1.DecisionTestPriorSittings{
			ByAccount: int32Ptr(r.PriorByAccount),
			ByEmail:   int32Ptr(r.PriorByEmail),
			ByCookie:  int32Ptr(r.PriorByCookie),
		},
		KeyVersion:     r.KeyVersion,
		WantsResults:   r.WantsResults,
		FinishedAt:     rfc3339OrEmpty(r.FinishedAt),
		ReviewedByName: r.ReviewedByName,
	}
}

// derefOr reads a nullable int, substituting a value for NULL. Only safe
// where the substitute cannot collide with a real reading.
func derefOr(p *int, missing int) int {
	if p == nil {
		return missing
	}
	return *p
}

// int32Ptr carries NULL through as NULL rather than flattening it.
func int32Ptr(p *int) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p)
	return &v
}

// rfc3339OrEmpty renders a nullable timestamp. Empty rather than the
// zero time, so "never reviewed" does not read as 1 January year one.
func rfc3339OrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
