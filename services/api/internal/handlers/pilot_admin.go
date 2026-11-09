package handlers

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/pilot"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// The admin half of the pilot: /admin/pilot (FSD FR-PLT-11, FR-PLT-12).
// Reads and exports only. Exports name participants by a P-NN code
// assigned by session order, never by name or address, because they
// leave the site for the owner's private folder.

func (h *Pilot) summaryProto(ctx context.Context, m *users.PilotSessionSummary) *v1.PilotSessionSummary {
	out := &v1.PilotSessionSummary{
		Session:      h.sessionProto(ctx, &m.Session),
		MemberId:     strconv.FormatInt(m.Session.UserID, 10),
		Name:         m.Name,
		Email:        m.Email,
		CommentCount: m.CommentCount,
		FlagCount:    m.FlagCount,
	}
	if m.Session.SelfReportedMinutes != nil {
		out.SelfReportedMinutes = *m.Session.SelfReportedMinutes
	}
	if m.FirstOpened != nil && m.LastSubmit != nil {
		out.MeasuredMinutes = pilot.MeasuredMinutes(m.FirstOpened.Unix(), m.LastSubmit.Unix())
	}
	out.ReviewStatus = m.Session.ReviewStatus
	out.ReviewNote = m.Session.ReviewNote
	if m.Session.ReviewedAt != nil {
		out.ReviewedAt = timestamppb.New(*m.Session.ReviewedAt)
	}
	return out
}

// notesProto is the wire form of the owner's notes.
func notesProto(list []users.PilotNote) []*v1.PilotNote {
	out := make([]*v1.PilotNote, 0, len(list))
	for _, n := range list {
		out = append(out, &v1.PilotNote{TargetKind: n.TargetKind, TargetId: n.TargetID, Body: n.Body, Author: n.Author, UpdatedAt: timestamppb.New(n.UpdatedAt)})
	}
	return out
}

// ListPilotSessions lists every session for the admin page.
func (h *Pilot) ListPilotSessions(ctx context.Context, _ *connect.Request[v1.ListPilotSessionsRequest]) (*connect.Response[v1.ListPilotSessionsResponse], error) {
	list, err := h.users.ListPilotSessionSummaries(ctx)
	if err != nil {
		h.log.Error("ListPilotSessions failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("list failed"))
	}
	out := &v1.ListPilotSessionsResponse{InstrumentVersion: h.instrument.Version}
	for i := range list {
		out.Sessions = append(out.Sessions, h.summaryProto(ctx, &list[i]))
	}
	return connect.NewResponse(out), nil
}

// GetPilotSession returns one session in full.
func (h *Pilot) GetPilotSession(ctx context.Context, req *connect.Request[v1.GetPilotSessionRequest]) (*connect.Response[v1.GetPilotSessionResponse], error) {
	list, err := h.users.ListPilotSessionSummaries(ctx)
	if err != nil {
		h.log.Error("GetPilotSession failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("lookup failed"))
	}
	var m *users.PilotSessionSummary
	for i := range list {
		if list[i].Session.ID == req.Msg.SessionId {
			m = &list[i]
			break
		}
	}
	if m == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such session"))
	}
	answers, err := h.users.ListPilotAnswers(ctx, m.Session.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("lookup failed"))
	}
	comments, err := h.users.ListPilotComments(ctx, m.Session.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("lookup failed"))
	}
	events, err := h.users.ListPilotSectionEvents(ctx, m.Session.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("lookup failed"))
	}
	notes, err := h.users.ListPilotNotes(ctx, m.Session.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("lookup failed"))
	}
	out := &v1.GetPilotSessionResponse{Summary: h.summaryProto(ctx, m), Notes: notesProto(notes)}
	for _, a := range answers {
		pa := &v1.PilotAnswer{ItemId: a.ItemID, ValueJson: a.ValueJSON, Flagged: a.Flagged, Section: a.Section}
		// The question text, so the owner's review reads as questions.
		if it, sec := h.instrument.Item(a.ItemID); it != nil {
			pa.Prompt = it.Prompt
			if sec != nil && pa.Section == "" {
				pa.Section = sec.ID
			}
		}
		out.Answers = append(out.Answers, pa)
	}
	out.Comments = commentsProto(comments, "")
	for _, e := range events {
		out.Events = append(out.Events, &v1.PilotSectionEvent{Section: e.Section, Event: e.Event, At: timestamppb.New(e.At)})
	}
	return connect.NewResponse(out), nil
}

