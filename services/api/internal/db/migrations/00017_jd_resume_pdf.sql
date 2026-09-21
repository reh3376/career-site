-- +goose Up
-- +goose StatementBegin

-- The rendered, owner-password-locked PDF. Kept on the row rather than
-- in object storage for now: one résumé is ~100 KB, submissions are
-- rare, and this avoids wiring MinIO before content management needs
-- it. generated_resume_url (existing column) points at the api route
-- that streams it, gated by the submission's result token.
ALTER TABLE jd_submissions
  ADD COLUMN resume_pdf       bytea,
  ADD COLUMN resume_pdf_pages integer;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE jd_submissions
  DROP COLUMN resume_pdf,
  DROP COLUMN resume_pdf_pages;

-- +goose StatementEnd
