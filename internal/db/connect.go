package db

import (
	"fmt"
	"os"
	"strconv"

	"github.com/jmoiron/sqlx"
)

// Default pool sizes for email-service entrypoints (api + worker).
//
// Previously both binaries called sqlx.Connect("pgx", dsn) and stopped there,
// which left Postgres pool settings at Go's defaults (MaxOpenConns=0 = unlimited).
// Per-replica that's a silent unbounded pool; multiplied across pods + the
// services PgBouncer cap, it's an unbounded connection budget.
//
// 10 open / 2 idle covers email-service's workload (low-volume API writes +
// async worker queue draining) with headroom. Tunable in prod via the env
// vars below without a rebuild.
const (
	defaultMaxOpenConns = 10
	defaultMaxIdleConns = 2
)

// Connect opens a Postgres connection via sqlx and applies bounded pool
// sizing. The api-server and worker both go through this helper so the
// connection budget can't drift between entrypoints.
func Connect(databaseURL string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("sqlx.Connect: %w", err)
	}
	db.SetMaxOpenConns(envInt("DB_MAX_OPEN_CONNS", defaultMaxOpenConns))
	db.SetMaxIdleConns(envInt("DB_MAX_IDLE_CONNS", defaultMaxIdleConns))
	return db, nil
}

func envInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
