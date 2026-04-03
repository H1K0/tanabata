package postgres

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

type Storage struct {
	db *pgxpool.Pool
}

var connPool *pgxpool.Pool

// Initialize new database storage
func New(dbURL string) (*Storage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DB URL: %w", err)
	}
	config.MaxConns = 10
	config.MinConns = 2
	config.HealthCheckPeriod = time.Minute
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	err = db.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}
	return &Storage{db: db}, nil
}

// Close database storage
func (s *Storage) Close() {
	s.db.Close()
}

// Run handler inside transaction
func (s *Storage) transaction(ctx context.Context, handler func(context.Context, pgx.Tx) (statusCode int, err error)) (statusCode int, err error) {
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
func (s *Storage) handleDBError(errIn error) (statusCode int, err error) {
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
