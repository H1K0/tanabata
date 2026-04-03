package db

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var connPool *pgxpool.Pool

func InitDB(connString string) error {
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return fmt.Errorf("error while parsing connection string: %w", err)
	}

	poolConfig.MaxConns = 100
	poolConfig.MinConns = 0
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.HealthCheckPeriod = 30 * time.Second

	connPool, err = pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return fmt.Errorf("error while initializing DB connections pool: %w", err)
	}
	return nil
}

func transaction(handler func(context.Context, pgx.Tx) (statusCode int, err error)) (statusCode int, err error) {
	ctx := context.Background()
	tx, err := connPool.Begin(ctx)
	if err != nil {
		statusCode = http.StatusInternalServerError
		return
	}
	statusCode, err = handler(ctx, tx)
	if err != nil {
		tx.Rollback(ctx)
		return
	}
	err = tx.Commit(ctx)
	if err != nil {
		statusCode = http.StatusInternalServerError
	}
	return
}

// Handle database error
func handleDBError(errIn error) (statusCode int, err error) {
	if errIn == nil {
		statusCode = http.StatusOK
		return
	}
	if errors.Is(errIn, pgx.ErrNoRows) {
		err = fmt.Errorf("not found")
		statusCode = http.StatusNotFound
		return
	}
	var pgErr *pgconn.PgError
	if errors.As(errIn, &pgErr) {
		switch pgErr.Code {
		case "22P02", "22007": // Invalid data format
			err = fmt.Errorf("%s", pgErr.Message)
			statusCode = http.StatusBadRequest
			return
		case "23505": // Unique constraint violation
			err = fmt.Errorf("already exists")
			statusCode = http.StatusConflict
			return
		}
	}
	return http.StatusInternalServerError, errIn
}
