-- +goose Up
-- +goose StatementBegin

-- Which documents, and whether they were the same documents.
--
-- eval_run_documents (00037) captures what the corpus held when a run
-- started: id, title, kind, visibility, chunk count. That answers
-- "which documents did this run read", which was unanswerable once the
-- corpus moved on.
--
-- It does not answer "and were they the same". Identity is captured,
-- content is not. A document re-indexed with edited text keeps its id,
-- and if the edit does not move the chunk count the manifest is
-- byte-identical to the one before it. The run then looks reproducible
-- when it is not, which is worse than having no manifest, because a
-- manifest invites confidence.
--
-- That happened the same day it shipped. The ontology article had 362
-- asterisk artifacts removed on 2026-09-29 and came back with exactly
-- 26 chunks, the number it had before. Nothing in the manifest would
-- have shown the text had changed.
--
-- corpus_documents already carries content_hash and already updates it
-- on re-index, so nothing new has to be computed; it simply was not
-- being copied. Copied rather than joined, for the same reason the
-- title is: a later re-index must not rewrite the history of a run
-- that read the earlier version.
ALTER TABLE eval_run_documents ADD COLUMN IF NOT EXISTS content_hash bytea;

COMMENT ON COLUMN eval_run_documents.content_hash IS
  'The document''s content_hash when the run started. Null for rows captured before 00040, which means unknown rather than unchanged.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE eval_run_documents DROP COLUMN IF EXISTS content_hash;
-- +goose StatementEnd
