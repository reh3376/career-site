// Package adminquery serves the read-only SQL surface behind
// /admin/db. Every call goes through this package so the safety
// rails — SELECT-only, statement timeout, row cap — are in exactly
// one place and the handler is a thin wrapper.
//
// The MVP allows only SELECT statements. Future hardening (write
// mode with an explicit confirm, distinct DB role, per-admin audit
// table) is planned; see project_admin_console_backlog.md.
package adminquery

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/reh3376/career-site/services/api/internal/db"
)

// Sensible caps for the MVP. The proto's validation clamps the
// caller's requested timeout to [100, 10000]; the row-count cap is
// server-side only so an admin can't bypass it.
const (
	DefaultTimeoutMs = 3000
	MinTimeoutMs     = 100
	MaxTimeoutMs     = 10000
	RowCap           = 500
	MaxColumns       = 64
)

// Table + column metadata returned to the schema panel.
type Column struct {
	Name     string
	DataType string
	Nullable bool
}

type Table struct {
	Name           string
	Columns        []Column
	ApproxRowCount int64
}

// QueryResult is a serialization-friendly shape the handler maps
// straight into the proto response.
type QueryResult struct {
	Columns     []string
	ColumnTypes []string
	Rows        [][]any // one entry per row; nil for NULL cells
	Truncated   bool
	ElapsedMs   int32
}

// selectRe matches a leading SELECT / WITH (CTE that yields SELECT)
// keyword, ignoring leading whitespace and any leading `--` comment
// lines. Case-insensitive; `(?s)` so `.` crosses newlines in the
// comment-stripping pass. The regex is deliberately conservative —
// it's the WHERE clause of a safety rail, not full SQL parsing.
var selectRe = regexp.MustCompile(`(?is)^\s*(--[^\n]*\n\s*)*(select|with)\b`)

// forbiddenRe rejects anything that looks like a write, DDL, session
// change, or statement chain. Runs against the query with comments
// stripped so `-- delete from users` in a comment doesn't trip it.
// This is layered defense: even if we already require SELECT/WITH at
// the front, this catches ";DELETE ..." tacked on.
var forbiddenRe = regexp.MustCompile(`(?is)\b(insert|update|delete|drop|truncate|alter|create|grant|revoke|comment|copy|call|do|set|reset|listen|notify|unlisten|vacuum|analyze|reindex|checkpoint|discard|lock)\b`)

// blockCommentRe is used to strip /* ... */ comments before the
// forbidden-token pass.
var blockCommentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)

