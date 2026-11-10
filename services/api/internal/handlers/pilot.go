package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/events"
	"github.com/reh3376/career-site/services/api/internal/pilot"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// The employer-side pilot (FSD §5.16; docs/framework-research/15).
//
// Every member RPC here passes one gate: the caller holds the pilot
// flag, or is an admin, who sees the pilot as a participant would. A
// member without the flag is told allowed=false by GetPilot and refused
// by everything else, so the page can render a plain 404 and nothing
// about the pilot leaks to a member who is not in it.
//
// The instrument, the record and the workflow are data loaded once at
// start-up and validated then, so a broken edit stops the server rather
// than a participant.
type Pilot struct {
	careerv1connect.UnimplementedPilotServiceHandler

	log    *slog.Logger
	users  *users.Repo
	events *events.Writer // product event stream; nil is silent

	instrument    *pilot.Instrument
	instrumentRaw []byte
	record        *pilot.Record
	workflow      *pilot.Workflow
}

// NewPilot loads the embedded data. An invalid file is a start-up
// failure, by design.
func NewPilot(log *slog.Logger, repo *users.Repo) (*Pilot, error) {
	in, raw, err := pilot.LoadInstrument(pilot.CurrentInstrumentVersion)
	if err != nil {
		return nil, err
	}
	rec, err := pilot.LoadRecord(pilot.CurrentRecordVersion)
	if err != nil {
		return nil, err
	}
	wf, err := pilot.LoadWorkflow(pilot.CurrentWorkflowVersion)
	if err != nil {
		return nil, err
	}
	return &Pilot{log: log, users: repo, instrument: in, instrumentRaw: raw, record: rec, workflow: wf}, nil
}

// SetEvents wires the product event stream. Optional.
func (h *Pilot) SetEvents(w *events.Writer) { h.events = w }

// EnsureInstrument copies the current definition into pilot_instruments
// if that version is not there yet (FR-PLT-04). Called at boot; the row
// is what pilot_sessions.instrument_version references.
func (h *Pilot) EnsureInstrument(ctx context.Context) error {
	return h.users.EnsurePilotInstrument(ctx, h.instrument.Version, h.instrumentRaw)
}

// gate returns the caller when they may enter the pilot.
func (h *Pilot) gate(ctx context.Context) (*users.User, error) {
	u, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	if !(u.PilotApproved || u.Role == users.RoleAdmin) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("the pilot is not open to this account"))
	}
	return u, nil
}

// session returns the caller's session or a failed-precondition error
// telling them to start.
func (h *Pilot) session(ctx context.Context, u *users.User) (*users.PilotSession, error) {
	s, err := h.users.GetPilotSession(ctx, u.ID, h.instrument.Version)
	if errors.Is(err, users.ErrNotFound) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("start the pilot first"))
	}
	if err != nil {
		h.log.Error("pilot session lookup failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("pilot unavailable"))
	}
	return s, nil
}

// sessionProto fills the wire form, including the computed progress.
func (h *Pilot) sessionProto(ctx context.Context, s *users.PilotSession) *v1.PilotSession {
	out := &v1.PilotSession{
		Id:                s.ID,
		Status:            s.Status,
		RoleBranch:        s.RoleBranch,
		InstrumentVersion: s.InstrumentVersion,
		ConsentAt:         timestamppb.New(s.ConsentAt),
		RecordingOk:       s.RecordingOK,
	}
	if s.SurveyFinishedAt != nil {
		out.SurveyFinishedAt = timestamppb.New(*s.SurveyFinishedAt)
	}
	if s.RecordFinishedAt != nil {
		out.RecordFinishedAt = timestamppb.New(*s.RecordFinishedAt)
	}
	if s.WorkflowFinishedAt != nil {
		out.WorkflowFinishedAt = timestamppb.New(*s.WorkflowFinishedAt)
	}
	if s.FinishedAt != nil {
		out.FinishedAt = timestamppb.New(*s.FinishedAt)
	}
	done, err := h.users.SubmittedPilotSections(ctx, s.ID)
	if err != nil {
		h.log.Warn("pilot submitted sections lookup failed", slog.String("session", s.ID), slog.String("error", err.Error()))
		done = map[string]bool{}
	}
	for _, id := range h.instrument.SectionOrder(s.RoleBranch) {
		if done[id] {
			out.SectionsDone = append(out.SectionsDone, id)
		}
	}
	out.NextSection = h.instrument.NextSection(s.RoleBranch, done)
	return out
}

