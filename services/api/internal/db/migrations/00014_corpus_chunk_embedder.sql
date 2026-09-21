-- +goose Up
-- +goose StatementBegin

-- Record which embedder produced each chunk's vector. Without this the
-- embed sweep can only find chunks with a NULL embedding; it cannot tell
-- that a vector produced by the stub provider (or an older model) is
-- stale once the sidecar switches to Ollama. The sweep selects
--   embedding IS NULL OR embedder_model IS DISTINCT FROM <current>
-- so flipping the provider and pressing "Embed sweep" re-embeds
-- everything, no manual reset needed.
--
-- Existing rows keep embedder_model = NULL and are therefore treated as
-- stale by the first sweep, which is the correct outcome for anything
-- embedded before this column existed.

ALTER TABLE corpus_chunks
  ADD COLUMN embedder_model text,
  ADD COLUMN embedded_at    timestamptz;

COMMENT ON COLUMN corpus_chunks.embedder_model IS
  'Sidecar embedder name that produced `embedding`, e.g. stub or ollama:nomic-embed-text. NULL = not embedded or pre-tracking.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE corpus_chunks
  DROP COLUMN embedder_model,
  DROP COLUMN embedded_at;

-- +goose StatementEnd
