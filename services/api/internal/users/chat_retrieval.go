package users

import (
	"context"
	"fmt"

	"github.com/reh3376/career-site/services/api/internal/corpusscope"
)

// SearchCorpusForChat is retrieval for Ask Roger.
//
// Deliberately a separate entry point from SearchCorpus rather than a
// flag on it. The two answer different questions and have different
// rules, and a shared function with a boolean is how the JD reviewer
// would eventually start retrieving documents the owner excluded from
// the assistant, or the assistant start retrieving résumé variants
// meant only for scoring.
//
// Two gates, on two different axes:
//
//   - chatbot_include (FR-CHAT-18) decides whether the assistant may
//     see the document at all.
//   - visibility decides whether a source may be quoted, linked or
//     named. Private material may inform an answer and must never be
//     cited, which is a locked decision and is enforced at the citation
//     boundary rather than here, because an answer that silently
//     ignored the owner's own records would be worse than one that
//     cannot show its working.
//
// The scope rides the context and defaults to public, so a caller that
// forgets to widen it retrieves too little rather than too much.
func (r *Repo) SearchCorpusForChat(
	ctx context.Context, embedding []float32, topK int,
) ([]CorpusHit, error) {
	if len(embedding) == 0 {
		return nil, fmt.Errorf("empty embedding")
	}
	// Bounded lower than the JD path's 200. Chat pays for context in
	// first-token latency, measured at roughly 32 prompt tokens per
	// second on the production box, so twenty chunks would be a minute
	// before a word appears. The caller decides how many to *show* the
	// model; this is the ceiling on what it may consider.
	if topK <= 0 || topK > 24 {
		topK = 8
	}

	const q = `
    SELECT c.id, c.document_id, c.chunk_index, c.text,
           COALESCE(c.token_count, 0),
           d.title, d.source_kind, d.source_path, d.visibility,
           1 - (c.embedding <=> $1) AS similarity
    FROM corpus_chunks c
    JOIN corpus_documents d ON d.id = c.document_id
    WHERE c.embedding IS NOT NULL
      AND d.chatbot_include
      AND ($3 = true OR d.visibility = 'public')
    ORDER BY c.embedding <=> $1
    LIMIT $2
  `
	rows, err := r.pool.Query(ctx, q,
		vectorLiteral(embedding), topK, corpusscope.AllowsPrivate(ctx))
	if err != nil {
		return nil, fmt.Errorf("search corpus for chat: %w", err)
	}
	defer rows.Close()

	var out []CorpusHit
	for rows.Next() {
		var h CorpusHit
		if err := rows.Scan(
			&h.Chunk.ID, &h.Chunk.DocumentID, &h.Chunk.ChunkIndex, &h.Chunk.Text,
			&h.Chunk.TokenCount,
			&h.Title, &h.SourceKind, &h.SourcePath, &h.Visibility,
			&h.Similarity,
		); err != nil {
			return nil, fmt.Errorf("scan corpus hit: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// Citable reports whether a hit may be shown as a source.
//
// The rule is the locked one: private material may inform an answer but
// must never be quoted at length, linked, or named as a source
// (FR-CHAT-04). A caller building citations filters on this; a caller
// building the model's context does not.
func Citable(h CorpusHit) bool { return h.Visibility == "public" }
