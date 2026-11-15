package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/corpusscope"
	"github.com/reh3376/career-site/services/api/internal/hiring"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// The match record as a person reads it, and the override of a
// judgment (FSD FR-HF-08, FR-HF-17; Phase 0 step 6).
//
// Two readers, two records of the same run. An admin is sent the record
// as it stands: the effective verdicts, which are a person's where one
// was set, the numbers computed over them, who overrode what and why,
// which document and passage each judgment cites, and the decision-log
// row behind it. The member who submitted the posting is sent the
// record as the model alone left it (hiring.Record.ModelOnly): the
// model's verdicts, the numbers computed over those, and no override
// (owner's decision, 2026-11-13). In Phase 0 the only person who can
// override is the candidate, standing in for a hiring manager, and a
// recruiter is shown what the model found, not what the candidate made
// of it. A member is also sent the kind of evidence and the title of a
// document only when the document is public: the private corpus stays
// private even by its titles.

// recordAudience says who a record is being built for.
type recordAudience struct {
	admin bool
}

func coverageProto(rec hiring.Record) *v1.Coverage {
	if !rec.HasNumbers {
		return nil
	}
	return &v1.Coverage{
		Evidenced: rec.Coverage.Evidenced, Unresolved: rec.Coverage.Unresolved, Unmet: rec.Coverage.Unmet,
		EquivalentShare: rec.Coverage.EquivalentShare, SelfAttestedShare: rec.Coverage.SelfAttestedShare,
		FormulaVersion: rec.FormulaVersion, PreferredFraction: rec.PreferredFraction,
	}
}

func judgmentProto(j hiring.RecordJudgment, aud recordAudience) *v1.Judgment {
	out := &v1.Judgment{
		Id: j.ID, RequirementId: j.RequirementID, RequirementLocalId: j.LocalID,
		Determination: j.Determination, Basis: j.Basis, Uncertainty: j.Uncertainty,
		EffectiveDetermination: j.EffectiveDetermination, EffectiveBasis: j.EffectiveBasis,
		DerivedVerdict: j.DerivedVerdict, Rationale: j.Rationale,
		GapClass: j.GapClass, NotAssessed: j.NotAssessed, Stale: j.Stale,
		KeywordKnown: j.KeywordKnown, KeywordFound: j.KeywordFound, KeywordTerms: j.KeywordTerms,
		ReportedFacts: &v1.ReportedFacts{
			StatedSpanYears: j.Facts.StatedSpanYears, MostRecentYear: int32(j.Facts.MostRecentYear),
			PartiesEvidenced: j.Facts.PartiesEvidenced,
		},
	}
	if s := j.Facts.StatedScope; s != nil && s.Value > 0 {
		out.ReportedFacts.StatedScope = &v1.ScopeThreshold{Unit: s.Unit, Value: s.Value}
	}
	for _, e := range j.Evidence {
		ref := &v1.EvidenceRef{Id: e.ID, SourceKind: e.SourceKind, Withdrawn: e.Withdrawn}
		if aud.admin {
			ref.Title, ref.ChunkId = e.Title, e.ChunkID
		} else if e.Public {
			ref.Title = e.Title
		}
		out.Evidence = append(out.Evidence, ref)
	}
	for _, a := range j.RuleAdjustments {
		out.RuleAdjustments = append(out.RuleAdjustments, &v1.RuleAdjustment{
			Rule: a.Rule, RuleVersion: a.RuleVersion,
			FromDetermination: a.FromDetermination, ToDetermination: a.ToDetermination,
			FromBasis: a.FromBasis, ToBasis: a.ToBasis,
			AddedUncertainty: a.AddedUncertainty, Reason: a.Reason,
		})
	}
	// An override is the hiring side's and is sent to an admin only. A
	// member's record was made model-only before it reached here; the
	// guard is kept so that a caller who forgets cannot leak one.
	if o := j.Override; o != nil && aud.admin {
		out.Override = &v1.JudgmentOverride{
			Determination: o.Determination, Basis: o.Basis, Role: o.Role,
			ByDisplay: o.ByDisplay, Reason: o.Reason,
			CarriedFromAttempt: int32(o.CarriedFromAttempt), Recheck: o.Recheck,
		}
		if o.CarriedFromAttempt > 0 {
			out.Override.ModelDeterminationThen, out.Override.ModelBasisThen = o.ModelDeterminationThen, o.ModelBasisThen
		}
		if !o.At.IsZero() {
			out.Override.At = timestamppb.New(o.At)
		}
	}
	if aud.admin {
		out.DecisionId = j.DecisionID
		if e := j.Earlier; e != nil {
			out.EarlierOverride = &v1.EarlierOverride{
				Attempt: int32(e.Attempt), Determination: e.Determination, Basis: e.Basis,
				Role: e.Role, SameVersion: e.SameVersion,
			}
			if !e.At.IsZero() {
				out.EarlierOverride.At = timestamppb.New(e.At)
			}
		}
	}
	return out
}