// ListTables reads the public schema out of information_schema plus
// pg_class.reltuples for approximate row counts. Ordered by name.
func ListTables(ctx context.Context, pool *db.Pool) ([]Table, error) {
	const q = `
    SELECT
      c.table_name,
      c.column_name,
      c.data_type,
      c.is_nullable,
      COALESCE(cls.reltuples, 0)::bigint AS approx_rows
    FROM information_schema.columns c
    JOIN pg_class cls
      ON cls.relname = c.table_name
     AND cls.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = 'public')
    WHERE c.table_schema = 'public'
    ORDER BY c.table_name, c.ordinal_position
  `
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	byName := map[string]*Table{}
	order := []string{}
	for rows.Next() {
		var tableName, columnName, dataType, isNullable string
		var approxRows int64
		if err := rows.Scan(&tableName, &columnName, &dataType, &isNullable, &approxRows); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		t, ok := byName[tableName]
		if !ok {
			t = &Table{Name: tableName, ApproxRowCount: approxRows}
			byName[tableName] = t
			order = append(order, tableName)
		}
		t.Columns = append(t.Columns, Column{
			Name:     columnName,
			DataType: dataType,
			Nullable: isNullable == "YES",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate: %w", err)
	}
	out := make([]Table, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out, nil
}

// Run executes a caller-supplied SELECT (or CTE resolving to SELECT)
// against the pool. Enforces the safety rails and returns a
// serialization-friendly result.
func Run(ctx context.Context, pool *db.Pool, sql string, timeoutMs int32) (*QueryResult, error) {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return nil, errors.New("empty query")
	}
	// Strip block comments before the forbidden-token check so
	// `SELECT 1 /* DELETE FROM users */` isn't rejected. Also strip
	// trailing semicolons + whitespace so a single-statement query
	// with a trailing `;` still passes the "one statement" heuristic.
	stripped := blockCommentRe.ReplaceAllString(trimmed, " ")
	stripped = strings.TrimRight(stripped, "; \t\r\n")
	if !selectRe.MatchString(stripped) {
		return nil, errors.New("only SELECT / WITH queries are allowed")
	}
	if strings.Contains(stripped, ";") {
		return nil, errors.New("only one statement per query")
	}
	if forbiddenRe.MatchString(stripped) {
		return nil, errors.New("query contains a disallowed keyword (INSERT / UPDATE / DELETE / DDL / SET / …)")
	}

	// Clamp timeout, apply as a statement_timeout inside a
	// read-only transaction so a runaway can't chew the pod.
	if timeoutMs <= 0 {
		timeoutMs = DefaultTimeoutMs
	}
	if timeoutMs < MinTimeoutMs {
		timeoutMs = MinTimeoutMs
	}
	if timeoutMs > MaxTimeoutMs {
		timeoutMs = MaxTimeoutMs
	}

	txCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs+500)*time.Millisecond)
	defer cancel()

	tx, err := pool.BeginTx(txCtx, pgx.TxOptions{
		AccessMode: pgx.ReadOnly,
		IsoLevel:   pgx.ReadCommitted,
	})
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(context.Background()) //nolint:errcheck // read-only, rollback always safe

	if _, err := tx.Exec(txCtx, fmt.Sprintf("SET LOCAL statement_timeout = %d", timeoutMs)); err != nil {
		return nil, fmt.Errorf("set timeout: %w", err)
	}

	start := time.Now()
	rows, err := tx.Query(txCtx, sql)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	if len(fields) > MaxColumns {
		return nil, fmt.Errorf("query returned %d columns, cap is %d", len(fields), MaxColumns)
	}
	cols := make([]string, len(fields))
	types := make([]string, len(fields))
	for i, f := range fields {
		cols[i] = string(f.Name)
		// pgx exposes the OID; we render it as a decimal for now.
		// A follow-up can map common OIDs to human names via
		// pg_type but this is enough for the schema panel.
		types[i] = pgTypeName(f.DataTypeOID)
	}

	out := &QueryResult{Columns: cols, ColumnTypes: types}
	for rows.Next() {
		if len(out.Rows) >= RowCap {
			out.Truncated = true
			break
		}
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("row values: %w", err)
		}
		out.Rows = append(out.Rows, values)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate: %w", err)
	}
	out.ElapsedMs = int32(time.Since(start).Milliseconds())
	return out, nil
}

// pgTypeName translates common Postgres type OIDs into short human
// names for the results header. Unknown OIDs render as "oid=<n>" so
// the caller sees something rather than a mystery number.
func pgTypeName(oid uint32) string {
	// Values from pg_type.h — the common ones the app's schema uses.
	switch oid {
	case 16:
		return "bool"
	case 20:
		return "bigint"
	case 21:
		return "smallint"
	case 23:
		return "int"
	case 25:
		return "text"
	case 700:
		return "float4"
	case 701:
		return "float8"
	case 1043:
		return "varchar"
	case 1082:
		return "date"
	case 1114:
		return "timestamp"
	case 1184:
		return "timestamptz"
	case 2950:
		return "uuid"
	case 3802:
		return "jsonb"
	case 114:
		return "json"
	case 17:
		return "bytea"
	default:
		return fmt.Sprintf("oid=%d", oid)
	}
}
