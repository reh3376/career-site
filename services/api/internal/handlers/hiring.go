package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/corpusscope"
	"github.com/reh3376/career-site/services/api/internal/events"
	"github.com/reh3376/career-site/services/api/internal/hiring"
	"github.com/reh3376/career-site/services/api/internal/jd"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Hiring serves HiringService: the requirement as an editable,
// versioned row (FSD §5.15 FR-HF-01, 02, 03, 23; Phase 0 step 5). A
// person with the admin role reads what the extraction drafted,
// corrects it with a reason, and releases it; the release starts a run
// that judges the released list. In Phase 0 the admin is the owner
// standing in for the hiring manager.
type Hiring struct {
	log      *slog.Logger
	users    *users.Repo
	repo     *hiring.Repo
	settings *hiring.SettingsStore
	// scorer runs the re-judge a release starts; nil when the reviewer
	// is not wired (development without a model), in which case a
	// release still releases and says no run began.
	scorer *jd.Scorer
	events *events.Writer
	// bands names the fit a score falls in, for the fit after review on
	// a record that carries an override; nil falls back to the scorer's.
	bands *jd.BandsStore
	// resumes writes a résumé after review; nil when the reviewer is
	// not wired, in which case none is offered.
	resumes reviewResumeRunner
}

// SetBands wires the fit bands.
func (h *Hiring) SetBands(b *jd.BandsStore) { h.bands = b }

// reviewResumeRunner is what writes a résumé after review: the scorer,
// or a stand-in under test.
type reviewResumeRunner interface {
	CanWriteResume() bool
	WriteReviewResume(ctx context.Context, submissionID int64, runID, jdText string, hints prompts.Hints)
}

// SetReviewResumes wires the writer of résumés after review.
func (h *Hiring) SetReviewResumes(r reviewResumeRunner) { h.resumes = r }

// NewHiring wires the handler. scorer may be nil.
func NewHiring(log *slog.Logger, repo *users.Repo, hiringRepo *hiring.Repo, settings *hiring.SettingsStore, scorer *jd.Scorer) *Hiring {
	return &Hiring{log: log, users: repo, repo: hiringRepo, settings: settings, scorer: scorer}
}

// SetEvents wires the product event stream.
func (h *Hiring) SetEvents(w *events.Writer) { h.events = w }

// hiringError maps the hiring package's errors onto Connect codes, with
// the sentence the package wrote, which is written for the person
// editing.
func hiringError(err error) error {
	switch {
	case errors.Is(err, hiring.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("requirement not found"))
	case errors.Is(err, hiring.ErrInvalidRequirement):
		msg := strings.TrimPrefix(err.Error(), hiring.ErrInvalidRequirement.Error()+": ")
		return connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
	case errors.Is(err, hiring.ErrNothingChanged):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, hiring.ErrNotEditable):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewError(connect.CodeInternal, errors.New("the requirement could not be saved"))
}

func requirementStatusProto(status string) v1.RequirementStatus {
	switch status {
	case hiring.StatusDraft:
		return v1.RequirementStatus_REQUIREMENT_STATUS_DRAFT
	case hiring.StatusActive:
		return v1.RequirementStatus_REQUIREMENT_STATUS_ACTIVE
	case hiring.StatusRetired:
		return v1.RequirementStatus_REQUIREMENT_STATUS_RETIRED
	}
	return v1.RequirementStatus_REQUIREMENT_STATUS_UNSPECIFIED
}

func thresholdsProto(t hiring.Thresholds) *v1.Thresholds {
	out := &v1.Thresholds{
		MinYears: t.MinYears, RecencyYears: t.RecencyYears,
		CredentialLevel: t.CredentialLevel, CredentialField: t.CredentialField, RelationshipKind: t.RelationshipKind,
	}
	if t.MinScope != nil {
		out.MinScope = &v1.ScopeThreshold{Unit: t.MinScope.Unit, Value: t.MinScope.Value}
	}
	return out
}

func thresholdsFromProto(t *v1.Thresholds) hiring.Thresholds {
	out := hiring.Thresholds{
		MinYears: t.MinYears, RecencyYears: t.RecencyYears,
		CredentialLevel:  strings.TrimSpace(t.GetCredentialLevel()),
		CredentialField:  strings.TrimSpace(t.GetCredentialField()),
		RelationshipKind: strings.TrimSpace(t.GetRelationshipKind()),
	}
	// A zero is "no threshold", not "zero years": a threshold of nothing
	// is a comparison nothing can fail, and storing it would only make
	// the row say it asks for something it does not.
	if out.MinYears != nil && *out.MinYears == 0 {
		out.MinYears = nil
	}
	if out.RecencyYears != nil && *out.RecencyYears == 0 {
		out.RecencyYears = nil
	}
	if s := t.GetMinScope(); s != nil {
		out.MinScope = &hiring.Scope{Unit: strings.TrimSpace(s.GetUnit()), Value: s.GetValue()}
	}
	return out
}

