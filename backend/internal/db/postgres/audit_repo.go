package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tanabata/backend/internal/db"
	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

// auditRowWithTotal matches the columns returned by the audit log SELECT.
// object_type is nullable (LEFT JOIN), object_id and details are nullable columns.
type auditRowWithTotal struct {
	ID          int64           `db:"id"`
	UserID      int16           `db:"user_id"`
	UserName    string          `db:"user_name"`
	Action      string          `db:"action"`
	ObjectType  *string         `db:"object_type"`
	ObjectID    *uuid.UUID      `db:"object_id"`
	Details     json.RawMessage `db:"details"`
	PerformedAt time.Time       `db:"performed_at"`
	Total       int             `db:"total"`
}

func toAuditEntry(r auditRowWithTotal) domain.AuditEntry {
	return domain.AuditEntry{
		ID:          r.ID,
		UserID:      r.UserID,
		UserName:    r.UserName,
		Action:      r.Action,
		ObjectType:  r.ObjectType,
		ObjectID:    r.ObjectID,
		Details:     r.Details,
		PerformedAt: r.PerformedAt,
	}
}

// AuditRepo implements port.AuditRepo using PostgreSQL.
type AuditRepo struct {
	pool *pgxpool.Pool
}

// NewAuditRepo creates an AuditRepo backed by pool.
func NewAuditRepo(pool *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{pool: pool}
}

var _ port.AuditRepo = (*AuditRepo)(nil)

// Log inserts one audit record. action_type_id and object_type_id are resolved
// from the reference tables inside the INSERT via subqueries.
func (r *AuditRepo) Log(ctx context.Context, entry domain.AuditEntry) error {
	const sql = `
		INSERT INTO activity.audit_log
		    (user_id, action_type_id, object_type_id, object_id, details)
		VALUES (
		    $1,
		    (SELECT id FROM activity.action_types WHERE name = $2),
		    CASE WHEN $3::text IS NOT NULL
		         THEN (SELECT id FROM core.object_types WHERE name = $3)
		         ELSE NULL END,
		    $4,
		    $5
		)`

	q := connOrTx(ctx, r.pool)
	_, err := q.Exec(ctx, sql,
		entry.UserID,
		entry.Action,
		entry.ObjectType,
		entry.ObjectID,
		entry.Details,
	)
	if err != nil {
		return fmt.Errorf("AuditRepo.Log: %w", err)
	}
	return nil
}

// List returns a filtered, offset-paginated page of audit log entries ordered
// newest-first.
func (r *AuditRepo) List(ctx context.Context, filter domain.AuditFilter) (*domain.AuditPage, error) {
	var conds []string
	args := make([]any, 0, 8)
	n := 1

	if filter.UserID != nil {
		conds = append(conds, fmt.Sprintf("a.user_id = $%d", n))
		args = append(args, *filter.UserID)
		n++
	}
	if filter.Action != "" {
		conds = append(conds, fmt.Sprintf("at.name = $%d", n))
		args = append(args, filter.Action)
		n++
	}
	if filter.ObjectType != "" {
		conds = append(conds, fmt.Sprintf("ot.name = $%d", n))
		args = append(args, filter.ObjectType)
		n++
	}
	if filter.ObjectID != nil {
		conds = append(conds, fmt.Sprintf("a.object_id = $%d", n))
		args = append(args, *filter.ObjectID)
		n++
	}
	if filter.From != nil {
		conds = append(conds, fmt.Sprintf("a.performed_at >= $%d", n))
		args = append(args, *filter.From)
		n++
	}
	if filter.To != nil {
		conds = append(conds, fmt.Sprintf("a.performed_at <= $%d", n))
		args = append(args, *filter.To)
		n++
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	limit := db.ClampLimit(filter.Limit, 50, 200)
	offset := db.ClampOffset(filter.Offset)
	args = append(args, limit, offset)

	sql := fmt.Sprintf(`
		SELECT a.id, a.user_id, u.name AS user_name,
		       at.name AS action,
		       ot.name AS object_type,
		       a.object_id, a.details,
		       a.performed_at,
		       COUNT(*) OVER() AS total
		FROM activity.audit_log a
		JOIN  core.users u             ON u.id  = a.user_id
		JOIN  activity.action_types at ON at.id = a.action_type_id
		LEFT JOIN core.object_types ot ON ot.id = a.object_type_id
		%s
		ORDER BY a.performed_at DESC
		LIMIT $%d OFFSET $%d`, where, n, n+1)

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("AuditRepo.List: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[auditRowWithTotal])
	if err != nil {
		return nil, fmt.Errorf("AuditRepo.List scan: %w", err)
	}

	page := &domain.AuditPage{Offset: offset, Limit: limit}
	if len(collected) > 0 {
		page.Total = collected[0].Total
	}
	page.Items = make([]domain.AuditEntry, len(collected))
	for i, row := range collected {
		page.Items[i] = toAuditEntry(row)
	}
	return page, nil
}
