package users

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CorpusDocument mirrors corpus_documents. content_hash and meta are
// exposed so ingest jobs can skip re-embedding unchanged sources.
type CorpusDocument struct {
	ID          int64
	SourceKind  string
	SourcePath  string
	Title       string
	MIME        string
	Meta        []byte // raw jsonb; caller owns parsing
	ContentHash []byte
	IngestedAt  time.Time
	UpdatedAt   time.Time
}

// CorpusChunk mirrors corpus_chunks. Embedding is nil when the
// chunk was inserted but not yet embedded (insert-first, embed-later
// ingest pattern).
type CorpusChunk struct {
	ID         int64
	DocumentID int64
	ChunkIndex int32
	Text       string
	Embedding  []float32 // nil = not embedded yet
	TokenCount int32
	Meta       []byte
	CreatedAt  time.Time
}

// CorpusHit is a retrieval result: one chunk with the source's title
// + a cosine similarity score in [0, 1] (higher = closer).
type CorpusHit struct {
	Chunk      CorpusChunk
	Title      string
	SourceKind string
	SourcePath string
	Similarity float32
}

// UpsertCorpusDocument creates or replaces a document by (source_kind,
// source_path). Returns the stored row.
func (r *Repo) UpsertCorpusDocument(
	ctx context.Context, in CorpusDocument,
) (*CorpusDocument, error) {
	if in.MIME == "" {
		in.MIME = "text/markdown"
	}
	if in.Meta == nil {
		in.Meta = []byte("{}")
	}
	const q = `
    INSERT INTO corpus_documents
      (source_kind, source_path, title, mime, meta, content_hash)
    VALUES ($1, $2, $3, $4, $5::jsonb, $6)
    ON CONFLICT (source_kind, source_path) DO UPDATE
      SET title        = EXCLUDED.title,
          mime         = EXCLUDED.mime,
          meta         = EXCLUDED.meta,
          content_hash = EXCLUDED.content_hash,
          updated_at   = now()
    RETURNING id, ingested_at, updated_at
  `
	out := in
	err := r.pool.QueryRow(ctx, q,
		in.SourceKind, in.SourcePath, in.Title, in.MIME, string(in.Meta), in.ContentHash,
	).Scan(&out.ID, &out.IngestedAt, &out.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert corpus doc: %w", err)
	}
	return &out, nil
}

// UpsertCorpusChunk stores one chunk. Embedding may be nil; a later
// pass fills it via SetCorpusChunkEmbedding. Idempotent on
// (document_id, chunk_index).
func (r *Repo) UpsertCorpusChunk(
	ctx context.Context, in CorpusChunk,
) (*CorpusChunk, error) {
	if in.Meta == nil {
		in.Meta = []byte("{}")
	}
	const q = `
    INSERT INTO corpus_chunks
      (document_id, chunk_index, text, embedding, token_count, meta)
    VALUES ($1, $2, $3, $4, $5, $6::jsonb)
    ON CONFLICT (document_id, chunk_index) DO UPDATE
      SET text        = EXCLUDED.text,
          embedding   = COALESCE(EXCLUDED.embedding, corpus_chunks.embedding),
          token_count = EXCLUDED.token_count,
          meta        = EXCLUDED.meta
    RETURNING id, created_at
  `
	var embArg any
	if in.Embedding != nil {
		embArg = vectorLiteral(in.Embedding)
	}
	out := in
	err := r.pool.QueryRow(ctx, q,
		in.DocumentID, in.ChunkIndex, in.Text, embArg, in.TokenCount, string(in.Meta),
	).Scan(&out.ID, &out.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert corpus chunk: %w", err)
	}
	return &out, nil
}

// SetCorpusChunkEmbedding fills the embedding for one chunk. Used
// by the embed pass in an insert-first, embed-later ingest.
func (r *Repo) SetCorpusChunkEmbedding(
	ctx context.Context, chunkID int64, embedding []float32,
) error {
	if len(embedding) == 0 {
		return fmt.Errorf("empty embedding")
	}
	const q = `UPDATE corpus_chunks SET embedding = $2 WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, chunkID, vectorLiteral(embedding))
	if err != nil {
		return fmt.Errorf("set chunk embedding: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SearchCorpus returns the top-k chunks nearest to the query
// embedding by cosine distance. Similarity in the result is
// (1 - distance) so callers see a "higher = closer" number they can
// threshold against directly (e.g. score ≥ 0.65 gates the JD flow).
//
// The HNSW index on corpus_chunks.embedding makes this fast even
// as the corpus grows; the query below is what pgvector's docs
// call the "canonical KNN" pattern.
func (r *Repo) SearchCorpus(
	ctx context.Context, embedding []float32, topK int,
) ([]CorpusHit, error) {
	if len(embedding) == 0 {
		return nil, fmt.Errorf("empty embedding")
	}
	if topK <= 0 || topK > 200 {
		topK = 20
	}
	const q = `
    SELECT c.id, c.document_id, c.chunk_index, c.text,
           COALESCE(c.token_count, 0),
           d.title, d.source_kind, d.source_path,
           1 - (c.embedding <=> $1) AS similarity
    FROM corpus_chunks c
    JOIN corpus_documents d ON d.id = c.document_id
    WHERE c.embedding IS NOT NULL
    ORDER BY c.embedding <=> $1
    LIMIT $2
  `
	rows, err := r.pool.Query(ctx, q, vectorLiteral(embedding), topK)
	if err != nil {
		return nil, fmt.Errorf("search corpus: %w", err)
	}
	defer rows.Close()

	var out []CorpusHit
	for rows.Next() {
		var h CorpusHit
		if err := rows.Scan(
			&h.Chunk.ID, &h.Chunk.DocumentID, &h.Chunk.ChunkIndex, &h.Chunk.Text,
			&h.Chunk.TokenCount,
			&h.Title, &h.SourceKind, &h.SourcePath,
			&h.Similarity,
		); err != nil {
			return nil, fmt.Errorf("scan corpus hit: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// HashCorpusContent returns SHA-256 of the source text — used by
// ingest jobs to decide whether a document needs re-embedding.
func HashCorpusContent(text string) []byte {
	sum := sha256.Sum256([]byte(text))
	return sum[:]
}

// vectorLiteral formats a []float32 as pgvector's text literal:
// `[0.1,0.2,0.3]`. pgx's parameter binding accepts this as-is for
// the vector type; no third-party pgvector-go dependency needed.
func vectorLiteral(v []float32) string {
	var b strings.Builder
	b.Grow(len(v) * 8)
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		// %g is exact enough for retrieval; pgvector rounds internally.
		b.WriteString(strconv.FormatFloat(float64(f), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}
