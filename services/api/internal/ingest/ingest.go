package ingest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/reh3376/career-site/services/api/internal/users"
)

// Ingester turns raw source text into corpus rows + embeddings. Its
// contract is deliberately simple so a follow-up filesystem walker,
// admin form, and sidecar CLI can all reuse the same code path:
//
//	IngestText(ctx, in) → summary
//
// The insert-first / embed-later pattern is honoured: the document
// and chunks are written before the sidecar Embed call, so a
// mid-ingest failure leaves a partially-embedded row rather than
// losing the text entirely. A follow-up embed-sweep RPC (not built
// yet) can fill in missing embeddings.
type Ingester struct {
	log     *slog.Logger
	users   *users.Repo
	sidecar EmbedClient
	chunker Chunker
}

// EmbedClient is the shape Ingester needs from the sidecar client:
// batch a list of texts into vectors plus a model identifier. Kept
// deliberately narrow so tests can pass a fake without booting the
// gRPC stack, and so the ingest package doesn't depend on the
// generated proto types.
type EmbedClient interface {
	Embed(ctx context.Context, texts []string) (vectors [][]float32, model string, err error)
}

// NewIngester wires the dependencies. Passing nil for chunker uses
// the v1 ParagraphChunker with default options.
func NewIngester(log *slog.Logger, repo *users.Repo, sc EmbedClient, chunker Chunker) *Ingester {
	if chunker == nil {
		chunker = NewParagraphChunker(Options{})
	}
	return &Ingester{log: log, users: repo, sidecar: sc, chunker: chunker}
}

// IngestInput is one document to ingest. SourceKind is a slug like
// "article" / "resume" / "career_note". SourcePath is a stable key
// per source_kind — for pasted text, callers pass a synthesised key
// like `paste/<slug>`; for a filesystem source, the relative path.
type IngestInput struct {
	SourceKind string
	SourcePath string
	Title      string
	Body       string // raw text (post-front-matter for markdown)
	Meta       []byte // jsonb, optional
}

// IngestResult reports what the ingester did with one document.
// Skipped=true means the content_hash matched the stored row and
// no chunking / embedding ran.
type IngestResult struct {
	DocumentID     int64
	ChunksInserted int
	ChunksEmbedded int
	Skipped        bool
	ChunkerName    string
	EmbedderModel  string
	StartedAt      time.Time
	CompletedAt    time.Time
}

// ErrEmpty is returned when the caller passes an empty body. Ingest
// treats that as a caller error, not something to store as an empty
// document.
var ErrEmpty = errors.New("ingest: body is empty")

// IngestText runs the full pipeline for one document.
func (i *Ingester) IngestText(ctx context.Context, in IngestInput) (*IngestResult, error) {
	res := &IngestResult{StartedAt: time.Now().UTC(), ChunkerName: i.chunker.Name()}

	body := strings.TrimSpace(in.Body)
	if body == "" {
		return nil, ErrEmpty
	}
	if in.SourceKind == "" || in.SourcePath == "" {
		return nil, errors.New("ingest: source_kind and source_path are required")
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = firstNonEmptyLine(body)
	}
	if title == "" {
		title = in.SourcePath
	}

	newHash := users.HashCorpusContent(body)

	// Content-hash short-circuit: if a document with this
	// (source_kind, source_path) already exists AND its content_hash
	// equals what we're about to store, nothing has changed —
	// re-ingest is a no-op. Callers pay the SELECT but nothing else.
	if existing, err := i.users.GetCorpusDocumentByPath(ctx, in.SourceKind, in.SourcePath); err == nil {
		if existing != nil && bytes.Equal(existing.ContentHash, newHash) {
			res.DocumentID = existing.ID
			res.Skipped = true
			res.CompletedAt = time.Now().UTC()
			i.log.Info("ingest skipped (unchanged)",
				slog.String("source_kind", in.SourceKind),
				slog.String("source_path", in.SourcePath),
				slog.Int64("document_id", existing.ID),
			)
			return res, nil
		}
	}

	doc, err := i.users.UpsertCorpusDocument(ctx, users.CorpusDocument{
		SourceKind:  in.SourceKind,
		SourcePath:  in.SourcePath,
		Title:       title,
		Meta:        in.Meta,
		ContentHash: newHash,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert doc: %w", err)
	}
	res.DocumentID = doc.ID

	// Chunk + insert (no embedding yet — insert-first).
	chunks := i.chunker.Chunk(body)
	if len(chunks) == 0 {
		res.CompletedAt = time.Now().UTC()
		return res, nil
	}
	chunkIDs := make([]int64, 0, len(chunks))
	texts := make([]string, 0, len(chunks))
	for _, c := range chunks {
		stored, err := i.users.UpsertCorpusChunk(ctx, users.CorpusChunk{
			DocumentID: doc.ID,
			ChunkIndex: c.Index,
			Text:       c.Text,
			TokenCount: c.TokenCount,
		})
		if err != nil {
			return nil, fmt.Errorf("upsert chunk %d: %w", c.Index, err)
		}
		chunkIDs = append(chunkIDs, stored.ID)
		texts = append(texts, c.Text)
		res.ChunksInserted++
	}

	// Embed pass. Sidecar Embed accepts a batch (max 64 per proto);
	// split into batches so an oversized document still lands.
	const batchSize = 64
	for start := 0; start < len(texts); start += batchSize {
		end := min(start+batchSize, len(texts))
		vectors, model, err := i.sidecar.Embed(ctx, texts[start:end])
		if err != nil {
			i.log.Warn("embed batch failed",
				slog.Int64("document_id", doc.ID),
				slog.Int("batch_start", start),
				slog.String("error", err.Error()),
			)
			// Leave the already-inserted chunks with nil embedding;
			// a follow-up sweep can retry without re-chunking.
			continue
		}
		res.EmbedderModel = model
		for j, emb := range vectors {
			if err := i.users.SetCorpusChunkEmbedding(ctx, chunkIDs[start+j], emb); err != nil {
				i.log.Warn("set embedding failed",
					slog.Int64("chunk_id", chunkIDs[start+j]),
					slog.String("error", err.Error()),
				)
				continue
			}
			res.ChunksEmbedded++
		}
	}

	res.CompletedAt = time.Now().UTC()
	i.log.Info("ingest done",
		slog.String("source_kind", in.SourceKind),
		slog.String("source_path", in.SourcePath),
		slog.Int64("document_id", doc.ID),
		slog.Int("chunks_inserted", res.ChunksInserted),
		slog.Int("chunks_embedded", res.ChunksEmbedded),
		slog.String("chunker", res.ChunkerName),
		slog.String("embedder", res.EmbedderModel),
	)
	return res, nil
}

// firstNonEmptyLine returns the first non-blank line of text, with
// leading markdown heading markers stripped. Used as a fallback
// title.
func firstNonEmptyLine(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return strings.TrimSpace(strings.TrimLeft(line, "# "))
	}
	return ""
}
