package users

import (
	"context"
	"fmt"
	"time"
)

// SavedQuery is one row of admin_saved_queries. Owner-scoped: the
// repo never returns rows for a user who isn't the caller.
type SavedQuery struct {
	ID        int64
	OwnerID   int64
	Name      string
	SQL       string
	CreatedAt time.Time
	UpdatedAt time.Time
}

const savedQueryCols = `id, owner_id, name, sql_body, created_at, updated_at`

// ListSavedQueries returns every saved query owned by ownerID,
// ordered by name (case-insensitive). Small result set; no pagination.
func (r *Repo) ListSavedQueries(ctx context.Context, ownerID int64) ([]SavedQuery, error) {
	const q = `SELECT ` + savedQueryCols + `
    FROM admin_saved_queries
    WHERE owner_id = $1
    ORDER BY LOWER(name)`
	rows, err := r.pool.Query(ctx, q, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list saved queries: %w", err)
	}
	defer rows.Close()
	var out []SavedQuery
	for rows.Next() {
		var s SavedQuery
		if err := rows.Scan(
			&s.ID, &s.OwnerID, &s.Name, &s.SQL, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan saved query: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpsertSavedQuery creates a saved query or replaces one with the
// same (owner, name). Returns the stored row and true when a new
// row was inserted, using the RETURNING xmax=0 trick to distinguish
// insert from update in one round-trip.
func (r *Repo) UpsertSavedQuery(
	ctx context.Context, ownerID int64, name, sql string,
) (*SavedQuery, bool, error) {
	const q = `
    INSERT INTO admin_saved_queries (owner_id, name, sql_body)
    VALUES ($1, $2, $3)
    ON CONFLICT (owner_id, name) DO UPDATE
      SET sql_body   = EXCLUDED.sql_body,
          updated_at = now()
    RETURNING ` + savedQueryCols + `, (xmax = 0) AS inserted
  `
	s := &SavedQuery{}
	var inserted bool
	err := r.pool.QueryRow(ctx, q, ownerID, name, sql).Scan(
		&s.ID, &s.OwnerID, &s.Name, &s.SQL, &s.CreatedAt, &s.UpdatedAt,
		&inserted,
	)
	if err != nil {
		return nil, false, fmt.Errorf("upsert saved query: %w", err)
	}
	return s, inserted, nil
}

// DeleteSavedQuery removes a saved query owned by ownerID. Returns
// ErrNotFound when no row matches — the owner_id filter means an
// admin can never delete another admin's row even by id-guessing.
func (r *Repo) DeleteSavedQuery(ctx context.Context, ownerID, id int64) error {
	const q = `DELETE FROM admin_saved_queries WHERE id = $1 AND owner_id = $2`
	tag, err := r.pool.Exec(ctx, q, id, ownerID)
	if err != nil {
		return fmt.Errorf("delete saved query: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