// recordProto renders a record for its reader. title and employer are
// the submitter's own words for the role, which the record's role row
// only approximates. A reader who is not an admin is rendered the
// model-only record, whatever was passed in.
func recordProto(rec hiring.Record, title, employer string, aud recordAudience) *v1.MatchRecord {
	if !aud.admin {
		rec = rec.ModelOnly()
	}
	if strings.TrimSpace(title) == "" {
		title = rec.RoleTitle
	}
	out := &v1.MatchRecord{
		RunId: rec.RunID, RoleId: rec.RoleID, SubmissionId: strconv.FormatInt(rec.SubmissionID, 10),
		Attempt: int32(rec.Attempt), SupersedesRunId: rec.SupersedesRunID, Superseded: rec.Superseded,
		RunTrigger: rec.RunTrigger, RoleTitle: strings.TrimSpace(title), Employer: strings.TrimSpace(employer),
		Coverage:      coverageProto(rec),
		OverrideCount: int32(rec.OverrideCount),
		KeywordRule:   rec.KeywordRule, KeywordSkipped: rec.KeywordSkipped,
		Versions: rec.Versions, Fit: rec.Fit, MatchScore: rec.MatchScore,
		CorpusDocuments: int32(rec.CorpusDocuments), Model: rec.Model,
		RequirementsEdited: rec.RequirementsEdited,
	}
	if rec.FinishedAt != nil {
		out.FinishedAt = timestamppb.New(*rec.FinishedAt)
	}
	for _, q := range rec.Requirements {
		p := requirementProto(q)
		if !aud.admin {
			// The editor's name and reason are the hiring side's working
			// notes; a member is shown that a requirement was edited, by
			// its edited_at, and not by whom or why.
			p.EditedByDisplay, p.EditReason = "", ""
		}
		out.Requirements = append(out.Requirements, p)
	}
	for _, j := range rec.Judgments {
		out.Judgments = append(out.Judgments, judgmentProto(j, aud))
	}
	return out
}

// overrideRoles is the override_role vocabulary without the role
// reserved for a candidate's dispute (D-38).
func overrideRoles(reg *hiring.Registry) []*v1.VocabCode {
	var out []*v1.VocabCode
	for _, c := range vocabProto(reg, hiring.VocabOverrideRole) {
		if c.Code != hiring.RoleCandidateDispute {
			out = append(out, c)
		}
	}
	return out
}

// overrideChoices pairs each determination with the bases that do not
// contradict it, in the registry's order.
func overrideChoices(reg *hiring.Registry) []*v1.OverrideChoice {
	var bases []string
	for _, c := range reg.Codes(hiring.VocabBasis) {
		if c.RetiredIn == "" {
			bases = append(bases, c.Code)
		}
	}
	var out []*v1.OverrideChoice
	for _, d := range reg.Codes(hiring.VocabDetermination) {
		if d.RetiredIn != "" {
			continue
		}
		if allowed := hiring.OverrideBases(d.Code, bases); len(allowed) > 0 {
			out = append(out, &v1.OverrideChoice{Determination: d.Code, Bases: allowed})
		}
	}
	return out
}

