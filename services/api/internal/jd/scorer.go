// Package jd wires the JD-upload flow through the Ask Roger corpus:
// embed the submitted JD → retrieve → pre-score, then (when an LLM is
// available) extract requirements, judge each against retrieved
// evidence, and compute the match score in code from the verdicts.
// The résumé itself is generated in a follow-up step behind the same
// interface.
//
// The threshold (0.65) is the gate Roger set for the "generate a
// tailored résumé" path. Below that, the caller sees a polite
// fallback and Roger sees the row in /admin/jd for manual triage.
package jd

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/reh3376/career-site/services/api/internal/ingest"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// MatchThreshold is the gate above which a JD is considered a match.
// Applies to the requirement-weighted score when the assessor is
// wired, else to the retrieval pre-score.
const MatchThreshold = 0.65

// TopK caps how many chunks feed the retrieval pre-score.
const TopK = 8

// Scorer runs the JD → score pipeline. Depends on the same
// EmbedClient interface `ingest` uses, so a fake can drive tests
// without booting the gRPC stack.
type Scorer struct {
	log   *slog.Logger
	users *users.Repo
	embed ingest.EmbedClient
	// assessor is optional; nil means the retrieval pre-score is the
	// gate (dev without an LLM provider, or stub provider).
	assessor *Assessor
	// resume is optional; nil leaves above-threshold rows at
	// `generating` for manual attention.
	resume *ResumeWriter
}

// NewScorer wires the deps. assessor and resume may be nil.
func NewScorer(log *slog.Logger, repo *users.Repo, embed ingest.EmbedClient, assessor *Assessor, resume *ResumeWriter) *Scorer {
	return &Scorer{log: log, users: repo, embed: embed, assessor: assessor, resume: resume}
}

// Score embeds the JD, retrieves top-K corpus chunks, and returns the
// mean of their similarities: a cheap retrieval pre-score, monotonic
// in the strength and breadth of the match, kept for diagnostics
// alongside the requirement-weighted score.
func (s *Scorer) Score(ctx context.Context, jdText string) (score float64, hits []users.CorpusHit, err error) {
	if s.embed == nil {
		return 0, nil, errors.New("scorer: no embed client wired")
	}
	vectors, _, err := s.embed.Embed(ctx, []string{jdText}, ingest.PurposeQuery)
	if err != nil {
		return 0, nil, err
	}
	if len(vectors) != 1 {
		return 0, nil, errors.New("scorer: embedder returned no vector")
	}
	hits, err = s.users.SearchCorpus(ctx, vectors[0], TopK)
	if err != nil {
		return 0, nil, err
	}
	if len(hits) == 0 {
		return 0, hits, nil
	}
	var sum float32
	for _, h := range hits {
		sum += h.Similarity
	}
	return float64(sum) / float64(len(hits)), hits, nil
}