// GetPilot says whether the caller may enter and where they are.
func (h *Pilot) GetPilot(ctx context.Context, _ *connect.Request[v1.GetPilotRequest]) (*connect.Response[v1.GetPilotResponse], error) {
	u, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	out := &v1.GetPilotResponse{
		Allowed:           u.PilotApproved || u.Role == users.RoleAdmin,
		Admin:             u.Role == users.RoleAdmin,
		InstrumentVersion: h.instrument.Version,
	}
	if !out.Allowed {
		return connect.NewResponse(out), nil
	}
	s, err := h.users.GetPilotSession(ctx, u.ID, h.instrument.Version)
	if err == nil {
		out.Session = h.sessionProto(ctx, s)
	} else if !errors.Is(err, users.ErrNotFound) {
		h.log.Warn("pilot session lookup failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
	}
	return connect.NewResponse(out), nil
}

// StartPilot records consent and opens or resumes the session.
func (h *Pilot) StartPilot(ctx context.Context, req *connect.Request[v1.StartPilotRequest]) (*connect.Response[v1.StartPilotResponse], error) {
	u, err := h.gate(ctx)
	if err != nil {
		return nil, err
	}
	if !req.Msg.Consent {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("consent is required to start"))
	}
	s, err := h.users.StartPilotSession(ctx, u.ID, h.instrument.Version, req.Msg.RecordingOk)
	if err != nil {
		h.log.Error("start pilot session failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not start the pilot"))
	}
	h.events.Emit(ctx, requestEvent(req, "pilot.consent", u.ID, map[string]any{"user_id": u.ID}))
	return connect.NewResponse(&v1.StartPilotResponse{Session: h.sessionProto(ctx, s)}), nil
}

// answersByItem loads a session's answers as item id to value JSON.
func (h *Pilot) answersByItem(ctx context.Context, sessionID string) (map[string]string, []users.PilotAnswer, error) {
	list, err := h.users.ListPilotAnswers(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	m := make(map[string]string, len(list))
	for _, a := range list {
		m[a.ItemID] = a.ValueJSON
	}
	return m, list, nil
}

func itemProto(it *pilot.Item, options []string) *v1.PilotItem {
	out := &v1.PilotItem{
		Id:        it.ID,
		Type:      it.Type,
		Prompt:    it.Prompt,
		Help:      it.Help,
		Options:   options,
		MaxSelect: int32(it.MaxSelect),
		Required:  it.Required,
		PilotOnly: it.PilotOnly,
		// Named so the page can draw C2's and C3's rows from C1's live
		// selection; the options above are what the saved answer gives.
		SourceItem: it.SourceItem,
	}
	if it.Scale != nil {
		out.Scale = &v1.PilotScale{Min: int32(it.Scale.Min), Max: int32(it.Scale.Max), MinLabel: it.Scale.MinLabel, MaxLabel: it.Scale.MaxLabel}
	}
	for _, d := range it.Dimensions {
		out.Dimensions = append(out.Dimensions, &v1.PilotDimension{Id: d.ID, Label: d.Label, Options: d.Options})
	}
	return out
}

// sectionAccess checks that a section may be shown to the session.
func (h *Pilot) sectionAccess(s *users.PilotSession, sectionID string) (*pilot.Section, error) {
	sec := h.instrument.Section(sectionID)
	if sec == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such section"))
	}
	first := h.instrument.Sections[0].ID
	if sectionID != first && s.RoleBranch == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("answer section %s first", first))
	}
	if s.RoleBranch == pilot.BranchEnd && sectionID != first {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the survey ended after the first section"))
	}
	return sec, nil
}