// GetMatchRecord returns one run's match record to the member who
// submitted the posting or to an admin (FR-HF-17).
func (h *Hiring) GetMatchRecord(
	ctx context.Context,
	req *connect.Request[v1.GetMatchRecordRequest],
) (*connect.Response[v1.GetMatchRecordResponse], error) {
	caller, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	notFound := connect.NewError(connect.CodeNotFound, errors.New("submission not found"))
	id, err := strconv.ParseInt(strings.TrimSpace(req.Msg.GetSubmissionId()), 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid submission_id"))
	}
	s, err := h.users.GetJdSubmission(ctx, id)
	if err != nil {
		return nil, notFound
	}
	isAdmin := caller.Role == users.RoleAdmin
	// A member reads the record of a posting they submitted and no
	// other; the same not-found as a missing id, so an id cannot be
	// probed.
	if !isAdmin && (s.UserID == 0 || s.UserID != caller.ID) {
		return nil, notFound
	}
	// An admin may ask for the record as the member is sent it, to see
	// what a recruiter sees.
	aud := recordAudience{admin: isAdmin && !req.Msg.GetAsMember()}
	out := &v1.GetMatchRecordResponse{}
	reg, vErr := h.repo.LoadVocab(ctx)
	if vErr != nil {
		h.log.Warn("hiring: vocabulary unavailable for the record", slog.String("error", vErr.Error()))
	} else {
		out.Determinations = vocabProto(reg, hiring.VocabDetermination)
		out.Bases = vocabProto(reg, hiring.VocabBasis)
		out.Uncertainties = vocabProto(reg, hiring.VocabUncertainty)
		out.Types = vocabProto(reg, hiring.VocabRequirementType)
		out.Classes = vocabProto(reg, hiring.VocabRequirementClass)
		out.SourceKinds = vocabProto(reg, hiring.VocabSourceKind)
		out.OverrideRoles = overrideRoles(reg)
	}

	runID := strings.TrimSpace(req.Msg.GetRunId())
	if runID == "" {
		latest, _, _, ok, err := h.repo.LatestRecordRun(ctx, id)
		if err != nil {
			h.log.Error("hiring: latest record run", slog.Int64("submission", id), slog.String("error", err.Error()))
			return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the record"))
		}
		if !ok {
			// A posting last scored before the judgment rows began has no
			// record in this form; that is an answer, not an error.
			return connect.NewResponse(out), nil
		}
		runID = latest
	} else if !looksLikeUUID(runID) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid run_id"))
	}
	rec, err := h.repo.GetRecord(ctx, runID)
	if errors.Is(err, hiring.ErrNotFound) {
		if req.Msg.GetRunId() == "" {
			return connect.NewResponse(out), nil
		}
		return nil, connect.NewError(connect.CodeNotFound, errors.New("that run has no match record"))
	}
	if err != nil {
		h.log.Error("hiring: match record", slog.String("run", runID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the record"))
	}
	// A run id names a run of this posting or nothing: the ownership
	// check above was made on the submission.
	if rec.SubmissionID != id {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("that run has no match record"))
	}
	out.Record = h.recordFor(ctx, rec, s.RoleHint, s.EmployerHint, aud)
	if aud.admin && !rec.Superseded && reg != nil {
		out.CanOverride = true
		out.OverrideChoices = overrideChoices(reg)
	}
	// The résumé after review is the hiring side's and is sent to an
	// admin only; a member's response never names it.
	if aud.admin {
		existing, has, rErr := h.repo.GetReviewResume(ctx, rec.RunID)
		if rErr != nil {
			h.log.Warn("hiring: review résumé unavailable", slog.String("run", rec.RunID), slog.String("error", rErr.Error()))
		}
		if has {
			out.ReviewResume = reviewResumeProto(existing)
		}
		out.CanWriteReviewResume, out.ReviewResumeNote = h.reviewResumeOffer(ctx, rec, has && existing.Status == hiring.ReviewResumeWriting)
	}
	return connect.NewResponse(out), nil
}

func reviewResumeProto(x hiring.ReviewResume) *v1.ReviewResume {
	out := &v1.ReviewResume{
		RunId: x.RunID, Status: x.Status, Error: x.Error, Markdown: x.Markdown,
		RequestedAt:   timestamppb.New(x.RequestedAt),
		ReviewedScore: x.ReviewedScore, OverrideCount: int32(x.OverrideCount), Model: x.Model,
	}
	if x.FinishedAt != nil {
		out.FinishedAt = timestamppb.New(*x.FinishedAt)
	}
	if x.HasPDF {
		out.DownloadPath = ReviewResumeDownloadPath(x.RunID)
	}
	return out
}

// ReviewResumeDownloadPath is where an admin's session downloads the
// PDF of a résumé after review.
func ReviewResumeDownloadPath(runID string) string {
	return "/api/hiring/review-resume/" + runID + ".pdf"
}

// gate is the résumé gate in force, the lower edge of the strong band.
func (h *Hiring) gate(ctx context.Context) (float64, bool) {
	switch {
	case h.bands != nil:
		return h.bands.Get(ctx).Strong, true
	case h.scorer != nil:
		return h.scorer.Threshold(), true
	}
	return 0, false
}