func requirementProto(q hiring.Requirement) *v1.Requirement {
	out := &v1.Requirement{
		Id: q.ID, RoleId: q.RoleID, LocalId: q.LocalID, Version: int32(q.Version), Supersedes: q.Supersedes,
		SchemaVersion: q.SchemaVersion, VocabVersion: q.VocabVersion,
		Text: q.Text, SourceQuote: q.SourceQuote, Type: q.Type, Category: q.Category,
		RequirementClass: q.Class, Weight: int32(q.Weight),
		EvidenceStandard: q.EvidenceStandard, EvidenceStandardTemplate: q.EvidenceStandardTemplate,
		Alternatives: q.Alternatives, EquivalenceNotes: q.EquivalenceNotes, Equivalency: q.Equivalency,
		Thresholds: thresholdsProto(q.Thresholds), NamedParties: q.NamedParties,
		Included: q.Included, InclusionReason: q.InclusionReason(),
		EditedByDisplay: q.EditedByDisplay, EditReason: q.EditReason,
		Status:           requirementStatusProto(q.Status),
		ReleasedByPerson: q.ReleasedBy > 0,
		CreatedAt:        timestamppb.New(q.CreatedAt),
	}
	if q.EditedAt != nil {
		out.EditedAt = timestamppb.New(*q.EditedAt)
	}
	if q.ReleasedAt != nil {
		out.ReleasedAt = timestamppb.New(*q.ReleasedAt)
	}
	if len(q.Extraction) > 0 {
		var prov struct {
			TypeSource string `json:"type_source"`
		}
		if json.Unmarshal(q.Extraction, &prov) == nil {
			out.TypeSource = prov.TypeSource
		}
	}
	return out
}

func vocabProto(reg *hiring.Registry, vocab string) []*v1.VocabCode {
	codes := reg.Codes(vocab)
	out := make([]*v1.VocabCode, 0, len(codes))
	for _, c := range codes {
		out = append(out, &v1.VocabCode{Code: c.Code, Label: c.Label, Description: c.Description})
	}
	return out
}

// ListRequirements lists a role's requirements for the editor.
func (h *Hiring) ListRequirements(
	ctx context.Context,
	req *connect.Request[v1.ListRequirementsRequest],
) (*connect.Response[v1.ListRequirementsResponse], error) {
	if _, err := RequireCaller(ctx); err != nil {
		return nil, err
	}
	roleID := strings.TrimSpace(req.Msg.GetRoleId())
	submission := strings.TrimSpace(req.Msg.GetSubmissionId())
	if (roleID == "") == (submission == "") {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("give the role_id or the submission_id, one of them"))
	}
	out := &v1.ListRequirementsResponse{}
	// The vocabularies come with the list, so the editor labels its
	// choices from the registry rather than from copy (14 §3.1).
	if reg, err := h.repo.LoadVocab(ctx); err != nil {
		h.log.Warn("hiring: vocabulary unavailable for the editor", slog.String("error", err.Error()))
	} else {
		out.Types = vocabProto(reg, hiring.VocabRequirementType)
		out.Classes = vocabProto(reg, hiring.VocabRequirementClass)
		out.ScopeUnits = vocabProto(reg, hiring.VocabScopeUnit)
		out.CredentialLevels = vocabProto(reg, hiring.VocabCredentialLevel)
		out.RelationshipKinds = vocabProto(reg, hiring.VocabRelationshipKind)
	}
	if roleID == "" {
		id, err := strconv.ParseInt(submission, 10, 64)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid submission_id"))
		}
		found, ok, err := h.repo.RoleForSubmission(ctx, id)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the role"))
		}
		if !ok {
			// A posting last scored before the hiring tables began has no
			// rows; that is an answer, not an error.
			return connect.NewResponse(out), nil
		}
		roleID = found
	} else if !looksLikeUUID(roleID) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid role_id"))
	}
	rows, err := h.repo.ListRequirements(ctx, roleID, req.Msg.GetIncludeRetired())
	if err != nil {
		h.log.Error("hiring: list requirements", slog.String("role", roleID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the requirements"))
	}
	out.RoleId = roleID
	for _, q := range rows {
		out.Requirements = append(out.Requirements, requirementProto(q))
		if q.Status == hiring.StatusDraft {
			out.PendingDrafts++
		}
		if q.Status != hiring.StatusRetired && (q.EditedBy > 0 || q.Status == hiring.StatusDraft) {
			out.HumanEdited = true
		}
	}
	return connect.NewResponse(out), nil
}

// looksLikeUUID keeps a malformed id from reaching a ::uuid cast, where
// it would come back as a database error rather than a bad request.
func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
	}
	return true
}

