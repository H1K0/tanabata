package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

type permissionRow struct {
	UserID       int16     `db:"user_id"`
	UserName     string    `db:"user_name"`
	ObjectTypeID int16     `db:"object_type_id"`
	ObjectID     uuid.UUID `db:"object_id"`
	CanView      bool      `db:"can_view"`
	CanEdit      bool      `db:"can_edit"`
}

func toPermission(r permissionRow) domain.Permission {
	return domain.Permission{
		UserID:       r.UserID,
		UserName:     r.UserName,
		ObjectTypeID: r.ObjectTypeID,
		ObjectID:     r.ObjectID,
		CanView:      r.CanView,
		CanEdit:      r.CanEdit,
	}
}

// ACLRepo implements port.ACLRepo using PostgreSQL.
type ACLRepo struct {
	pool *pgxpool.Pool
}

// NewACLRepo creates an ACLRepo backed by pool.
func NewACLRepo(pool *pgxpool.Pool) *ACLRepo {
	return &ACLRepo{pool: pool}
}

var _ port.ACLRepo = (*ACLRepo)(nil)

func (r *ACLRepo) List(ctx context.Context, objectTypeID int16, objectID uuid.UUID) ([]domain.Permission, error) {
	const sql = `
		SELECT p.user_id, u.name AS user_name, p.object_type_id, p.object_id,
		       p.can_view, p.can_edit
		FROM acl.permissions p
		JOIN core.users u ON u.id = p.user_id
		WHERE p.object_type_id = $1 AND p.object_id = $2
		ORDER BY u.name`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sql, objectTypeID, objectID)
	if err != nil {
		return nil, fmt.Errorf("ACLRepo.List: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[permissionRow])
	if err != nil {
		return nil, fmt.Errorf("ACLRepo.List scan: %w", err)
	}
	perms := make([]domain.Permission, len(collected))
	for i, row := range collected {
		perms[i] = toPermission(row)
	}
	return perms, nil
}

func (r *ACLRepo) Get(ctx context.Context, userID int16, objectTypeID int16, objectID uuid.UUID) (*domain.Permission, error) {
	const sql = `
		SELECT p.user_id, u.name AS user_name, p.object_type_id, p.object_id,
		       p.can_view, p.can_edit
		FROM acl.permissions p
		JOIN core.users u ON u.id = p.user_id
		WHERE p.user_id = $1 AND p.object_type_id = $2 AND p.object_id = $3`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sql, userID, objectTypeID, objectID)
	if err != nil {
		return nil, fmt.Errorf("ACLRepo.Get: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[permissionRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("ACLRepo.Get scan: %w", err)
	}
	p := toPermission(row)
	return &p, nil
}

func (r *ACLRepo) Set(ctx context.Context, objectTypeID int16, objectID uuid.UUID, perms []domain.Permission) error {
	q := connOrTx(ctx, r.pool)

	const del = `DELETE FROM acl.permissions WHERE object_type_id = $1 AND object_id = $2`
	if _, err := q.Exec(ctx, del, objectTypeID, objectID); err != nil {
		return fmt.Errorf("ACLRepo.Set delete: %w", err)
	}

	if len(perms) == 0 {
		return nil
	}

	const ins = `
		INSERT INTO acl.permissions (user_id, object_type_id, object_id, can_view, can_edit)
		VALUES ($1, $2, $3, $4, $5)`
	for _, p := range perms {
		if _, err := q.Exec(ctx, ins, p.UserID, objectTypeID, objectID, p.CanView, p.CanEdit); err != nil {
			return fmt.Errorf("ACLRepo.Set insert: %w", err)
		}
	}
	return nil
}
