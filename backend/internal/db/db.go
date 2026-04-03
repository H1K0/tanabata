// Package db provides shared helpers used by all database adapters.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// txKey is the context key used to store an active transaction.
type txKey struct{}

// TxFromContext returns the pgx.Tx stored in ctx by the Transactor, along
// with a boolean indicating whether a transaction is active.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}

// ContextWithTx returns a copy of ctx that carries tx.
// Called by the Transactor before invoking the user function.
func ContextWithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// Querier is the common query interface satisfied by both *pgxpool.Pool and
// pgx.Tx, allowing repo helpers to work with either.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// ScanRow executes a single-row query against q and scans the result using
// scan. It wraps pgx.ErrNoRows so callers can detect missing rows without
// importing pgx directly.
func ScanRow[T any](ctx context.Context, q Querier, sql string, args []any, scan func(pgx.Row) (T, error)) (T, error) {
	row := q.QueryRow(ctx, sql, args...)
	val, err := scan(row)
	if err != nil {
		var zero T
		if err == pgx.ErrNoRows {
			return zero, fmt.Errorf("%w", pgx.ErrNoRows)
		}
		return zero, fmt.Errorf("ScanRow: %w", err)
	}
	return val, nil
}

// ClampLimit enforces the [1, max] range on limit, returning def when limit
// is zero or negative.
func ClampLimit(limit, def, max int) int {
	if limit <= 0 {
		return def
	}
	if limit > max {
		return max
	}
	return limit
}

// ClampOffset returns 0 for negative offsets.
func ClampOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}
