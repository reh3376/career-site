-- +goose Up
-- +goose StatementBegin

-- Owner-editable settings that used to be constants or env. One row
-- per key, JSON value, who changed it and when. First use: the JD fit
-- bands (very strong / strong / possible / weak thresholds), edited
-- from /admin/jd.
CREATE TABLE app_settings (
  key        text PRIMARY KEY,
  value      jsonb NOT NULL,
  updated_by bigint REFERENCES users(id) ON DELETE SET NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS app_settings;

-- +goose StatementEnd
