// Package postgres provides the PostgreSQL implementations of the port interfaces.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tanabata/backend/internal/db"
)

// NewPool creates and validates a *pgxpool.Pool from the given connection URL.
// The pool is ready to use; the caller is responsible for closing it.
func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	return pool, nil
}

// Transactor implements port.Transactor using a pgxpool.Pool.
type Transactor struct {
	pool *pgxpool.Pool
}

// NewTransactor creates a Transactor backed by pool.
func NewTransactor(pool *pgxpool.Pool) *Transactor {
	return &Transactor{pool: pool}
}

// WithTx begins a transaction, stores it in ctx, and calls fn. If fn returns
// an error the transaction is rolled back; otherwise it is committed.
func (t *Transactor) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := t.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	txCtx := db.ContextWithTx(ctx, tx)

	if err := fn(txCtx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// connOrTx returns the pgx.Tx stored in ctx by WithTx, or the pool itself when
// no transaction is active. The returned value satisfies db.Querier and can be
// used directly for queries and commands.
func connOrTx(ctx context.Context, pool *pgxpool.Pool) db.Querier {
	if tx, ok := db.TxFromContext(ctx); ok {
		return tx
	}
	return pool
}
