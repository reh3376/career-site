// Package jd wires the JD-upload flow through the Ask Roger corpus:
// embed the submitted JD → retrieve top-K chunks → aggregate to a
// match score → update the jd_submissions row → return the hits
// so a follow-up LLM pass can generate the tailored résumé.
//
// The threshold (0.65) is the gate Roger set for the "generate a
// tailored résumé" path. Below that, the caller sees a polite
// fallback and Roger sees the row in /admin/jd for manual triage.
package jd

import (
	"context"
	"errors"
	"log/slog"

	"github.com/reh3376/career-site/services/api/internal/ingest"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Threshold above which a JD is considered a match. Anything below
// takes the fallback path (no résumé generation, human triage).
// Chosen in the /jd-upload copy — change here + on the page.
const MatchThreshold = 0.65

// TopK caps how many chunks influence the score. Small enough that
// a JD hitting a couple of strong career notes doesn't get drowned
// by a long tail of low-similarity noise; large enough that we
// aren't over-fitting to one chunk.
const TopK = 8

// Scorer runs the JD → score pipeline. Depends on the same
// EmbedClient interface `ingest` uses, so a fake can drive tests
// without booting the gRPC stack.
type Scorer struct {
	log   *slog.Logger
	users *users.Repo
	embed ingest.EmbedClient
}

// NewScorer wires the deps.
func NewScorer(log *slog.Logger, repo *users.Repo, embed ingest.EmbedClient) *Scorer {
	return &Scorer{log: log, users: repo, embed: embed}
}

// Score embeds the JD, retrieves top-K corpus chunks, aggregates to
// a match score, and returns both. Aggregation is the mean of the
// top-K similarity values — simple, monotonic in the strength and
// breadth of the match. When the corpus has fewer than TopK
// embedded chunks the mean is over what exists.
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

// ScoreAndPersist runs the scoring pass for one jd_submissions row
// and writes the outcome. Best-effort — swallows persistence
// failures (they're logged) since the row already exists and a
// re-score can retry. Called from a goroutine by the JD handler.
func (s *Scorer) ScoreAndPersist(ctx context.Context, submissionID int64, jdText string) {
	if err := s.users.UpdateJdScoring(ctx, submissionID, "scoring", nil, ""); err != nil {
		s.log.Warn("jd: failed to mark scoring", slog.Int64("id", submissionID), slog.String("error", err.Error()))
	}

	score, hits, err := s.Score(ctx, jdText)
	if err != nil {
		s.log.Warn("jd: score failed",
			slog.Int64("id", submissionID),
			slog.String("error", err.Error()),
		)
		if uErr := s.users.UpdateJdScoring(ctx, submissionID, "failed", nil, truncErr(err.Error())); uErr != nil {
			s.log.Warn("jd: failed to record failure", slog.Int64("id", submissionID), slog.String("error", uErr.Error()))
		}
		return
	}

	// If the corpus is empty (no embedded chunks), fall through to
	// below_threshold with an explanatory note so the admin knows
	// this wasn't a real "poor match" verdict.
	if len(hits) == 0 {
		s.log.Info("jd: no corpus hits", slog.Int64("id", submissionID))
		_ = s.users.UpdateJdScoring(ctx, submissionID, "below_threshold", &score,
			"corpus is empty — no chunks available to score against")
		return
	}

	next := "below_threshold"
	if score >= MatchThreshold {
		// Cross the threshold → mark generating. LLM résumé
		// generation lands in a follow-up PR; until then, the
		// admin view flags these for manual attention.
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
}

// truncErr keeps a persisted error short — the row's `error`
// column is UI-facing on /admin/jd; a stack trace is out of place.
func truncErr(s string) string {
	const max = 500
	if len(s) <= max {
		return s
	}
	return s[:max]
}
