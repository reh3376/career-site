package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// SweepOptions bounds one embed sweep. MaxChunks caps the total work
// per call so an admin click can't run for an hour; BatchSize matches
// the sidecar's 64-text Embed limit.
type SweepOptions struct {
	MaxChunks int
	BatchSize int
}

// SweepResult reports what one sweep did. Remaining is the number of
// chunks still stale for Model after this call; the admin keeps
// clicking (or a scheduler keeps calling) until it reaches zero.
type SweepResult struct {
	Model      string
	Considered int
	Embedded   int
	Failed     int
	Remaining  int32
	StartedAt  time.Time
	FinishedAt time.Time
}

// EmbedSweep re-embeds every chunk whose vector is missing or was
// produced by a different embedder than the sidecar's current one.
// This is what makes a provider flip (stub → ollama, or a model
// upgrade) safe: nothing is deleted, the sweep just walks the corpus
// until every chunk carries the current model's vector.
//
// The current model is learned with a one-text probe so the sweep
// never has to be told which provider is live.
func (i *Ingester) EmbedSweep(ctx context.Context, opts SweepOptions) (*SweepResult, error) {
	if opts.BatchSize <= 0 || opts.BatchSize > 64 {
		opts.BatchSize = 64
	}
	if opts.MaxChunks <= 0 {
		opts.MaxChunks = 512
	}
	res := &SweepResult{StartedAt: time.Now().UTC()}

	_, model, err := i.sidecar.Embed(ctx, []string{"embedder probe"})
	if err != nil {
		return nil, fmt.Errorf("sweep: probe embedder: %w", err)
	}
	if model == "" {
		return nil, fmt.Errorf("sweep: sidecar reported an empty model name")
	}
	res.Model = model

	for res.Considered < opts.MaxChunks {
		limit := min(opts.BatchSize, opts.MaxChunks-res.Considered)
		chunks, err := i.users.ListChunksNeedingEmbedding(ctx, model, limit)
		if err != nil {
			return res, fmt.Errorf("sweep: list: %w", err)
		}
		if len(chunks) == 0 {
			break
		}
		res.Considered += len(chunks)

		texts := make([]string, len(chunks))
		for j, c := range chunks {
			texts[j] = c.Text
		}
		vectors, gotModel, err := i.sidecar.Embed(ctx, texts)
		if err != nil {
			// A failing batch is not retried here; the chunks stay stale
			// and the next sweep picks them up. Stop rather than spin.
			res.Failed += len(chunks)
			i.log.Warn("embed sweep batch failed", slog.Int("batch", len(chunks)),
				slog.String("error", err.Error()))
			break
		}
		if gotModel != model {
			return res, fmt.Errorf("sweep: embedder changed mid-sweep (%s → %s)", model, gotModel)
		}
		for j, emb := range vectors {
			if err := i.users.SetCorpusChunkEmbedding(ctx, chunks[j].ID, emb, model); err != nil {
				res.Failed++
				i.log.Warn("embed sweep set failed", slog.Int64("chunk_id", chunks[j].ID),
					slog.String("error", err.Error()))
				continue
			}
			res.Embedded++
		}
		// A batch that embedded nothing would loop forever on the same
		// rows; bail and let the admin see Failed > 0.
		if res.Embedded == 0 && res.Failed > 0 {
			break
		}
	}

	remaining, err := i.users.CountChunksNeedingEmbedding(ctx, model)
	if err != nil {
		return res, fmt.Errorf("sweep: count remaining: %w", err)
	}
	res.Remaining = remaining
	res.FinishedAt = time.Now().UTC()
	i.log.Info("embed sweep done",
		slog.String("model", model), slog.Int("considered", res.Considered),
		slog.Int("embedded", res.Embedded), slog.Int("failed", res.Failed),
		slog.Int("remaining", int(res.Remaining)))
	return res, nil
}
