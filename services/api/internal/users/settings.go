package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// GetSetting reads one app_settings value. ok is false when unset.
func (r *Repo) GetSetting(ctx context.Context, key string) (value json.RawMessage, ok bool, err error) {
	var v []byte
	err = r.pool.QueryRow(ctx, `SELECT value FROM app_settings WHERE key = $1`, key).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get setting %s: %w", key, err)
	}
	return json.RawMessage(v), true, nil
}

// SetSetting writes one app_settings value, recording who changed it.
func (r *Repo) SetSetting(ctx context.Context, key string, value json.RawMessage, updatedBy int64) error {
	_, err := r.pool.Exec(ctx, `
    INSERT INTO app_settings (key, value, updated_by, updated_at)
    VALUES ($1, $2, NULLIF($3, 0), now())
    ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now()`,
		key, []byte(value), updatedBy)
	if err != nil {
		return fmt.Errorf("set setting %s: %w", key, err)
	}
	return nil
}
