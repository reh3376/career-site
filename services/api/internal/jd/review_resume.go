package jd

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/reh3376/career-site/services/api/internal/hiring"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/runid"
)

// A résumé written after review (owner's decision, 2026-11-13; FSD
// FR-HF-08). A run that scored below the gate writes no résumé. When a
// person's verdicts lift it to the gate, the owner may ask for the
// résumé on the verdicts as they stand. It is written by the same
// writer, under the same grounding rules, through the same single
// pipeline slot, and stored in review_resumes, which the member's
// review never reads. Nothing here touches the submission's own status,
// résumé or PDF, and nothing is emailed.

// CanWriteResume reports whether this scorer can write a résumé after
// review: it needs the résumé writer and the hiring tables.
func (s *Scorer) CanWriteResume() bool {
	return s != nil && s.resume != nil && s.hiring != nil && s.hiring.Repo() != nil
}

// assessmentFromRecord is a run's record as the résumé writer reads an
// assessment: each requirement with its verdict and the passages its
// judgment cited. The verdict is the effective one, a person's where a
// person set it, read three ways as the writer's prompt expects. The
// evidence is what the judge cited and nothing more: a verdict a person
// raised adds no passage, and the writer's own source check then
// decides what it can say, so an override can redirect emphasis and
// cannot put an unsupported line on the page.
func assessmentFromRecord(rec hiring.Record) *Assessment {
	out := &Assessment{Requirements: FromRows(rec.Requirements)}
	for _, j := range rec.Judgments {
		var chunks []int64
		for _, e := range j.Evidence {
			if e.ChunkID > 0 && !e.Withdrawn {
				chunks = append(chunks, e.ChunkID)
			}
		}
		out.Judgments = append(out.Judgments, Judgment{
			RequirementID: j.LocalID,
			Verdict:       hiring.DeriveVerdict(j.EffectiveDetermination),
			Determination: j.EffectiveDetermination,
			Basis:         j.EffectiveBasis,
			EvidenceIDs:   chunks,
			NotAssessed:   j.NotAssessed,
		})
	}
	return out
}

// WriteReviewResume writes the tailored résumé for a run on its
// verdicts as they stand when the pipeline slot is reached, and stores
// it in review_resumes. The row was opened as `writing` by the caller;
// every path out of here closes it as ready or failed. ctx carries the
// corpus scope the run read. It is run in the background and returns
// nothing: the row is the result.
func (s *Scorer) WriteReviewResume(ctx context.Context, submissionID int64, runID, jdText string, hints prompts.Hints) {
	persist := context.WithoutCancel(ctx)
	if !s.CanWriteResume() {
		return
	}
	repo := s.hiring.Repo()
	fail := func(reason string) {
		s.log.Warn("jd: résumé after review failed", slog.Int64("id", submissionID), slog.String("run", runID), slog.String("error", reason))
		if err := repo.FailReviewResume(persist, runID, truncErr(reason)); err != nil {
			s.log.Warn("jd: could not record the résumé failure", slog.String("run", runID), slog.String("error", err.Error()))
		}
	}
	if !s.acquire(ctx) {
		fail("timed out waiting for the pipeline; ask for it again")
		return
	}
	defer s.release()
	ctx, cancel := context.WithTimeout(ctx, s.pipelineTimeout)
	defer cancel()
	// The model call is attributed to the run whose record it is
	// written from.
	ctx = runid.With(ctx, runID)

	// Read now, not when the button was pressed: the slot may have been
	// a long wait and the verdicts are the ones that stand.
	rec, err := repo.GetRecord(persist, runID)
	if err != nil {
		fail("the record could not be read: " + err.Error())
		return
	}
	// The posting's own retrieval, as the run's résumé stage has it.
	_, hits, err := s.Score(ctx, jdText)
	if err != nil {
		fail("retrieval for the posting failed: " + err.Error())
		return
	}
	started := time.Now()
	res, markdown, err := s.resume.Write(ctx, submissionID, jdText, hints, assessmentFromRecord(rec), hits)
	if err != nil {
		fail(err.Error())
		return
	}
	raw, _ := json.Marshal(res)
	if err := repo.FinishReviewResume(persist, runID, raw, markdown, res.Model, res.PromptID, res.PromptVers); err != nil {
		fail("storing the résumé: " + err.Error())
		return
	}
	s.log.Info("jd: résumé after review ready",
		slog.Int64("id", submissionID), slog.String("run", runID), slog.String("model", res.Model),
		slog.Int("bullets_dropped", res.Dropped), slog.Int("chars", len(markdown)), slog.Duration("took", time.Since(started)))

	// The PDF is optional, as it is for a run's own résumé: the markdown
	// is already there, so a render failure is logged and the row stays
	// ready.
	pdf, pages, err := s.resume.RenderPDF(ctx, submissionID, raw)
	if err != nil {
		s.log.Warn("jd: review résumé pdf render failed", slog.String("run", runID), slog.String("error", err.Error()))
		return
	}
	if pdf == nil {
		return
	}
	if err := repo.SetReviewResumePDF(persist, runID, pdf, pages); err != nil {
		s.log.Warn("jd: could not store the review résumé pdf", slog.String("run", runID), slog.String("error", err.Error()))
	}
}
