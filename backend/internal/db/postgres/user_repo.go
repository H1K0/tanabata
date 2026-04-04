package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tanabata/backend/internal/db"
	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

// userRow matches the columns returned by every user SELECT.
type userRow struct {
	ID        int16  `db:"id"`
	Name      string `db:"name"`
	Password  string `db:"password"`
	IsAdmin   bool   `db:"is_admin"`
	CanCreate bool   `db:"can_create"`
	IsBlocked bool   `db:"is_blocked"`
}

// userRowWithTotal extends userRow with a window-function total for List.
type userRowWithTotal struct {
	userRow
	Total int `db:"total"`
}

func toUser(r userRow) domain.User {
	return domain.User{
		ID:        r.ID,
		Name:      r.Name,
		Password:  r.Password,
		IsAdmin:   r.IsAdmin,
		CanCreate: r.CanCreate,
		IsBlocked: r.IsBlocked,
	}
}

// userSortColumn whitelists valid sort keys to prevent SQL injection.
var userSortColumn = map[string]string{
	"name": "name",
	"id":   "id",
}

// UserRepo implements port.UserRepo using PostgreSQL.
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo creates a UserRepo backed by pool.
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

var _ port.UserRepo = (*UserRepo)(nil)

func (r *UserRepo) List(ctx context.Context, params port.OffsetParams) (*domain.UserPage, error) {
	col, ok := userSortColumn[params.Sort]
	if !ok {
		col = "id"
	}
	ord := "ASC"
	if params.Order == "desc" {
		ord = "DESC"
	}
	limit := db.ClampLimit(params.Limit, 50, 200)
	offset := db.ClampOffset(params.Offset)

	sql := fmt.Sprintf(`
		SELECT id, name, password, is_admin, can_create, is_blocked,
		       COUNT(*) OVER() AS total
		FROM core.users
		ORDER BY %s %s
		LIMIT $1 OFFSET $2`, col, ord)

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sql, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("UserRepo.List: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[userRowWithTotal])
	if err != nil {
		return nil, fmt.Errorf("UserRepo.List scan: %w", err)
	}

	page := &domain.UserPage{Offset: offset, Limit: limit}
	if len(collected) > 0 {
		page.Total = collected[0].Total
	}
	page.Items = make([]domain.User, len(collected))
	for i, row := range collected {
		page.Items[i] = toUser(row.userRow)
	}
	return page, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id int16) (*domain.User, error) {
	const sql = `
		SELECT id, name, password, is_admin, can_create, is_blocked
		FROM core.users WHERE id = $1`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sql, id)
	if err != nil {
		return nil, fmt.Errorf("UserRepo.GetByID: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[userRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("UserRepo.GetByID scan: %w", err)
	}
	u := toUser(row)
	return &u, nil
}

func (r *UserRepo) GetByName(ctx context.Context, name string) (*domain.User, error) {
	const sql = `
		SELECT id, name, password, is_admin, can_create, is_blocked
		FROM core.users WHERE name = $1`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sql, name)
	if err != nil {
		return nil, fmt.Errorf("UserRepo.GetByName: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[userRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("UserRepo.GetByName scan: %w", err)
	}
	u := toUser(row)
	return &u, nil
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) (*domain.User, error) {
	const sql = `
		INSERT INTO core.users (name, password, is_admin, can_create)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, password, is_admin, can_create, is_blocked`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sql, u.Name, u.Password, u.IsAdmin, u.CanCreate)
	if err != nil {
		return nil, fmt.Errorf("UserRepo.Create: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[userRow])
	if err != nil {
		return nil, fmt.Errorf("UserRepo.Create scan: %w", err)
	}
	created := toUser(row)
	return &created, nil
}

func (r *UserRepo) Update(ctx context.Context, id int16, u *domain.User) (*domain.User, error) {
	const sql = `
		UPDATE core.users
		SET name = $2, password = $3, is_admin = $4, can_create = $5, is_blocked = $6
		WHERE id = $1
		RETURNING id, name, password, is_admin, can_create, is_blocked`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sql, id, u.Name, u.Password, u.IsAdmin, u.CanCreate, u.IsBlocked)
	if err != nil {
		return nil, fmt.Errorf("UserRepo.Update: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[userRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("UserRepo.Update scan: %w", err)
	}
	updated := toUser(row)
	return &updated, nil
}

func (r *UserRepo) Delete(ctx context.Context, id int16) error {
	const sql = `DELETE FROM core.users WHERE id = $1`
	q := connOrTx(ctx, r.pool)
	tag, err := q.Exec(ctx, sql, id)
	if err != nil {
		return fmt.Errorf("UserRepo.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
