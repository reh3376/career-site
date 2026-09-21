-- +goose Up
-- +goose StatementBegin

-- Visibility gate for corpus documents. The corpus is deliberately
-- broader than what the public site shows: internal worksheets,
-- post-mortems, interview prep and client-adjacent notes feed Ask
-- Roger's judgement (soft-skills evidence) but must never surface
-- verbatim on a public page or in a generated résumé.
--
--   public       committed content under apps/web/content; may be
--                quoted, linked, and cited by title.
--   corpus_only  private mount; retrieval may use it to inform an
--                answer, but downstream prompts must not quote it or
--                name the source. Default for everything the private
--                walker ingests (fail-closed).

ALTER TABLE corpus_documents
  ADD COLUMN visibility text NOT NULL DEFAULT 'public'
    CHECK (visibility IN ('public', 'corpus_only'));

COMMENT ON COLUMN corpus_documents.visibility IS
  'public (citable on the site) or corpus_only (informs answers, never quoted or named).';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE corpus_documents DROP COLUMN visibility;

-- +goose StatementEnd
