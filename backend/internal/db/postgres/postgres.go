// Package postgres provides the PostgreSQL implementations of the port interfaces.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tanabata/backend/internal/db"
)

// appName tags every connection as application_name, so the backend's sessions
// are identifiable in pg_stat_activity and server logs (and distinguishable from
// e.g. goose migrations or a psql shell).
const appName = "tanabata-backend"

// NewPool creates and validates a *pgxpool.Pool from the given connection URL.
// The pool is ready to use; the caller is responsible for closing it.
func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.ParseConfig: %w", err)
	}
	// Set application_name unless the operator already specified one in the DSN
	// (or via PGAPPNAME), so an explicit override still wins.
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	if cfg.ConnConfig.RuntimeParams["application_name"] == "" {
		cfg.ConnConfig.RuntimeParams["application_name"] = appName
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.NewWithConfig: %w", err)
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

// Object type IDs as seeded in core.object_types (007_seed_data.sql).
const (
	objTypeFile     int16 = 1
	objTypeTag      int16 = 2
	objTypeCategory int16 = 3
	objTypePool     int16 = 4
)

// aclVisibilityCond returns a SQL boolean fragment that is true when the viewer
// may see the row at <alias>.id of the given object type under the
// private-by-default model: the row is public, the viewer created it, or the
// viewer holds an explicit can_view grant. objectTypeID is a trusted constant
// and is inlined; viewerID is bound as $n (referenced twice). Returns the
// fragment, the next free parameter index, and the extended args.
//
// Callers skip this entirely for admins (who bypass ACL).
func aclVisibilityCond(alias string, objectTypeID int16, viewerID int16, n int, args []any) (string, int, []any) {
	cond := fmt.Sprintf(
		"(%[1]s.is_public OR %[1]s.creator_id = $%[2]d OR EXISTS ("+
			"SELECT 1 FROM acl.permissions p "+
			"WHERE p.object_type_id = %[3]d AND p.object_id = %[1]s.id "+
			"AND p.user_id = $%[2]d AND p.can_view))",
		alias, n, objectTypeID)
	return cond, n + 1, append(args, viewerID)
}