func patchFromProto(p *v1.RequirementPatch) (hiring.RequirementPatch, []string) {
	var out hiring.RequirementPatch
	var fields []string
	if p.Text != nil {
		out.Text = p.Text
		fields = append(fields, "text")
	}
	if p.Type != nil {
		out.Type = p.Type
		fields = append(fields, "type")
	}
	if p.RequirementClass != nil {
		out.Class = p.RequirementClass
		fields = append(fields, "class")
	}
	if p.Weight != nil {
		w := int(*p.Weight)
		out.Weight = &w
		fields = append(fields, "weight")
	}
	if p.Thresholds != nil {
		t := thresholdsFromProto(p.Thresholds)
		out.Thresholds = &t
		fields = append(fields, "thresholds")
	}
	if p.EvidenceStandard != nil {
		out.EvidenceStandard = p.EvidenceStandard
		fields = append(fields, "evidence_standard")
	}
	if p.EquivalenceNotes != nil {
		out.EquivalenceNotes = p.EquivalenceNotes
		fields = append(fields, "equivalence_notes")
	}
	if p.Equivalency != nil {
		out.Equivalency = p.Equivalency
		fields = append(fields, "equivalency")
	}
	if p.Alternatives != nil {
		list := append([]string{}, p.Alternatives.GetValues()...)
		out.Alternatives = &list
		fields = append(fields, "alternatives")
	}
	if p.NamedParties != nil {
		list := append([]string{}, p.NamedParties.GetValues()...)
		out.NamedParties = &list
		fields = append(fields, "named_parties")
	}
	return out, fields
}

// UpdateRequirement saves an edit as a new draft version (FR-HF-03).
func (h *Hiring) UpdateRequirement(
	ctx context.Context,
	req *connect.Request[v1.UpdateRequirementRequest],
) (*connect.Response[v1.UpdateRequirementResponse], error) {
	admin, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	id := strings.TrimSpace(req.Msg.GetRequirementId())
	if !looksLikeUUID(id) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid requirement_id"))
	}
	if req.Msg.GetPatch() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a patch is required"))
	}
	patch, fields := patchFromProto(req.Msg.GetPatch())
	// The catalog renders the equivalence notes again when the wording
	// changes. Without it the notes are carried over as they are, which
	// is the safe direction.
	catalog, cErr := h.repo.LoadCatalog(ctx)
	if cErr != nil {
		h.log.Warn("hiring: catalog unavailable for an edit; equivalence notes carried over", slog.String("error", cErr.Error()))
		catalog = nil
	}
	row, err := h.repo.UpdateRequirement(ctx, id, patch, admin.ID, req.Msg.GetEditReason(), catalog)
	if err != nil {
		if !errors.Is(err, hiring.ErrInvalidRequirement) && !errors.Is(err, hiring.ErrNothingChanged) &&
			!errors.Is(err, hiring.ErrNotEditable) && !errors.Is(err, hiring.ErrNotFound) {
			h.log.Error("hiring: update requirement", slog.String("id", id), slog.String("error", err.Error()))
		}
		return nil, hiringError(err)
	}
	h.log.Info("hiring: requirement edited", slog.String("role", row.RoleID), slog.String("local_id", row.LocalID),
		slog.Int("version", row.Version), slog.Int64("admin_id", admin.ID))
	h.events.Emit(ctx, requestEvent(req, "admin.requirement_edited", admin.ID, map[string]any{
		"requirement_id": row.ID, "role_id": row.RoleID, "local_id": row.LocalID,
		"version": row.Version, "fields": strings.Join(fields, ","),
	}))
	return connect.NewResponse(&v1.UpdateRequirementResponse{Requirement: requirementProto(row)}), nil
}