// reviewResumeOffer says whether a résumé after review may be asked for
// now and, when the record carries an override and it may not, why not,
// as a sentence for the owner (owner's decision, 2026-11-13). It is
// offered for the newest record of a run that finished below the gate,
// and so wrote no résumé, once a person's verdicts lift the gate's
// score to the gate. A record with no override is offered nothing and
// told nothing: there is no review to write on.
func (h *Hiring) reviewResumeOffer(ctx context.Context, rec hiring.Record, writing bool) (bool, string) {
	score, reviewed := rec.ReviewedScore()
	if !reviewed {
		return false, ""
	}
	if writing {
		return false, ""
	}
	if rec.Superseded {
		return false, "A later run has replaced this record; a résumé after review is written from the newest one."
	}
	if rec.RunResumeGenerated {
		return false, "This run wrote a résumé of its own, so none is written after review."
	}
	if rec.RunStatus != "below_threshold" {
		return false, "A résumé after review is offered for a run that finished below the résumé gate."
	}
	gate, ok := h.gate(ctx)
	if !ok || h.resumes == nil || !h.resumes.CanWriteResume() {
		return false, "The résumé writer is not available here."
	}
	if score < gate {
		return false, fmt.Sprintf("The score after review, %.2f, is below the résumé gate of %.2f, so no résumé is offered.", score, gate)
	}
	return true, ""
}

// WriteReviewResume asks for the tailored résumé on a run's verdicts
// after review (FR-HF-08). It opens the row as being written and starts
// the write in the background; the row is the result.
func (h *Hiring) WriteReviewResume(
	ctx context.Context,
	req *connect.Request[v1.WriteReviewResumeRequest],
) (*connect.Response[v1.WriteReviewResumeResponse], error) {
	admin, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	runID := strings.TrimSpace(req.Msg.GetRunId())
	if !looksLikeUUID(runID) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid run_id"))
	}
	rec, err := h.repo.GetRecord(ctx, runID)
	if errors.Is(err, hiring.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("that run has no match record"))
	}
	if err != nil {
		h.log.Error("hiring: record for a review résumé", slog.String("run", runID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the record"))
	}
	existing, has, err := h.repo.GetReviewResume(ctx, runID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the résumé"))
	}
	if has && existing.Status == hiring.ReviewResumeWriting {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("a résumé is already being written for this run"))
	}
	ok, why := h.reviewResumeOffer(ctx, rec, false)
	if !ok {
		if why == "" {
			why = "No verdict on this record has been set by a person, so there is no review to write a résumé on."
		}
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New(why))
	}
	s, err := h.users.GetJdSubmission(ctx, rec.SubmissionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("the run's posting was not found"))
	}
	score, _ := rec.ReviewedScore()
	gate, _ := h.gate(ctx)
	if err := h.repo.StartReviewResume(ctx, hiring.ReviewResumeRequest{
		RunID: runID, SubmissionID: rec.SubmissionID, ReviewedScore: score, Threshold: gate,
		OverrideCount: rec.OverrideCount, RequestedBy: admin.ID,
	}); err != nil {
		if errors.Is(err, hiring.ErrReviewResumeBusy) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("a résumé is already being written for this run"))
		}
		h.log.Error("hiring: start review résumé", slog.String("run", runID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("the résumé could not be started"))
	}
	// Written under the scope the run itself read, so the résumé cannot
	// draw on documents the review of this posting was not allowed to.
	scope := corpusscope.Public
	if rec.RunRetrievalScope == "all" {
		scope = corpusscope.All
	}
	hints := prompts.Hints{Role: s.RoleHint, Employer: s.EmployerHint}
	go h.resumes.WriteReviewResume(corpusscope.With(context.Background(), scope), rec.SubmissionID, runID, s.JdText, hints)

	h.log.Info("hiring: résumé after review requested", slog.String("run", runID), slog.Int64("submission", rec.SubmissionID),
		slog.Float64("reviewed_score", score), slog.Int64("admin_id", admin.ID))
	h.events.Emit(ctx, requestEvent(req, "admin.review_resume_requested", admin.ID, map[string]any{
		"run_id": runID, "submission_id": rec.SubmissionID, "reviewed_score": score, "threshold": gate,
		"override_count": rec.OverrideCount, "again": has,
	}))
	row, _, err := h.repo.GetReviewResume(ctx, runID)
	if err != nil {
		return connect.NewResponse(&v1.WriteReviewResumeResponse{}), nil
	}
	return connect.NewResponse(&v1.WriteReviewResumeResponse{ReviewResume: reviewResumeProto(row)}), nil
}

// overrideError maps the override's errors onto Connect codes with the
// sentence the package wrote for the reviewer.
func overrideError(err error) error {
	switch {
	case errors.Is(err, hiring.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("judgment not found"))
	case errors.Is(err, hiring.ErrInvalidOverride):
		msg := strings.TrimPrefix(err.Error(), hiring.ErrInvalidOverride.Error()+": ")
		return connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
	case errors.Is(err, hiring.ErrOverrideRefused):
		msg := strings.TrimPrefix(err.Error(), hiring.ErrOverrideRefused.Error()+": ")
		return connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
	}
	return connect.NewError(connect.CodeInternal, errors.New("the override could not be saved"))
}

