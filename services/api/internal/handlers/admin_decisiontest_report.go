package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// The console's side of the decision test report (FSD FR-DT-23): see
// what a run's report says, take either copy, build it again, send it.
//
// Thin, like its neighbours. What a report contains is decided in
// dtreport, when one is built and sent is decided in DTReports, and
// these four functions translate between that and the wire.

// SetDTReports wires the report service the console acts through. The
// same instance the finish handler and the sweep use, so a rebuild from
// the console and a build in the background cannot run over each other.
func (a *Admin) SetDTReports(r *DTReports) { a.dtReports = r }

// dtReportProto describes where a run's report stands.
func (a *Admin) dtReportProto(ctx context.Context, key string) *v1.DecisionTestReport {
	st, err := a.users.DTReportStateByKey(ctx, key)
	if err != nil {
		// The run page is about the run. A report that cannot be read is
		// worth a warning and an honest "none", not a 404 on the page a
		// reviewer came to use.
		a.log.Warn("decision test report: state", slog.String("session", key), slog.String("error", err.Error()))
		return &v1.DecisionTestReport{State: "none"}
	}
	return toProtoDTReport(st)
}

func toProtoDTReport(st *users.DTReportState) *v1.DecisionTestReport {
	out := &v1.DecisionTestReport{
		State:               "none",
		CanSend:             st.CanSend,
		ParticipantFilename: users.DTReportFilename(st.SessionKey, false),
		OwnerFilename:       users.DTReportFilename(st.SessionKey, true),
		EmailedBy:           st.EmailedBy,
	}
	if st.EmailedAt != nil {
		out.EmailedAt = st.EmailedAt.UTC().Format(time.RFC3339)
	}
	switch {
	case st.Report != nil:
		r := st.Report
		out.State = "ready"
		out.Version = int32(r.Version)
		out.GeneratedAt = r.CreatedAt.UTC().Format(time.RFC3339)
		out.Compared = r.Compared
		out.NoRankingReason = r.Reason
		out.Others = int32(r.Others)
		out.Score = int32(r.Summary.Score)
		out.Total = int32(r.Summary.Total)
		out.Rank = int32(r.Summary.Rank)
		out.GroupSize = int32(r.Summary.GroupSize)
		out.Percentile = r.Summary.Percentile
		out.ParticipantPages = int32(r.ParticipantPages)
		out.OwnerPages = int32(r.OwnerPages)
		out.Template = r.Template
		out.Engine = r.Engine
	case st.Attempts > 0:
		// Tried and not built. Still "failed" while retries remain: the
		// console says what went wrong last, and the sweep keeps going.
		out.State = "failed"
		out.Error = st.LastError
	}
	return out
}

// dtReportSession resolves the run a report action is about.
func (a *Admin) dtReportSession(ctx context.Context, key string) (*users.DTSession, error) {
	if a.dtReports == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("reports are not configured"))
	}
	sess, err := a.users.DTSessionByKey(ctx, key)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no such run"))
	}
	return sess, nil
}

// DownloadDecisionTestReport hands over one copy of a run's report.
func (a *Admin) DownloadDecisionTestReport(
	ctx context.Context, req *connect.Request[v1.DownloadDecisionTestReportRequest],
) (*connect.Response[v1.DownloadDecisionTestReportResponse], error) {
	sess, err := a.dtReportSession(ctx, req.Msg.GetSessionKey())
	if err != nil {
		return nil, err
	}
	// Unspecified is the participant's copy. The owner's holds the
	// answer key, and it should take asking for by name to get it.
	owner := req.Msg.GetCopy() == v1.DecisionTestReportCopy_DECISION_TEST_REPORT_COPY_OWNER
	st, err := a.users.DTReportStateByID(ctx, sess.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("could not read the report"))
	}
	if st.Report == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("this run has no report yet"))
	}
	pdf, version, err := a.users.DTReportPDF(ctx, sess.ID, owner)
	if err != nil {
		a.log.Error("decision test report: download", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("could not read the report"))
	}
	a.log.Info("decision test report downloaded",
		slog.String("session", sess.PublicID), slog.Bool("owner_copy", owner), slog.Int("version", version))
	return connect.NewResponse(&v1.DownloadDecisionTestReportResponse{
		Filename: users.DTReportFilename(st.SessionKey, owner),
		Pdf:      pdf,
		Version:  int32(version),
	}), nil
}

// GenerateDecisionTestReport builds a run's report again.
func (a *Admin) GenerateDecisionTestReport(
	ctx context.Context, req *connect.Request[v1.GenerateDecisionTestReportRequest],
) (*connect.Response[v1.GenerateDecisionTestReportResponse], error) {
	admin, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := a.dtReportSession(ctx, req.Msg.GetSessionKey())
	if err != nil {
		return nil, err
	}
	version, err := a.dtReports.Generate(ctx, sess.ID, fmt.Sprintf("admin:%d", admin.ID))
	if err != nil {
		// The failure is the answer the console needs: it is recorded on
		// the run and returned as the report's state, so the page shows
		// what went wrong instead of a generic refusal.
		a.log.Warn("decision test report: generate",
			slog.String("session", sess.PublicID), slog.String("error", err.Error()))
		rep := a.dtReportProto(ctx, sess.PublicID)
		if rep.GetState() != "ready" {
			rep.State, rep.Error = "failed", err.Error()
		} else {
			// An earlier version still stands; say that this build failed
			// without pretending the run has no report.
			rep.Error = err.Error()
		}
		return connect.NewResponse(&v1.GenerateDecisionTestReportResponse{Report: rep}), nil
	}
	a.log.Info("decision test report rebuilt",
		slog.String("session", sess.PublicID), slog.Int("version", version), slog.Int64("admin", admin.ID))
	return connect.NewResponse(&v1.GenerateDecisionTestReportResponse{
		Report: a.dtReportProto(ctx, sess.PublicID),
	}), nil
}

// SendDecisionTestReport emails the participant their report.
func (a *Admin) SendDecisionTestReport(
	ctx context.Context, req *connect.Request[v1.SendDecisionTestReportRequest],
) (*connect.Response[v1.SendDecisionTestReportResponse], error) {
	admin, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := a.dtReportSession(ctx, req.Msg.GetSessionKey())
	if err != nil {
		return nil, err
	}
	switch err := a.dtReports.Send(ctx, sess.ID, admin.ID); {
	case errors.Is(err, ErrDTReportNotWanted), errors.Is(err, ErrDTReportMissing):
		// Refusals with a reason the owner can act on, so the reason is
		// what they are told.
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	case err != nil:
		a.log.Error("decision test report: send",
			slog.String("session", sess.PublicID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("the email could not be sent"))
	}
	a.log.Info("decision test report sent from the console",
		slog.String("session", sess.PublicID), slog.Int64("admin", admin.ID))
	return connect.NewResponse(&v1.SendDecisionTestReportResponse{
		Report: a.dtReportProto(ctx, sess.PublicID),
	}), nil
}