// ScoreAndPersist runs the pipeline for one jd_submissions row and
// writes the outcome. Best-effort: persistence failures are logged,
// the row already exists and a re-score can retry. Called from a
// goroutine by the JD handler.
func (s *Scorer) ScoreAndPersist(ctx context.Context, submissionID int64, jdText string, hints prompts.Hints) {
	if err := s.users.UpdateJdScoring(ctx, submissionID, "scoring", nil, ""); err != nil {
		s.log.Warn("jd: failed to mark scoring", slog.Int64("id", submissionID), slog.String("error", err.Error()))
	}

	retrieval, hits, err := s.Score(ctx, jdText)
	if err != nil {
		s.log.Warn("jd: score failed", slog.Int64("id", submissionID), slog.String("error", err.Error()))
		if uErr := s.users.UpdateJdScoring(ctx, submissionID, "failed", nil, truncErr(err.Error())); uErr != nil {
			s.log.Warn("jd: failed to record failure", slog.Int64("id", submissionID), slog.String("error", uErr.Error()))
		}
		return
	}
	if len(hits) == 0 {
		s.log.Info("jd: no corpus hits", slog.Int64("id", submissionID))
		_ = s.users.UpdateJdScoring(ctx, submissionID, "below_threshold", &retrieval,
			"corpus is empty: no chunks available to score against")
		return
	}

	score := retrieval
	var assessment *Assessment
	if s.assessor != nil {
		started := time.Now()
		assessment, err = s.assessor.Assess(ctx, submissionID, jdText, hints)
		if err != nil {
			// Fall back to the retrieval pre-score but keep the failure
			// on the record so the admin can see the gate was degraded.
			s.log.Warn("jd: assessment failed, using retrieval score",
				slog.Int64("id", submissionID), slog.String("error", err.Error()))
			assessment = &Assessment{Error: truncErr(err.Error())}
		} else {
			score = assessment.Score
			s.log.Info("jd: assessed",
				slog.Int64("id", submissionID),
				slog.Int("requirements", len(assessment.Requirements)),
				slog.Float64("score", assessment.Score),
				slog.Float64("retrieval_score", retrieval),
				slog.Duration("took", time.Since(started)),
			)
		}
		if raw, mErr := json.Marshal(assessment); mErr == nil {
			if uErr := s.users.SetJdAssessment(ctx, submissionID, retrieval, raw); uErr != nil {
				s.log.Warn("jd: failed to store assessment", slog.Int64("id", submissionID), slog.String("error", uErr.Error()))
			}
		}
	} else if uErr := s.users.SetJdAssessment(ctx, submissionID, retrieval, nil); uErr != nil {
		s.log.Warn("jd: failed to store retrieval score", slog.Int64("id", submissionID), slog.String("error", uErr.Error()))
	}

	next := "below_threshold"
	if score >= MatchThreshold {
		next = "generating"
	}
	if err := s.users.UpdateJdScoring(ctx, submissionID, next, &score, ""); err != nil {
		s.log.Warn("jd: failed to record score",
			slog.Int64("id", submissionID),
			slog.Float64("score", score),
			slog.String("error", err.Error()),
		)
		return
	}
	s.log.Info("jd: scored",
		slog.Int64("id", submissionID),
		slog.Float64("score", score),
		slog.String("status", next),
		slog.Int("top_hits", len(hits)),
	)
	if next != "generating" || s.resume == nil {
		return
	}

	// Grounded résumé: one model call, sources verified in code.
	var usable *Assessment
	if assessment != nil && assessment.Error == "" {
		usable = assessment
	}
	started := time.Now()
	res, markdown, err := s.resume.Write(ctx, submissionID, jdText, hints, usable, hits)
	if err != nil {
		s.log.Warn("jd: résumé failed", slog.Int64("id", submissionID), slog.String("error", err.Error()))
		if uErr := s.users.UpdateJdScoring(ctx, submissionID, "failed", &score, "résumé generation failed: "+truncErr(err.Error())); uErr != nil {
			s.log.Warn("jd: failed to record résumé failure", slog.Int64("id", submissionID), slog.String("error", uErr.Error()))
		}
		return
	}
	raw, _ := json.Marshal(res)
	if err := s.users.SetJdResume(ctx, submissionID, raw, markdown, res.Model, res.PromptID, res.PromptVers); err != nil {
		s.log.Warn("jd: failed to store résumé", slog.Int64("id", submissionID), slog.String("error", err.Error()))
		return
	}
	s.log.Info("jd: résumé ready",
		slog.Int64("id", submissionID),
		slog.String("model", res.Model),
		slog.Int("bullets_dropped", res.Dropped),
		slog.Int("chars", len(markdown)),
		slog.Duration("took", time.Since(started)),
	)

	// PDF is optional: the markdown is already deliverable, so a render
	// failure is logged and the row stays ready.
	pdf, pages, err := s.resume.RenderPDF(ctx, submissionID, raw)
	if err != nil {
		s.log.Warn("jd: pdf render failed", slog.Int64("id", submissionID), slog.String("error", err.Error()))
		return
	}
	if pdf == nil {
		return
	}
	if err := s.users.SetJdResumePDF(ctx, submissionID, pdf, pages, s.resume.DownloadPath(submissionID)); err != nil {
		s.log.Warn("jd: failed to store pdf", slog.Int64("id", submissionID), slog.String("error", err.Error()))
		return
	}
	s.log.Info("jd: pdf ready", slog.Int64("id", submissionID), slog.Int("bytes", len(pdf)), slog.Int("pages", int(pages)))
}

// truncErr keeps a persisted error short: the row's `error` column is
// UI-facing on /admin/jd; a stack trace is out of place.
func truncErr(s string) string {
	const max = 500
	if len(s) <= max {
		return s
	}
	return s[:max]
}
