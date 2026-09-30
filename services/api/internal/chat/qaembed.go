package chat

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/reh3376/career-site/services/api/internal/ingest"
	"github.com/reh3376/career-site/services/api/internal/tenant"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Embedding the Q&A bank's phrasings.
//
// A phrasing with no vector never matches, so until this runs an
// enabled bank entry is present, correct and unreachable. That is a
// quiet failure and the worst shape one could take here: the assistant
// looks like it works, every question takes the slow path, and nothing
// says why.
//
// It runs on a timer rather than inline with writing an entry, for two
// reasons. Writing an entry is an owner action on an admin form and
// should not fail because the sidecar is restarting; and editing a
// question clears its vector deliberately (see users.UpdateQAEntry), so
// there has to be something that notices and fills it in again.

// QAEmbedStore is the slice of the repository this job needs.
type QAEmbedStore interface {
	QAPhrasingsNeedingEmbedding(ctx context.Context, limit int) ([]users.QAPhrasing, error)
	SetQAPhrasingEmbedding(ctx context.Context, phrasingID int64, embedding []float32) error
}

// QAEmbedder fills in missing phrasing vectors.
type QAEmbedder struct {
	Embed Embedder
	Store QAEmbedStore
	Log   *slog.Logger
	// Batch is how many phrasings are embedded per tick. The bank is
	// tens of entries, not thousands, so this is about bounding one
	// tick rather than about throughput.
	Batch int
}

// Run embeds one batch of phrasings.
//
// Embedded as a batch in a single call, not one call each: the sidecar
// takes a list, and on this box the per-call overhead dominates a short
// question.
//
// A tenant is attached here because the job runs on a timer with no
// request behind it, and the repository scopes every read by tenant. A
// missing one would return nothing forever and look exactly like
// "there is nothing to do".
func (q *QAEmbedder) Run(ctx context.Context) error {
	ctx = tenant.WithID(ctx, tenant.Default)

	batch := q.Batch
	if batch <= 0 {
		batch = 50
	}
	pending, err := q.Store.QAPhrasingsNeedingEmbedding(ctx, batch)
	if err != nil {
		return fmt.Errorf("qa embed: list pending: %w", err)
	}
	if len(pending) == 0 {
		return nil
	}

	texts := make([]string, 0, len(pending))
	for _, p := range pending {
		texts = append(texts, p.Text)
	}
	// PurposeQuery, not PurposeDocument, and the distinction matters.
	// These vectors are compared against an embedded *question*, so
	// they have to live in the same space as one. Embedding them as
	// documents would put every phrasing slightly off from the thing it
	// exists to match, and the symptom would be a bank that almost
	// never fires with no error anywhere.
	vecs, _, err := q.Embed.Embed(ctx, texts, ingest.PurposeQuery)
	if err != nil {
		return fmt.Errorf("qa embed: %w", err)
	}
	if len(vecs) != len(pending) {
		return fmt.Errorf("qa embed: got %d vectors for %d phrasings", len(vecs), len(pending))
	}

	var done int
	for i, p := range pending {
		if len(vecs[i]) == 0 {
			continue
		}
		if err := q.Store.SetQAPhrasingEmbedding(ctx, p.ID, vecs[i]); err != nil {
			// One bad row should not strand the rest of the batch; the
			// next tick will find it again.
			q.log().Warn("qa embed: could not store vector",
				slog.Int64("phrasing", p.ID), slog.String("error", err.Error()))
			continue
		}
		done++
	}
	if done > 0 {
		q.log().Info("qa bank phrasings embedded", slog.Int("count", done))
	}
	return nil
}

func (q *QAEmbedder) log() *slog.Logger {
	if q.Log != nil {
		return q.Log
	}
	return slog.Default()
}
