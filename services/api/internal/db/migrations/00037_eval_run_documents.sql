-- +goose Up
-- What the corpus held when an evaluation started.
--
-- A run already records corpus_fingerprint, which answers "was this the
-- same corpus as last time" and nothing else. It cannot answer "which
-- documents did this run actually read", and that question is the one
-- that keeps coming up: run 11 lost a met verdict on Blue Origin
-- because twenty-seven new chunks displaced the four that had been
-- supporting it, and reconstructing which documents existed at the time
-- meant reading git history for the corpus manifest.
--
-- The corpus is mutable and has no delete, so a document ingested after
-- a run makes that run unreadable in hindsight. This captures the list
-- at run start, denormalised on purpose: title and source_kind are
-- copied rather than joined, so a later re-index that changes a
-- document does not rewrite the history of a run that read the old one.
CREATE TABLE eval_run_documents (
    eval_run_id  bigint      NOT NULL REFERENCES eval_runs (id) ON DELETE CASCADE,
    document_id  bigint      NOT NULL,
    title        text        NOT NULL DEFAULT '',
    source_kind  text        NOT NULL DEFAULT '',
    visibility   text        NOT NULL DEFAULT '',
    chunk_count  integer     NOT NULL DEFAULT 0,
    PRIMARY KEY (eval_run_id, document_id)
);

CREATE INDEX idx_eval_run_documents_run ON eval_run_documents (eval_run_id);

-- +goose Down
DROP TABLE IF EXISTS eval_run_documents;
