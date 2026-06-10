package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

// ---------------------------------------------------------------------------
// Row struct
// ---------------------------------------------------------------------------

type categoryRow struct {
	ID          uuid.UUID `db:"id"`
	Name        string    `db:"name"`
	Notes       *string   `db:"notes"`
	Color       *string   `db:"color"`
	Metadata    []byte    `db:"metadata"`
	CreatorID   int16     `db:"creator_id"`
	CreatorName string    `db:"creator_name"`
	IsPublic    bool      `db:"is_public"`
}

type categoryRowWithTotal struct {
	categoryRow
	Total int `db:"total"`
}

// ---------------------------------------------------------------------------
// Converter
// ---------------------------------------------------------------------------

func toCategory(r categoryRow) domain.Category {
	c := domain.Category{
		ID:          r.ID,
		Name:        r.Name,
		Notes:       r.Notes,
		Color:       r.Color,
		CreatorID:   r.CreatorID,
		CreatorName: r.CreatorName,
		IsPublic:    r.IsPublic,
		CreatedAt:   domain.UUIDCreatedAt(r.ID),
	}
	if len(r.Metadata) > 0 && string(r.Metadata) != "null" {
		c.Metadata = json.RawMessage(r.Metadata)
	}
	return c
}

// ---------------------------------------------------------------------------
// Shared SQL
// ---------------------------------------------------------------------------

const categorySelectFrom = `
SELECT
    c.id,
    c.name,
    c.notes,
    c.color,
    c.metadata,
    c.creator_id,
    u.name AS creator_name,
    c.is_public
FROM data.categories c
JOIN core.users u ON u.id = c.creator_id`

func categorySortColumn(s string) string {
	if s == "name" {
		return "c.name"
	}
	return "c.id" // "created"
}

// ---------------------------------------------------------------------------
// CategoryRepo — implements port.CategoryRepo
// ---------------------------------------------------------------------------

// CategoryRepo handles category CRUD using PostgreSQL.
type CategoryRepo struct {
	pool *pgxpool.Pool
}

var _ port.CategoryRepo = (*CategoryRepo)(nil)

// NewCategoryRepo creates a CategoryRepo backed by pool.
func NewCategoryRepo(pool *pgxpool.Pool) *CategoryRepo {
	return &CategoryRepo{pool: pool}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func (r *CategoryRepo) List(ctx context.Context, params port.OffsetParams) (*domain.CategoryOffsetPage, error) {
	order := "ASC"
	if strings.ToLower(params.Order) == "desc" {
		order = "DESC"
	}
	sortCol := categorySortColumn(params.Sort)

	args := []any{}
	n := 1
	var conditions []string

	if params.Search != "" {
		conditions = append(conditions, fmt.Sprintf("lower(c.name) LIKE lower($%d)", n))
		args = append(args, "%"+params.Search+"%")
		n++
	}
	// Restrict to categories the viewer may see (private-by-default), unless admin.
	if !params.ViewerIsAdmin {
		var aclCond string
		aclCond, n, args = aclVisibilityCond("c", objTypeCategory, params.ViewerID, n, args)
		conditions = append(conditions, aclCond)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
SELECT
    c.id, c.name, c.notes, c.color, c.metadata,
    c.creator_id, u.name AS creator_name, c.is_public,
    COUNT(*) OVER() AS total
FROM data.categories c
JOIN core.users u ON u.id = c.creator_id
%s
ORDER BY %s %s NULLS LAST, c.id ASC
LIMIT $%d OFFSET $%d`, where, sortCol, order, n, n+1)

	args = append(args, limit, offset)

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("CategoryRepo.List query: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[categoryRowWithTotal])
	if err != nil {
		return nil, fmt.Errorf("CategoryRepo.List scan: %w", err)
	}

	items := make([]domain.Category, len(collected))
	total := 0
	for i, row := range collected {
		items[i] = toCategory(row.categoryRow)
		total = row.Total
	}
	return &domain.CategoryOffsetPage{
		Items:  items,
		Total:  total,
		Offset: offset,
		Limit:  limit,
	}, nil
}

// ---------------------------------------------------------------------------
// GetByID
// ---------------------------------------------------------------------------

func (r *CategoryRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Category, error) {
	const query = categorySelectFrom + `
WHERE c.id = $1`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("CategoryRepo.GetByID: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[categoryRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("CategoryRepo.GetByID scan: %w", err)
	}
	c := toCategory(row)
	return &c, nil
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func (r *CategoryRepo) Create(ctx context.Context, c *domain.Category) (*domain.Category, error) {
	const query = `
WITH ins AS (
    INSERT INTO data.categories (name, notes, color, metadata, creator_id, is_public)
    VALUES ($1, $2, $3, $4, $5, $6)
    RETURNING *
)
SELECT ins.id, ins.name, ins.notes, ins.color, ins.metadata,
       ins.creator_id, u.name AS creator_name, ins.is_public
FROM ins
JOIN core.users u ON u.id = ins.creator_id`

	var meta any
	if len(c.Metadata) > 0 {
		meta = c.Metadata
	}

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, query,
		c.Name, c.Notes, c.Color, meta, c.CreatorID, c.IsPublic)
	if err != nil {
		return nil, fmt.Errorf("CategoryRepo.Create: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[categoryRow])
	if err != nil {
		if isPgUniqueViolation(err) {
			return nil, domain.ErrConflict
		}
		return nil, fmt.Errorf("CategoryRepo.Create scan: %w", err)
	}
	created := toCategory(row)
	return &created, nil
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update replaces all mutable fields. The caller must merge current values
// with the patch before calling (read-then-write semantics).
func (r *CategoryRepo) Update(ctx context.Context, id uuid.UUID, c *domain.Category) (*domain.Category, error) {
	const query = `
WITH upd AS (
    UPDATE data.categories SET
        name      = $2,
        notes     = $3,
        color     = $4,
        metadata  = COALESCE($5, metadata),
        is_public = $6
    WHERE id = $1
    RETURNING *
)
SELECT upd.id, upd.name, upd.notes, upd.color, upd.metadata,
       upd.creator_id, u.name AS creator_name, upd.is_public
FROM upd
JOIN core.users u ON u.id = upd.creator_id`

	var meta any
	if len(c.Metadata) > 0 {
		meta = c.Metadata
	}

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, query,
		id, c.Name, c.Notes, c.Color, meta, c.IsPublic)
	if err != nil {
		return nil, fmt.Errorf("CategoryRepo.Update: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[categoryRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		if isPgUniqueViolation(err) {
			return nil, domain.ErrConflict
		}
		return nil, fmt.Errorf("CategoryRepo.Update scan: %w", err)
	}
	updated := toCategory(row)
	return &updated, nil
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func (r *CategoryRepo) Delete(ctx context.Context, id uuid.UUID) error {
	const query = `DELETE FROM data.categories WHERE id = $1`

	q := connOrTx(ctx, r.pool)
	ct, err := q.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("CategoryRepo.Delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}