// ReleaseRequirements releases a role's drafts and starts the run that
// judges the released list (FR-HF-03).
func (h *Hiring) ReleaseRequirements(
	ctx context.Context,
	req *connect.Request[v1.ReleaseRequirementsRequest],
) (*connect.Response[v1.ReleaseRequirementsResponse], error) {
	admin, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	roleID := strings.TrimSpace(req.Msg.GetRoleId())
	if !looksLikeUUID(roleID) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid role_id"))
	}
	submissionID, err := h.repo.SubmissionForRole(ctx, roleID)
	if errors.Is(err, hiring.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("role not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the role"))
	}
	var s *users.JdSubmission
	if submissionID > 0 {
		s, err = h.users.GetJdSubmission(ctx, submissionID)
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("the role's posting was not found"))
		}
		// Refused before anything is released: a run in flight read the
		// list as it was, and releasing under it would leave its record
		// naming versions that are no longer the released ones.
		if s.Status == "scoring" || s.Status == "generating" {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the posting is being scored; release when that run has finished"))
		}
	}
	released, err := h.repo.ReleaseRequirements(ctx, roleID, admin.ID)
	if err != nil {
		h.log.Error("hiring: release requirements", slog.String("role", roleID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not release the requirements"))
	}
	out := &v1.ReleaseRequirementsResponse{}
	for _, q := range released {
		out.Released = append(out.Released, requirementProto(q))
	}
	if submissionID > 0 {
		out.SubmissionId = strconv.FormatInt(submissionID, 10)
	}
	if h.scorer != nil && s != nil {
		active, aErr := h.repo.ActiveRequirements(ctx, roleID)
		if aErr != nil || len(active) == 0 {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the role has no released requirements to judge"))
		}
		if err := h.users.UpdateJdScoring(ctx, submissionID, "scoring", nil, ""); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("could not mark the posting as scoring"))
		}
		runs, rErr := h.users.ListJdRuns(ctx, submissionID)
		if rErr != nil {
			h.log.Warn("hiring: could not read the posting's runs; the re-judge reads the public corpus",
				slog.Int64("id", submissionID), slog.String("error", rErr.Error()))
		}
		scope := rejudgeScope(runs)
		hints := prompts.Hints{Role: s.RoleHint, Employer: s.EmployerHint}
		go func(id int64, text string, adminID int64) {
			h.scorer.RejudgeAndPersist(corpusscope.With(context.Background(), scope), id, text, hints, adminID)
		}(submissionID, s.JdText, admin.ID)
		out.RunStarted = true
	}
	h.log.Info("hiring: requirements released", slog.String("role", roleID), slog.Int("released", len(released)),
		slog.Bool("run_started", out.RunStarted), slog.Int64("admin_id", admin.ID))
	h.events.Emit(ctx, requestEvent(req, "admin.requirements_released", admin.ID, map[string]any{
		"role_id": roleID, "submission_id": submissionID, "released": len(released), "run_started": out.RunStarted,
	}))
	return connect.NewResponse(out), nil
}

// rejudgeScope is what a re-judge may read: what the posting's last
// run that says could read, so the edit is the only thing that changed
// between the two. A re-judge that also widened or narrowed retrieval
// would move verdicts for a reason nobody asked for; the same mistake
// once made a re-score look non-deterministic (0.929 to 0.571 on
// identical input, RescoreJd). Runs are newest first. With no run that
// says, the narrower scope is the safe one: a member's posting must
// never reach the private corpus by default.
func rejudgeScope(runs []users.JdRun) corpusscope.Scope {
	for _, r := range runs {
		switch r.RetrievalScope {
		case "all":
			return corpusscope.All
		case "public":
			return corpusscope.Public
		}
	}
	return corpusscope.Public
}

// GetHiringSettings reads the framework's settings.
func (h *Hiring) GetHiringSettings(
	ctx context.Context,
	req *connect.Request[v1.GetHiringSettingsRequest],
) (*connect.Response[v1.GetHiringSettingsResponse], error) {
	if _, err := RequireCaller(ctx); err != nil {
		return nil, err
	}
	s := h.settings.Get(ctx)
	return connect.NewResponse(&v1.GetHiringSettingsResponse{
		Settings: &v1.HiringSettings{PreferredFraction: s.PreferredFraction},
	}), nil
}

// SetHiringSettings stores the framework's settings (FR-HF-23: the
// preferred fraction is adjustable in the admin console).
func (h *Hiring) SetHiringSettings(
	ctx context.Context,
	req *connect.Request[v1.SetHiringSettingsRequest],
) (*connect.Response[v1.SetHiringSettingsResponse], error) {
	admin, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	in := req.Msg.GetSettings()
	if in == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("settings are required"))
	}
	next := hiring.Settings{PreferredFraction: in.GetPreferredFraction()}
	if err := h.settings.Set(ctx, next, admin.ID); err != nil {
		if errors.Is(err, hiring.ErrInvalidSettings) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		h.log.Error("hiring: set settings", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not save the settings"))
	}
	h.events.Emit(ctx, requestEvent(req, "admin.hiring_settings_changed", admin.ID, map[string]any{
		"preferred_fraction": next.PreferredFraction,
	}))
	return connect.NewResponse(&v1.SetHiringSettingsResponse{
		Settings: &v1.HiringSettings{PreferredFraction: next.PreferredFraction},
	}), nil
}
