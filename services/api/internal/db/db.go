// Package db owns the Postgres connection pool, migration runner, and health
// check. Migrations are embedded in the binary and applied on start so the
// process is self-installing on a fresh database.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver for database/sql (used by goose)
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Pool struct {
	*pgxpool.Pool
}

// Open creates a connection pool with sensible defaults. The DSN follows
// pgx's URL form: `postgres://user:pass@host:port/dbname?sslmode=disable`.
func Open(ctx context.Context, dsn string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	cfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("new pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Pool{Pool: pool}, nil
}

// Migrate applies every pending Up migration embedded in the binary. The
// process is idempotent; goose tracks state in the `goose_db_version` table.
func Migrate(ctx context.Context, dsn string) error {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open for migrate: %w", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	if err := goose.UpContext(ctx, sqlDB, "migrations"); err != nil {
		return fmt.Errorf("up: %w", err)
	}
	return nil
}

// Ping is used by /api/readyz.
func (p *Pool) Ping(ctx context.Context) error {
	return p.Pool.Ping(ctx)
}

// SetReadonlyRolePassword flips the migration-created role
// `career_admin_readonly` from NOLOGIN to LOGIN + PASSWORD, so the
// readonly pool can actually connect. Idempotent — Postgres accepts
// ALTER ROLE on the same password without complaint. Called at boot
// from the write pool (the readonly role has no CREATEROLE, so it
// cannot alter itself).
//
// The password is interpolated with pgx's SQL quoting via a parameter
// only for literal-in-string uses; ALTER ROLE requires an inline
// literal, so we escape single-quotes by doubling them (the standard
// Postgres string-literal escape). The caller passes a real random
// password from a secrets manager, not user input, so the escaping is
// belt-and-suspenders — but keeping it there means a future caller
// with a stray quote in the value cannot accidentally break the boot.
func (p *Pool) SetReadonlyRolePassword(ctx context.Context, password string) error {
	if password == "" {
		return fmt.Errorf("empty password")
	}
	escaped := strings.ReplaceAll(password, "'", "''")
	stmt := fmt.Sprintf(
		"ALTER ROLE career_admin_readonly WITH LOGIN PASSWORD '%s'",
		escaped,
	)
	if _, err := p.Pool.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("alter readonly role: %w", err)
	}
	return nil
}