// OverrideJudgment sets, replaces or withdraws a person's override of
// one judgment (FR-HF-08).
func (h *Hiring) OverrideJudgment(
	ctx context.Context,
	req *connect.Request[v1.OverrideJudgmentRequest],
) (*connect.Response[v1.OverrideJudgmentResponse], error) {
	admin, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	id := strings.TrimSpace(req.Msg.GetJudgmentId())
	if !looksLikeUUID(id) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid judgment_id"))
	}
	// The registry validates the codes. Without it the compiled-in
	// vocabulary does, which is the same list at the version this build
	// writes.
	reg, vErr := h.repo.LoadVocab(ctx)
	if vErr != nil {
		h.log.Warn("hiring: vocabulary unavailable for an override; validating against the compiled-in codes", slog.String("error", vErr.Error()))
		reg = nil
	}
	res, err := h.repo.OverrideJudgment(ctx, hiring.OverrideInput{
		JudgmentID: id, Determination: req.Msg.GetDetermination(), Basis: req.Msg.GetBasis(),
		Reason: req.Msg.GetReason(), Role: req.Msg.GetRole(), Withdraw: req.Msg.GetWithdraw(), By: admin.ID,
	}, reg)
	if err != nil {
		if !errors.Is(err, hiring.ErrInvalidOverride) && !errors.Is(err, hiring.ErrOverrideRefused) && !errors.Is(err, hiring.ErrNotFound) {
			h.log.Error("hiring: override judgment", slog.String("id", id), slog.String("error", err.Error()))
		}
		return nil, overrideError(err)
	}
	action := "override"
	if req.Msg.GetWithdraw() {
		action = "withdraw"
	}
	h.log.Info("hiring: judgment overridden", slog.String("run", res.RunID), slog.String("local_id", res.LocalID),
		slog.String("action", action), slog.String("from", res.PreviousEffective), slog.String("to", res.Effective),
		slog.Int64("admin_id", admin.ID))
	h.events.Emit(ctx, requestEvent(req, "admin.judgment_overridden", admin.ID, map[string]any{
		"judgment_id": id, "run_id": res.RunID, "submission_id": res.SubmissionID, "local_id": res.LocalID,
		"action": action, "role": strings.TrimSpace(req.Msg.GetRole()),
		"model_determination": res.ModelDetermined, "model_basis": res.ModelBasis,
		"determination": res.Effective, "basis": res.EffectiveBasis,
		"agrees_with_model": res.Effective == res.ModelDetermined && res.EffectiveBasis == res.ModelBasis,
		"override_count":    res.OverrideCount,
	}))
	rec, err := h.repo.GetRecord(ctx, res.RunID)
	if err != nil {
		// The override is saved; only the read-back failed.
		h.log.Error("hiring: record after override", slog.String("run", res.RunID), slog.String("error", err.Error()))
		return connect.NewResponse(&v1.OverrideJudgmentResponse{}), nil
	}
	title, employer := "", ""
	if s, sErr := h.users.GetJdSubmission(ctx, res.SubmissionID); sErr == nil {
		title, employer = s.RoleHint, s.EmployerHint
	}
	return connect.NewResponse(&v1.OverrideJudgmentResponse{
		Record: h.recordFor(ctx, rec, title, employer, recordAudience{admin: true}),
	}), nil
}

// recordFor renders a record for its reader and, for an admin reading a
// record that carries an override, adds the score the résumé gate
// reads worked out again with each person's verdict in place of the
// model's, and the fit it falls in (owner's decision, 2026-11-13: both
// are shown, the run's own never replaced). The bands are the ones in
// force now, which is what "what would this be today" means; without a
// band store the score is sent and the fit left empty.
func (h *Hiring) recordFor(ctx context.Context, rec hiring.Record, title, employer string, aud recordAudience) *v1.MatchRecord {
	out := recordProto(rec, title, employer, aud)
	if !aud.admin {
		return out
	}
	if score, ok := rec.ReviewedScore(); ok {
		out.ReviewedScore = &score
		switch {
		case h.bands != nil:
			out.ReviewedFit = h.bands.Get(ctx).Category(score)
		case h.scorer != nil:
			out.ReviewedFit = h.scorer.Bands(ctx).Category(score)
		}
	}
	return out
}
