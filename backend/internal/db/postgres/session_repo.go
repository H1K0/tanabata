package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

// sessionRow matches the columns stored in activity.sessions.
// IsCurrent is a service-layer concern and is not stored in the database.
type sessionRow struct {
	ID           int        `db:"id"`
	TokenHash    string     `db:"token_hash"`
	UserID       int16      `db:"user_id"`
	UserAgent    string     `db:"user_agent"`
	StartedAt    time.Time  `db:"started_at"`
	ExpiresAt    *time.Time `db:"expires_at"`
	LastActivity time.Time  `db:"last_activity"`
}

// sessionRowWithTotal extends sessionRow with a window-function count for ListByUser.
type sessionRowWithTotal struct {
	sessionRow
	Total int `db:"total"`
}

func toSession(r sessionRow) domain.Session {
	return domain.Session{
		ID:           r.ID,
		TokenHash:    r.TokenHash,
		UserID:       r.UserID,
		UserAgent:    r.UserAgent,
		StartedAt:    r.StartedAt,
		ExpiresAt:    r.ExpiresAt,
		LastActivity: r.LastActivity,
	}
}

// SessionRepo implements port.SessionRepo using PostgreSQL.
type SessionRepo struct {
	pool *pgxpool.Pool
}

// NewSessionRepo creates a SessionRepo backed by pool.
func NewSessionRepo(pool *pgxpool.Pool) *SessionRepo {
	return &SessionRepo{pool: pool}
}

var _ port.SessionRepo = (*SessionRepo)(nil)

func (r *SessionRepo) Create(ctx context.Context, s *domain.Session) (*domain.Session, error) {
	const sql = `
		INSERT INTO activity.sessions (token_hash, user_id, user_agent, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, token_hash, user_id, user_agent, started_at, expires_at, last_activity`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sql, s.TokenHash, s.UserID, s.UserAgent, s.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("SessionRepo.Create: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[sessionRow])
	if err != nil {
		return nil, fmt.Errorf("SessionRepo.Create scan: %w", err)
	}
	created := toSession(row)
	return &created, nil
}

func (r *SessionRepo) GetByTokenHash(ctx context.Context, hash string) (*domain.Session, error) {
	const sql = `
		SELECT id, token_hash, user_id, user_agent, started_at, expires_at, last_activity
		FROM activity.sessions
		WHERE token_hash = $1 AND is_active = true`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sql, hash)
	if err != nil {
		return nil, fmt.Errorf("SessionRepo.GetByTokenHash: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[sessionRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("SessionRepo.GetByTokenHash scan: %w", err)
	}
	s := toSession(row)
	return &s, nil
}

func (r *SessionRepo) ListByUser(ctx context.Context, userID int16) (*domain.SessionList, error) {
	const sql = `
		SELECT id, token_hash, user_id, user_agent, started_at, expires_at, last_activity,
		       COUNT(*) OVER() AS total
		FROM activity.sessions
		WHERE user_id = $1 AND is_active = true
		ORDER BY started_at DESC`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sql, userID)
	if err != nil {
		return nil, fmt.Errorf("SessionRepo.ListByUser: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[sessionRowWithTotal])
	if err != nil {
		return nil, fmt.Errorf("SessionRepo.ListByUser scan: %w", err)
	}

	list := &domain.SessionList{}
	if len(collected) > 0 {
		list.Total = collected[0].Total
	}
	list.Items = make([]domain.Session, len(collected))
	for i, row := range collected {
		list.Items[i] = toSession(row.sessionRow)
	}
	return list, nil
}

func (r *SessionRepo) UpdateLastActivity(ctx context.Context, id int, t time.Time) error {
	const sql = `UPDATE activity.sessions SET last_activity = $2 WHERE id = $1`
	q := connOrTx(ctx, r.pool)
	tag, err := q.Exec(ctx, sql, id, t)
	if err != nil {
		return fmt.Errorf("SessionRepo.UpdateLastActivity: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *SessionRepo) Delete(ctx context.Context, id int) error {
	const sql = `UPDATE activity.sessions SET is_active = false WHERE id = $1 AND is_active = true`
	q := connOrTx(ctx, r.pool)
	tag, err := q.Exec(ctx, sql, id)
	if err != nil {
		return fmt.Errorf("SessionRepo.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *SessionRepo) DeleteByUserID(ctx context.Context, userID int16) error {
	const sql = `UPDATE activity.sessions SET is_active = false WHERE user_id = $1 AND is_active = true`
	q := connOrTx(ctx, r.pool)
	_, err := q.Exec(ctx, sql, userID)
	if err != nil {
		return fmt.Errorf("SessionRepo.DeleteByUserID: %w", err)
	}
	return nil
}
