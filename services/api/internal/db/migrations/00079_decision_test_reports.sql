-- +goose Up
-- +goose StatementBegin

-- The decision test's individual results report (FSD §5.14 FR-DT-21 to
-- 24). Every finished run gets a report in two copies: one that can be
-- sent to the participant, and an owner's copy that also prints the
-- questions with the answers given and the correct answers.
--
-- A report is a snapshot. Its comparison group is whoever had finished
-- when it was built, so building it again later moves the ranking. Each
-- build is therefore a new version and the earlier ones are kept: the
-- copy a participant was sent has to stay reproducible after the group
-- has grown. Nothing here is ever updated in place except the fields
-- that record a send.
CREATE TABLE dt_reports (
  id                bigserial   PRIMARY KEY,
  session_id        bigint      NOT NULL REFERENCES dt_sessions(id) ON DELETE CASCADE,
  version           integer     NOT NULL,
  -- Which wording and layout wrote it, and which renderer drew it.
  template          text        NOT NULL,
  engine            text        NOT NULL DEFAULT '',
  -- Whether it ranks the participant, why not when it does not
  -- (small_group | repeat | unplaced | synthetic), and how many other
  -- first-time participants there were to compare with.
  compared          boolean     NOT NULL,
  no_ranking_reason text        NOT NULL DEFAULT '',
  others            integer     NOT NULL DEFAULT 0,
  -- The handful of figures the report states about the run: score,
  -- rank, percentile, confidence. Nothing about any question.
  summary           jsonb       NOT NULL DEFAULT '{}'::jsonb,
  participant_pdf   bytea       NOT NULL,
  owner_pdf         bytea       NOT NULL,
  participant_pages integer     NOT NULL DEFAULT 0,
  owner_pages       integer     NOT NULL DEFAULT 0,
  -- system for the build made when the run finished, admin:<user id>
  -- for one asked for from the console.
  triggered_by      text        NOT NULL DEFAULT 'system',
  created_at        timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT dt_reports_version_uq UNIQUE (session_id, version),
  CONSTRAINT dt_reports_version_ck CHECK (version >= 1)
);

COMMENT ON TABLE dt_reports IS
  'One row per build of a run''s results report, both PDFs on the row. Versions are kept, never replaced: a report is a snapshot of the comparison group at the time it was built. owner_pdf prints the questions and their answers and is admin-only (FSD FR-DT-21).';
COMMENT ON COLUMN dt_reports.owner_pdf IS
  'The owner''s copy. Contains the answer key for the questions this run drew; never sent to a participant (FSD FR-DT-04).';

-- Kept on the row rather than in object storage, as the résumé PDF is
-- (00017): two files of about half a megabyte per run, a handful of
-- runs a week. TOAST stores them out of line, so nothing that reads the
-- other columns pays for them.

-- Building a report can fail: the renderer is a separate service and
-- may be restarting. This is the retry ledger, one row per run that has
-- been tried. A run with a report has done_at set; a run without one is
-- tried again until attempts runs out, after which the participant is
-- sent their results without the PDF rather than nothing.
CREATE TABLE dt_report_jobs (
  session_id      bigint      PRIMARY KEY REFERENCES dt_sessions(id) ON DELETE CASCADE,
  attempts        integer     NOT NULL DEFAULT 0,
  -- A failure retrying cannot fix, such as a run closed before its last
  -- answer was stored. Stops the sweep from trying again.
  permanent       boolean     NOT NULL DEFAULT false,
  last_error      text        NOT NULL DEFAULT '',
  last_attempt_at timestamptz,
  done_at         timestamptz
);

COMMENT ON TABLE dt_report_jobs IS
  'Retry ledger for building decision test reports. Absent row: never tried. done_at set: a report exists.';

-- Every results email, automatic or sent from the console.
--
-- The automatic one is claimed here before it is sent, and the partial
-- unique index is what makes it once per run. The run screen closes a
-- run and then calls FinishSession a second time with the debrief
-- answer, and until this migration each call sent the email: every
-- participant who asked for results from 2026-10-07 received it twice,
-- seconds apart. A claim that cannot be made twice is the fix, rather
-- than a flag somebody has to remember to check.
CREATE TABLE dt_result_emails (
  id             bigserial   PRIMARY KEY,
  session_id     bigint      NOT NULL REFERENCES dt_sessions(id) ON DELETE CASCADE,
  -- system: the one send a finished run triggers. admin: sent from the
  -- console, as often as the owner chooses.
  source         text        NOT NULL,
  sent_by        bigint      REFERENCES users(id) ON DELETE SET NULL,
  -- claimed: about to be sent. sent, failed: how that ended.
  -- not_wanted: the participant did not ask, so nothing was sent.
  -- settled: the run finished before reports existed and its email, if
  -- it was owed one, went out at the time.
  outcome        text        NOT NULL,
  -- Which build of the report was attached. NULL when the email went
  -- without one.
  report_version integer,
  error          text        NOT NULL DEFAULT '',
  created_at     timestamptz NOT NULL DEFAULT now(),
  sent_at        timestamptz,

  CONSTRAINT dt_result_emails_source_ck  CHECK (source IN ('system','admin')),
  CONSTRAINT dt_result_emails_outcome_ck CHECK (outcome IN ('claimed','sent','failed','not_wanted','settled'))
);

CREATE UNIQUE INDEX dt_result_emails_system_uq ON dt_result_emails (session_id) WHERE source = 'system';
CREATE INDEX dt_result_emails_session_idx ON dt_result_emails (session_id, id DESC);

COMMENT ON TABLE dt_result_emails IS
  'Every decision test results email. At most one system row per run (partial unique index), which is what makes the automatic send happen once. No address is stored here: who it went to is the participant row, and what was delivered is notification_deliveries.';

-- Runs that finished before this migration are settled: whatever email
-- they were owed went out when they finished. Their reports are built
-- by the sweep like anyone's, and sent only if the owner chooses to
-- from the console. Without these rows the first sweep after the deploy
-- would email every past participant who had ever asked for results.
INSERT INTO dt_result_emails (session_id, source, outcome)
SELECT id, 'system', 'settled' FROM dt_sessions WHERE status = 'completed';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS dt_result_emails;
DROP TABLE IF EXISTS dt_report_jobs;
DROP TABLE IF EXISTS dt_reports;

-- +goose StatementEnd