// ExportPilot writes one table as CSV, participants as P-NN codes.
func (h *Pilot) ExportPilot(ctx context.Context, req *connect.Request[v1.ExportPilotRequest]) (*connect.Response[v1.ExportPilotResponse], error) {
	list, err := h.users.ListPilotSessionSummaries(ctx)
	if err != nil {
		h.log.Error("ExportPilot failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("export failed"))
	}
	// Codes by session order, oldest first, so a code is stable as long
	// as no earlier session is deleted, which DeletePilotSession is meant
	// to do only before the ten start (FR-PLT-15).
	sort.Slice(list, func(i, j int) bool { return list[i].Session.CreatedAt.Before(list[j].Session.CreatedAt) })
	code := map[string]string{}
	for i := range list {
		code[list[i].Session.ID] = pilot.ParticipantCode(i + 1)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	rows := 0
	ts := func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return t.UTC().Format(time.RFC3339)
	}
	switch req.Msg.Table {
	case "sessions":
		_ = w.Write([]string{"participant", "instrument_version", "role_branch", "status", "consent_at", "recording_ok",
			"survey_finished_at", "record_finished_at", "workflow_finished_at", "finished_at",
			"measured_minutes", "self_reported_minutes", "most_wanted_to_skip", "comment_count", "flag_count",
			"review_status", "review_note", "reviewed_at"})
		for i := range list {
			m := &list[i]
			s := &m.Session
			measured := int32(0)
			if m.FirstOpened != nil && m.LastSubmit != nil {
				measured = pilot.MeasuredMinutes(m.FirstOpened.Unix(), m.LastSubmit.Unix())
			}
			self := ""
			if s.SelfReportedMinutes != nil {
				self = strconv.Itoa(int(*s.SelfReportedMinutes))
			}
			_ = w.Write([]string{code[s.ID], s.InstrumentVersion, s.RoleBranch, s.Status, s.ConsentAt.UTC().Format(time.RFC3339),
				strconv.FormatBool(s.RecordingOK), ts(s.SurveyFinishedAt), ts(s.RecordFinishedAt), ts(s.WorkflowFinishedAt), ts(s.FinishedAt),
				strconv.Itoa(int(measured)), self, s.MostWantedToSkip, strconv.Itoa(int(m.CommentCount)), strconv.Itoa(int(m.FlagCount)),
				s.ReviewStatus, s.ReviewNote, ts(s.ReviewedAt)})
			rows++
		}
	case "answers":
		_ = w.Write([]string{"participant", "section", "item_id", "value_json", "flagged", "answered_at"})
		for i := range list {
			answers, err := h.users.ListPilotAnswers(ctx, list[i].Session.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.New("export failed"))
			}
			for _, a := range answers {
				_ = w.Write([]string{code[list[i].Session.ID], a.Section, a.ItemID, a.ValueJSON, strconv.FormatBool(a.Flagged), a.AnsweredAt.UTC().Format(time.RFC3339)})
				rows++
			}
		}
	case "comments":
		_ = w.Write([]string{"participant", "target_kind", "target_id", "body", "updated_at"})
		for i := range list {
			comments, err := h.users.ListPilotComments(ctx, list[i].Session.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.New("export failed"))
			}
			for _, c := range comments {
				_ = w.Write([]string{code[list[i].Session.ID], c.TargetKind, c.TargetID, c.Body, c.UpdatedAt.UTC().Format(time.RFC3339)})
				rows++
			}
		}
	case "notes":
		_ = w.Write([]string{"participant", "target_kind", "target_id", "body", "author", "updated_at"})
		for i := range list {
			notes, err := h.users.ListPilotNotes(ctx, list[i].Session.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.New("export failed"))
			}
			for _, n := range notes {
				_ = w.Write([]string{code[list[i].Session.ID], n.TargetKind, n.TargetID, n.Body, n.Author, n.UpdatedAt.UTC().Format(time.RFC3339)})
				rows++
			}
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("table is sessions, answers, comments or notes"))
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("export failed"))
	}
	return connect.NewResponse(&v1.ExportPilotResponse{Csv: buf.String(), RowCount: int32(rows)}), nil
}