// GetPilotSection returns one section for the caller's branch.
func (h *Pilot) GetPilotSection(ctx context.Context, req *connect.Request[v1.GetPilotSectionRequest]) (*connect.Response[v1.GetPilotSectionResponse], error) {
	u, err := h.gate(ctx)
	if err != nil {
		return nil, err
	}
	s, err := h.session(ctx, u)
	if err != nil {
		return nil, err
	}
	sec, err := h.sectionAccess(s, req.Msg.SectionId)
	if err != nil {
		return nil, err
	}
	byItem, list, err := h.answersByItem(ctx, s.ID)
	if err != nil {
		h.log.Error("pilot answers lookup failed", slog.String("session", s.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("pilot unavailable"))
	}
	out := &v1.GetPilotSectionResponse{Id: sec.ID, Title: sec.Title, Intro: sec.Intro}
	for _, it := range sec.VisibleItems(s.RoleBranch) {
		item := it
		out.Items = append(out.Items, itemProto(&item, item.ResolveOptions(byItem)))
	}
	for _, a := range list {
		if a.Section == sec.ID {
			out.Answers = append(out.Answers, &v1.PilotAnswer{ItemId: a.ItemID, ValueJson: a.ValueJSON, Flagged: a.Flagged, Note: a.Note})
		}
	}
	order := h.instrument.SectionOrder(s.RoleBranch)
	out.Total = int32(len(order))
	for i, id := range order {
		if id == sec.ID {
			out.Index = int32(i + 1)
		}
	}
	if err := h.users.RecordPilotSectionEvent(ctx, s.ID, sec.ID, "opened"); err != nil {
		h.log.Warn("pilot section event failed", slog.String("session", s.ID), slog.String("error", err.Error()))
	}
	out.Session = h.sessionProto(ctx, s)
	return connect.NewResponse(out), nil
}

// SubmitPilotSection validates and saves a section's answers.
func (h *Pilot) SubmitPilotSection(ctx context.Context, req *connect.Request[v1.SubmitPilotSectionRequest]) (*connect.Response[v1.SubmitPilotSectionResponse], error) {
	u, err := h.gate(ctx)
	if err != nil {
		return nil, err
	}
	s, err := h.session(ctx, u)
	if err != nil {
		return nil, err
	}
	if s.FinishedAt != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("this pilot session is closed"))
	}
	sec, err := h.sectionAccess(s, req.Msg.SectionId)
	if err != nil {
		return nil, err
	}
	byItem, _, err := h.answersByItem(ctx, s.ID)
	if err != nil {
		h.log.Error("pilot answers lookup failed", slog.String("session", s.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("pilot unavailable"))
	}
	// Submitted values join the saved ones so a derived item (C2, C3)
	// can resolve against C1 from the same submission.
	submitted := map[string]*v1.PilotAnswer{}
	for _, a := range req.Msg.Answers {
		if a == nil {
			continue
		}
		submitted[a.ItemId] = a
		byItem[a.ItemId] = a.ValueJson
	}
	visible := sec.VisibleItems(s.RoleBranch)
	known := map[string]bool{}
	var toSave []users.PilotAnswer
	for i := range visible {
		it := &visible[i]
		known[it.ID] = true
		a, ok := submitted[it.ID]
		if !ok || it.IsEmptyAnswer(a.ValueJson) {
			if it.Required {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s is required", it.ID))
			}
			// An optional item left blank but flagged, or with a note,
			// still said something: "this was unclear", or whatever the
			// note says. Either needs a row to live on, so the row is
			// saved with an empty value rather than dropped (FR-PLT-05).
			if ok && (a.Flagged || strings.TrimSpace(a.Note) != "") {
				empty := `""`
				if it.Type != pilot.TypeText {
					empty = "null"
				}
				toSave = append(toSave, users.PilotAnswer{Section: sec.ID, ItemID: it.ID, ValueJSON: empty, Flagged: a.Flagged, Note: strings.TrimSpace(a.Note)})
			}
			continue
		}
		if err := it.ValidateAnswer(a.ValueJson, it.ResolveOptions(byItem)); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		toSave = append(toSave, users.PilotAnswer{Section: sec.ID, ItemID: it.ID, ValueJSON: a.ValueJson, Flagged: a.Flagged, Note: strings.TrimSpace(a.Note)})
	}
	for id := range submitted {
		if !known[id] {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s is not in section %s for this participant", id, sec.ID))
		}
	}
	if err := h.users.UpsertPilotAnswers(ctx, s.ID, sec.ID, toSave); err != nil {
		h.log.Error("pilot answers save failed", slog.String("session", s.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not save the section"))
	}
	// Routing happens when the routing item is submitted.
	if a, ok := submitted[h.instrument.Branches.Item]; ok {
		branch := h.instrument.BranchFor(a.ValueJson)
		if branch != "" && branch != s.RoleBranch {
			if err := h.users.SetPilotRoleBranch(ctx, s.ID, branch); err != nil {
				h.log.Warn("pilot branch save failed", slog.String("session", s.ID), slog.String("error", err.Error()))
			} else {
				s.RoleBranch = branch
			}
		}
	}
	if err := h.users.RecordPilotSectionEvent(ctx, s.ID, sec.ID, "submitted"); err != nil {
		h.log.Warn("pilot section event failed", slog.String("session", s.ID), slog.String("error", err.Error()))
	}
	h.events.Emit(ctx, requestEvent(req, "pilot.section_submitted", u.ID, map[string]any{"section": sec.ID}))

	done, _ := h.users.SubmittedPilotSections(ctx, s.ID)
	if h.instrument.NextSection(s.RoleBranch, done) == "" {
		// The survey is complete for this branch. The two pilot-only
		// closers ride on the session for the admin list.
		var minutes *int32
		if raw, ok := byItem["P1"]; ok {
			var f float64
			if json.Unmarshal([]byte(raw), &f) == nil && f >= 0 && f < 100000 {
				m := int32(f)
				minutes = &m
			}
		}
		skip := ""
		if raw, ok := byItem["P2"]; ok {
			var t string
			if json.Unmarshal([]byte(raw), &t) == nil {
				skip = strings.TrimSpace(t)
			}
		}
		if err := h.users.MarkPilotSurveyFinished(ctx, s.ID, minutes, skip); err != nil {
			h.log.Warn("pilot survey finish failed", slog.String("session", s.ID), slog.String("error", err.Error()))
		}
	}
	fresh, err := h.users.GetPilotSessionByID(ctx, s.ID)
	if err != nil {
		fresh = s
	}
	return connect.NewResponse(&v1.SubmitPilotSectionResponse{
		Session: h.sessionProto(ctx, fresh),
		Ended:   fresh.RoleBranch == pilot.BranchEnd,
	}), nil
}

func commentsProto(list []users.PilotComment, prefix string) []*v1.PilotComment {
	var out []*v1.PilotComment
	for _, c := range list {
		if !strings.HasPrefix(c.TargetKind, prefix) {
			continue
		}
		out = append(out, &v1.PilotComment{TargetKind: c.TargetKind, TargetId: c.TargetID, Body: c.Body, Answer: c.Answer, UpdatedAt: timestamppb.New(c.UpdatedAt)})
	}
	return out
}

// GetPilotRecord returns the mocked match record and the caller's comments.
func (h *Pilot) GetPilotRecord(ctx context.Context, _ *connect.Request[v1.GetPilotRecordRequest]) (*connect.Response[v1.GetPilotRecordResponse], error) {
	u, err := h.gate(ctx)
	if err != nil {
		return nil, err
	}
	s, err := h.session(ctx, u)
	if err != nil {
		return nil, err
	}
	comments, err := h.users.ListPilotComments(ctx, s.ID)
	if err != nil {
		h.log.Warn("pilot comments lookup failed", slog.String("session", s.ID), slog.String("error", err.Error()))
	}
	return connect.NewResponse(&v1.GetPilotRecordResponse{
		RecordJson:    string(h.record.Raw),
		RecordVersion: h.record.Version,
		Comments:      commentsProto(comments, "record_"),
	}), nil
}

// GetPilotWorkflow returns the company workflow and the caller's comments.
func (h *Pilot) GetPilotWorkflow(ctx context.Context, _ *connect.Request[v1.GetPilotWorkflowRequest]) (*connect.Response[v1.GetPilotWorkflowResponse], error) {
	u, err := h.gate(ctx)
	if err != nil {
		return nil, err
	}
	s, err := h.session(ctx, u)
	if err != nil {
		return nil, err
	}
	comments, err := h.users.ListPilotComments(ctx, s.ID)
	if err != nil {
		h.log.Warn("pilot comments lookup failed", slog.String("session", s.ID), slog.String("error", err.Error()))
	}
	out := &v1.GetPilotWorkflowResponse{
		Intro:           h.workflow.Intro,
		Prompts:         h.workflow.Prompts,
		WorkflowVersion: h.workflow.Version,
		WorkflowJson:    string(h.workflow.Raw),
		Comments:        commentsProto(comments, "workflow_"),
	}
	for _, p := range h.workflow.Phases {
		out.Phases = append(out.Phases, &v1.PilotWorkflowPhase{Id: p.ID, Title: p.Title})
	}
	for _, n := range h.workflow.Nodes {
		out.Nodes = append(out.Nodes, &v1.PilotWorkflowNode{Id: n.ID, Phase: n.Phase, Name: n.Name, Owner: n.Owner, Summary: n.Summary, Decision: n.Decision, Open: n.Open})
	}
	return connect.NewResponse(out), nil
}

// UpsertPilotComment saves, replaces or removes one comment.
func (h *Pilot) UpsertPilotComment(ctx context.Context, req *connect.Request[v1.UpsertPilotCommentRequest]) (*connect.Response[v1.UpsertPilotCommentResponse], error) {
	u, err := h.gate(ctx)
	if err != nil {
		return nil, err
	}
	s, err := h.session(ctx, u)
	if err != nil {
		return nil, err
	}
	kind, target := req.Msg.TargetKind, strings.TrimSpace(req.Msg.TargetId)
	answer := req.Msg.Answer
	switch kind {
	case "record_row":
		if !h.record.HasRow(target) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no such record row"))
		}
	case "workflow_node":
		if !h.workflow.HasNode(target) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no such workflow node"))
		}
	case "workflow_outcomes":
		// The open question at a decision point: what can happen here.
		if !h.workflow.IsDecision(target) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("outcomes are asked at decision steps only"))
		}
	case "workflow_phase":
		// A stage question: <phase>-<question>, e.g. A-merge, B-owner.
		phase, q, ok := strings.Cut(target, "-")
		if !ok || !h.workflow.HasPhase(phase) || (q != "merge" && q != "owner") {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a stage question is <stage>-merge or <stage>-owner"))
		}
	case "workflow_missing":
		// A missing step is numbered within its stage: A-1 to A-20.
		phase, num, ok := strings.Cut(target, "-")
		n, err := strconv.Atoi(num)
		if !ok || !h.workflow.HasPhase(phase) || err != nil || n < 1 || n > 20 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a missing step is <stage>-1 to <stage>-20"))
		}
	case "record_general":
		if target != "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a general comment has no target id"))
		}
	case "workflow_general":
		switch target {
		case "", "overview", "candidate", "holds":
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a workflow general comment is overview, candidate or holds"))
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unknown comment target kind"))
	}
	if kind != "workflow_node" {
		answer = ""
	}
	switch answer {
	case "", "matches", "differs", "absent":
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("an answer is matches, differs or absent"))
	}
	body := strings.TrimSpace(req.Msg.Body)
	if len(body) > 4000 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a comment is at most 4000 characters"))
	}
	c, err := h.users.UpsertPilotComment(ctx, s.ID, kind, target, body, answer)
	if err != nil {
		h.log.Error("pilot comment save failed", slog.String("session", s.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not save the comment"))
	}
	h.events.Emit(ctx, requestEvent(req, "pilot.comment", u.ID, map[string]any{"target_kind": kind}))
	out := &v1.UpsertPilotCommentResponse{}
	if c != nil {
		out.Comment = &v1.PilotComment{TargetKind: c.TargetKind, TargetId: c.TargetID, Body: c.Body, Answer: c.Answer, UpdatedAt: timestamppb.New(c.UpdatedAt)}
	}
	return connect.NewResponse(out), nil
}

// MarkPilotPart stamps the record or the workflow part done.
func (h *Pilot) MarkPilotPart(ctx context.Context, req *connect.Request[v1.MarkPilotPartRequest]) (*connect.Response[v1.MarkPilotPartResponse], error) {
	u, err := h.gate(ctx)
	if err != nil {
		return nil, err
	}
	s, err := h.session(ctx, u)
	if err != nil {
		return nil, err
	}
	part := req.Msg.Part
	if part != "record" && part != "workflow" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("part is record or workflow"))
	}
	if err := h.users.MarkPilotPart(ctx, s.ID, part); err != nil {
		h.log.Error("pilot part mark failed", slog.String("session", s.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not record that"))
	}
	h.events.Emit(ctx, requestEvent(req, "pilot.part_done", u.ID, map[string]any{"part": part}))
	fresh, err := h.users.GetPilotSessionByID(ctx, s.ID)
	if err != nil {
		fresh = s
	}
	return connect.NewResponse(&v1.MarkPilotPartResponse{Session: h.sessionProto(ctx, fresh)}), nil
}

// FinishPilot closes the session once the survey is submitted.
func (h *Pilot) FinishPilot(ctx context.Context, req *connect.Request[v1.FinishPilotRequest]) (*connect.Response[v1.FinishPilotResponse], error) {
	u, err := h.gate(ctx)
	if err != nil {
		return nil, err
	}
	s, err := h.session(ctx, u)
	if err != nil {
		return nil, err
	}
	if s.SurveyFinishedAt == nil && s.RoleBranch != pilot.BranchEnd {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("finish the survey first"))
	}
	// The two readings are the pilot's second purpose, and the first
	// live run (2026-11-09) showed that a Finish offered before them is
	// taken: the owner finished without ever seeing the workflow. All
	// three parts, server-side (ADR 0031), before a session closes.
	if s.RecordFinishedAt == nil || s.WorkflowFinishedAt == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("read the match record and the workflow first"))
	}
	if err := h.users.FinishPilotSession(ctx, s.ID); err != nil {
		h.log.Error("pilot finish failed", slog.String("session", s.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not close the session"))
	}
	h.events.Emit(ctx, requestEvent(req, "pilot.finished", u.ID, map[string]any{"user_id": u.ID}))
	fresh, err := h.users.GetPilotSessionByID(ctx, s.ID)
	if err != nil {
		fresh = s
	}
	return connect.NewResponse(&v1.FinishPilotResponse{Session: h.sessionProto(ctx, fresh)}), nil
}
