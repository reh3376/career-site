-- +goose Up
-- +goose StatementBegin

-- Record how much of the corpus a run was allowed to see.
--
-- jd_runs already records the model, the host, the context size, the
-- prompt fingerprints and a fingerprint of the corpus itself, so that a
-- change in behaviour can be attributed rather than guessed at. It did
-- not record the retrieval scope, and on 2026-09-25 that cost an hour.
--
-- The same submission was scored twice, half a day apart. Attempt one:
-- thirteen requirements met, 0.929. Attempt two: eight met, 0.571. Same
-- text, same corpus fingerprint, same model, same prompts, no failed
-- calls. It looked like the pipeline was non-deterministic, which would
-- have contradicted a claim published on the public site.
--
-- The cause was that the rescore path passed a bare context, so the
-- scope defaulted to public and the run searched 52 chunks instead of
-- 241. Every recorded field was identical because the one field that
-- differed was not recorded.
--
-- A run that cannot say what it was allowed to read cannot be compared
-- with another run.
ALTER TABLE jd_runs ADD COLUMN retrieval_scope text NOT NULL DEFAULT '';

COMMENT ON COLUMN jd_runs.retrieval_scope IS
  'What the run was allowed to retrieve: "all" for the owner''s own runs and evaluations, "public" for a member submission. Empty on rows written before this was recorded.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE jd_runs DROP COLUMN IF EXISTS retrieval_scope;
-- +goose StatementEnd