// uuidShape is the only session id the delete accepts. A cast of an
// arbitrary string to uuid fails inside Postgres with an error that
// would read as "lookup failed"; this turns it into a 400 instead.
var uuidShape = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// DeletePilotSession removes one session and, by cascade, everything
// it holds (FSD FR-PLT-15). It exists for the owner's own runs on the
// deployed build. Nothing stops an admin deleting a participant's
// session, because the owner is the only admin; the audit row the
// interceptor writes and the event below record whose it was.
func (h *Pilot) DeletePilotSession(ctx context.Context, req *connect.Request[v1.DeletePilotSessionRequest]) (*connect.Response[v1.DeletePilotSessionResponse], error) {
	admin, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	id := req.Msg.SessionId
	if !uuidShape.MatchString(id) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("session_id must be a UUID"))
	}
	s, err := h.users.GetPilotSessionByID(ctx, id)
	if errors.Is(err, users.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such session"))
	}
	if err != nil {
		h.log.Error("DeletePilotSession lookup failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("lookup failed"))
	}
	deleted, err := h.users.DeletePilotSession(ctx, s.ID)
	if err != nil {
		h.log.Error("DeletePilotSession failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("delete failed"))
	}
	if !deleted {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such session"))
	}
	h.log.Info("pilot session deleted", slog.String("session_id", s.ID), slog.Int64("user_id", s.UserID), slog.Int64("by", admin.ID))
	h.events.Emit(ctx, requestEvent(req, "pilot.session_deleted", admin.ID, map[string]any{"user_id": s.UserID}))
	return connect.NewResponse(&v1.DeletePilotSessionResponse{MemberId: strconv.FormatInt(s.UserID, 10)}), nil
}

// UpsertPilotNote saves one of the owner's notes (FSD FR-PLT-16). The
// target must exist: an item in the instrument, a row in the record, a
// node in the workflow; the session and general kinds take no target.
func (h *Pilot) UpsertPilotNote(ctx context.Context, req *connect.Request[v1.UpsertPilotNoteRequest]) (*connect.Response[v1.UpsertPilotNoteResponse], error) {
	admin, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	id := req.Msg.SessionId
	if !uuidShape.MatchString(id) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("session_id must be a UUID"))
	}
	if _, err := h.users.GetPilotSessionByID(ctx, id); err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("no such session"))
		}
		return nil, connect.NewError(connect.CodeInternal, errors.New("lookup failed"))
	}
	kind, target := req.Msg.TargetKind, strings.TrimSpace(req.Msg.TargetId)
	switch kind {
	case "session", "record_general", "workflow_general":
		target = ""
	case "item":
		if it, _ := h.instrument.Item(target); it == nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no such survey item"))
		}
	case "record_row":
		if !h.record.HasRow(target) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no such record row"))
		}
	case "workflow_node":
		if !h.workflow.HasNode(target) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no such workflow node"))
		}
	case "workflow_missing":
		if target == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a missing-step note needs its number"))
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unknown target kind"))
	}
	body := strings.TrimSpace(req.Msg.Body)
	n, err := h.users.UpsertPilotNote(ctx, id, kind, target, body, admin.ID)
	if err != nil {
		h.log.Error("pilot note upsert failed", slog.String("session", id), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not save the note"))
	}
	if n == nil {
		return connect.NewResponse(&v1.UpsertPilotNoteResponse{Removed: true}), nil
	}
	return connect.NewResponse(&v1.UpsertPilotNoteResponse{UpdatedAt: timestamppb.New(n.UpdatedAt)}), nil
}

// SetPilotSessionReview records the owner's curation judgement on a
// session (FSD FR-PLT-17). Excluding needs a reason; the other two
// take one.
func (h *Pilot) SetPilotSessionReview(ctx context.Context, req *connect.Request[v1.SetPilotSessionReviewRequest]) (*connect.Response[v1.SetPilotSessionReviewResponse], error) {
	admin, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	id := req.Msg.SessionId
	if !uuidShape.MatchString(id) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("session_id must be a UUID"))
	}
	status, note := req.Msg.Status, strings.TrimSpace(req.Msg.Note)
	if status == "excluded" && note == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("say why the session is excluded"))
	}
	if err := h.users.SetPilotSessionReview(ctx, id, status, note, admin.ID); err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("no such session"))
		}
		h.log.Error("pilot review set failed", slog.String("session", id), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not save the judgement"))
	}
	list, err := h.users.ListPilotSessionSummaries(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("lookup failed"))
	}
	for i := range list {
		if list[i].Session.ID == id {
			return connect.NewResponse(&v1.SetPilotSessionReviewResponse{Summary: h.summaryProto(ctx, &list[i])}), nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, errors.New("no such session"))
}
