package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

// ---------------------------------------------------------------------------
// Row structs — use pgx-scannable types
// ---------------------------------------------------------------------------

type tagRow struct {
	ID            uuid.UUID  `db:"id"`
	Name          string      `db:"name"`
	Notes         *string     `db:"notes"`
	Color         *string     `db:"color"`
	CategoryID    *uuid.UUID `db:"category_id"`
	CategoryName  *string     `db:"category_name"`
	CategoryColor *string     `db:"category_color"`
	Metadata      []byte      `db:"metadata"`
	CreatorID     int16       `db:"creator_id"`
	CreatorName   string      `db:"creator_name"`
	IsPublic      bool        `db:"is_public"`
}

type tagRowWithTotal struct {
	tagRow
	Total int `db:"total"`
}

type tagRuleRow struct {
	WhenTagID   uuid.UUID `db:"when_tag_id"`
	ThenTagID   uuid.UUID `db:"then_tag_id"`
	ThenTagName string     `db:"then_tag_name"`
	IsActive    bool       `db:"is_active"`
}

// ---------------------------------------------------------------------------
// Converters
// ---------------------------------------------------------------------------

func toTag(r tagRow) domain.Tag {
	t := domain.Tag{
		ID:            r.ID,
		Name:          r.Name,
		Notes:         r.Notes,
		Color:         r.Color,
		CategoryID:    r.CategoryID,
		CategoryName:  r.CategoryName,
		CategoryColor: r.CategoryColor,
		CreatorID:     r.CreatorID,
		CreatorName:   r.CreatorName,
		IsPublic:      r.IsPublic,
		CreatedAt:     domain.UUIDCreatedAt(r.ID),
	}
	if len(r.Metadata) > 0 && string(r.Metadata) != "null" {
		t.Metadata = json.RawMessage(r.Metadata)
	}
	return t
}

func toTagRule(r tagRuleRow) domain.TagRule {
	return domain.TagRule{
		WhenTagID:   r.WhenTagID,
		ThenTagID:   r.ThenTagID,
		ThenTagName: r.ThenTagName,
		IsActive:    r.IsActive,
	}
}

// ---------------------------------------------------------------------------
// Shared SQL fragments
// ---------------------------------------------------------------------------

const tagSelectFrom = `
SELECT
    t.id,
    t.name,
    t.notes,
    t.color,
    t.category_id,
    c.name  AS category_name,
    c.color AS category_color,
    t.metadata,
    t.creator_id,
    u.name  AS creator_name,
    t.is_public
FROM data.tags t
LEFT JOIN data.categories c ON c.id = t.category_id
JOIN      core.users u ON u.id = t.creator_id`

func tagSortColumn(s string) string {
	switch s {
	case "name":
		return "t.name"
	case "color":
		return "t.color"
	case "category_name":
		return "c.name"
	default: // "created"
		return "t.id"
	}
}

// isPgUniqueViolation reports whether err is a PostgreSQL unique-constraint error.
func isPgUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// ---------------------------------------------------------------------------
// TagRepo — implements port.TagRepo
// ---------------------------------------------------------------------------

// TagRepo handles tag CRUD and file–tag relations.
type TagRepo struct {
	pool *pgxpool.Pool
}

var _ port.TagRepo = (*TagRepo)(nil)

// NewTagRepo creates a TagRepo backed by pool.
func NewTagRepo(pool *pgxpool.Pool) *TagRepo {
	return &TagRepo{pool: pool}
}

// ---------------------------------------------------------------------------
// List / ListByCategory
// ---------------------------------------------------------------------------

func (r *TagRepo) List(ctx context.Context, params port.OffsetParams) (*domain.TagOffsetPage, error) {
	return r.listTags(ctx, params, nil)
}

func (r *TagRepo) ListByCategory(ctx context.Context, categoryID uuid.UUID, params port.OffsetParams) (*domain.TagOffsetPage, error) {
	return r.listTags(ctx, params, &categoryID)
}

func (r *TagRepo) listTags(ctx context.Context, params port.OffsetParams, categoryID *uuid.UUID) (*domain.TagOffsetPage, error) {
	order := "ASC"
	if strings.ToLower(params.Order) == "desc" {
		order = "DESC"
	}
	sortCol := tagSortColumn(params.Sort)

	args := []any{}
	n := 1
	var conditions []string

	if params.Search != "" {
		conditions = append(conditions, fmt.Sprintf("lower(t.name) LIKE lower($%d)", n))
		args = append(args, "%"+params.Search+"%")
		n++
	}
	if categoryID != nil {
		conditions = append(conditions, fmt.Sprintf("t.category_id = $%d", n))
		args = append(args, *categoryID)
		n++
	}
	// Restrict to tags the viewer may see (private-by-default), unless admin.
	if !params.ViewerIsAdmin {
		var aclCond string
		aclCond, n, args = aclVisibilityCond("t", objTypeTag, params.ViewerID, n, args)
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
    t.id, t.name, t.notes, t.color,
    t.category_id,
    c.name  AS category_name,
    c.color AS category_color,
    t.metadata, t.creator_id,
    u.name  AS creator_name,
    t.is_public,
    COUNT(*) OVER() AS total
FROM data.tags t
LEFT JOIN data.categories c ON c.id = t.category_id
JOIN      core.users u ON u.id = t.creator_id
%s
ORDER BY %s %s NULLS LAST, t.id ASC
LIMIT $%d OFFSET $%d`, where, sortCol, order, n, n+1)

	args = append(args, limit, offset)

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("TagRepo.List query: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[tagRowWithTotal])
	if err != nil {
		return nil, fmt.Errorf("TagRepo.List scan: %w", err)
	}

	items := make([]domain.Tag, len(collected))
	total := 0
	for i, row := range collected {
		items[i] = toTag(row.tagRow)
		total = row.Total
	}
	return &domain.TagOffsetPage{
		Items:  items,
		Total:  total,
		Offset: offset,
		Limit:  limit,
	}, nil
}

// ---------------------------------------------------------------------------
// GetByID
// ---------------------------------------------------------------------------

func (r *TagRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tag, error) {
	const query = tagSelectFrom + `
WHERE t.id = $1`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("TagRepo.GetByID: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[tagRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("TagRepo.GetByID scan: %w", err)
	}
	t := toTag(row)
	return &t, nil
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func (r *TagRepo) Create(ctx context.Context, t *domain.Tag) (*domain.Tag, error) {
	const query = `
WITH ins AS (
    INSERT INTO data.tags (name, notes, color, category_id, metadata, creator_id, is_public)
    VALUES ($1, $2, $3, $4, $5, $6, $7)
    RETURNING *
)
SELECT
    ins.id, ins.name, ins.notes, ins.color,
    ins.category_id,
    c.name  AS category_name,
    c.color AS category_color,
    ins.metadata, ins.creator_id,
    u.name  AS creator_name,
    ins.is_public
FROM ins
LEFT JOIN data.categories c ON c.id = ins.category_id
JOIN      core.users u ON u.id = ins.creator_id`

	var meta any
	if len(t.Metadata) > 0 {
		meta = t.Metadata
	}

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, query,
		t.Name, t.Notes, t.Color, t.CategoryID, meta, t.CreatorID, t.IsPublic)
	if err != nil {
		return nil, fmt.Errorf("TagRepo.Create: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[tagRow])
	if err != nil {
		if isPgUniqueViolation(err) {
			return nil, domain.ErrConflict
		}
		return nil, fmt.Errorf("TagRepo.Create scan: %w", err)
	}
	created := toTag(row)
	return &created, nil
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update replaces all mutable fields. The caller must merge current values with
// the patch (read-then-write) before calling this.
func (r *TagRepo) Update(ctx context.Context, id uuid.UUID, t *domain.Tag) (*domain.Tag, error) {
	const query = `
WITH upd AS (
    UPDATE data.tags SET
        name        = $2,
        notes       = $3,
        color       = $4,
        category_id = $5,
        metadata    = COALESCE($6, metadata),
        is_public   = $7
    WHERE id = $1
    RETURNING *
)
SELECT
    upd.id, upd.name, upd.notes, upd.color,
    upd.category_id,
    c.name  AS category_name,
    c.color AS category_color,
    upd.metadata, upd.creator_id,
    u.name  AS creator_name,
    upd.is_public
FROM upd
LEFT JOIN data.categories c ON c.id = upd.category_id
JOIN      core.users u ON u.id = upd.creator_id`

	var meta any
	if len(t.Metadata) > 0 {
		meta = t.Metadata
	}

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, query,
		id, t.Name, t.Notes, t.Color, t.CategoryID, meta, t.IsPublic)
	if err != nil {
		return nil, fmt.Errorf("TagRepo.Update: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[tagRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		if isPgUniqueViolation(err) {
			return nil, domain.ErrConflict
		}
		return nil, fmt.Errorf("TagRepo.Update scan: %w", err)
	}
	updated := toTag(row)
	return &updated, nil
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func (r *TagRepo) Delete(ctx context.Context, id uuid.UUID) error {
	const query = `DELETE FROM data.tags WHERE id = $1`

	q := connOrTx(ctx, r.pool)
	ct, err := q.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("TagRepo.Delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// File–tag operations
// ---------------------------------------------------------------------------

func (r *TagRepo) ListByFile(ctx context.Context, fileID uuid.UUID) ([]domain.Tag, error) {
	const query = tagSelectFrom + `
JOIN data.file_tag ft ON ft.tag_id = t.id
WHERE ft.file_id = $1
ORDER BY t.name`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, query, fileID)
	if err != nil {
		return nil, fmt.Errorf("TagRepo.ListByFile: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[tagRow])
	if err != nil {
		return nil, fmt.Errorf("TagRepo.ListByFile scan: %w", err)
	}
	tags := make([]domain.Tag, len(collected))
	for i, row := range collected {
		tags[i] = toTag(row)
	}
	return tags, nil
}

func (r *TagRepo) AddFileTag(ctx context.Context, fileID, tagID uuid.UUID) error {
	const query = `
INSERT INTO data.file_tag (file_id, tag_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING`

	q := connOrTx(ctx, r.pool)
	if _, err := q.Exec(ctx, query, fileID, tagID); err != nil {
		return fmt.Errorf("TagRepo.AddFileTag: %w", err)
	}
	return nil
}

func (r *TagRepo) RemoveFileTag(ctx context.Context, fileID, tagID uuid.UUID) error {
	const query = `DELETE FROM data.file_tag WHERE file_id = $1 AND tag_id = $2`

	q := connOrTx(ctx, r.pool)
	if _, err := q.Exec(ctx, query, fileID, tagID); err != nil {
		return fmt.Errorf("TagRepo.RemoveFileTag: %w", err)
	}
	return nil
}

func (r *TagRepo) SetFileTags(ctx context.Context, fileID uuid.UUID, tagIDs []uuid.UUID) error {
	q := connOrTx(ctx, r.pool)

	if _, err := q.Exec(ctx,
		`DELETE FROM data.file_tag WHERE file_id = $1`, fileID); err != nil {
		return fmt.Errorf("TagRepo.SetFileTags delete: %w", err)
	}
	if len(tagIDs) == 0 {
		return nil
	}

	placeholders := make([]string, len(tagIDs))
	args := []any{fileID}
	for i, tagID := range tagIDs {
		placeholders[i] = fmt.Sprintf("($1, $%d)", i+2)
		args = append(args, tagID)
	}
	ins := `INSERT INTO data.file_tag (file_id, tag_id) VALUES ` +
		strings.Join(placeholders, ", ") + ` ON CONFLICT DO NOTHING`

	if _, err := q.Exec(ctx, ins, args...); err != nil {
		return fmt.Errorf("TagRepo.SetFileTags insert: %w", err)
	}
	return nil
}

func (r *TagRepo) CommonTagsForFiles(ctx context.Context, fileIDs []uuid.UUID) ([]domain.Tag, error) {
	if len(fileIDs) == 0 {
		return []domain.Tag{}, nil
	}
	return r.queryTagsByPresence(ctx, fileIDs, "=")
}

func (r *TagRepo) PartialTagsForFiles(ctx context.Context, fileIDs []uuid.UUID) ([]domain.Tag, error) {
	if len(fileIDs) == 0 {
		return []domain.Tag{}, nil
	}
	return r.queryTagsByPresence(ctx, fileIDs, "<")
}

func (r *TagRepo) queryTagsByPresence(ctx context.Context, fileIDs []uuid.UUID, op string) ([]domain.Tag, error) {
	placeholders := make([]string, len(fileIDs))
	args := make([]any, len(fileIDs)+1)
	for i, id := range fileIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	args[len(fileIDs)] = len(fileIDs)
	n := len(fileIDs) + 1

	query := fmt.Sprintf(`
SELECT
    t.id, t.name, t.notes, t.color,
    t.category_id,
    c.name  AS category_name,
    c.color AS category_color,
    t.metadata, t.creator_id,
    u.name  AS creator_name,
    t.is_public
FROM data.tags t
JOIN data.file_tag ft ON ft.tag_id = t.id
LEFT JOIN data.categories c ON c.id = t.category_id
JOIN      core.users u ON u.id = t.creator_id
WHERE ft.file_id IN (%s)
GROUP BY t.id, c.id, u.id
HAVING COUNT(DISTINCT ft.file_id) %s $%d
ORDER BY t.name`,
		strings.Join(placeholders, ", "), op, n)

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("TagRepo.queryTagsByPresence: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[tagRow])
	if err != nil {
		return nil, fmt.Errorf("TagRepo.queryTagsByPresence scan: %w", err)
	}
	tags := make([]domain.Tag, len(collected))
	for i, row := range collected {
		tags[i] = toTag(row)
	}
	return tags, nil
}

// ---------------------------------------------------------------------------
// TagRuleRepo — implements port.TagRuleRepo (separate type to avoid method collision)
// ---------------------------------------------------------------------------

// TagRuleRepo handles tag-rule CRUD.
type TagRuleRepo struct {
	pool *pgxpool.Pool
}

var _ port.TagRuleRepo = (*TagRuleRepo)(nil)

// NewTagRuleRepo creates a TagRuleRepo backed by pool.
func NewTagRuleRepo(pool *pgxpool.Pool) *TagRuleRepo {
	return &TagRuleRepo{pool: pool}
}

func (r *TagRuleRepo) ListByTag(ctx context.Context, tagID uuid.UUID) ([]domain.TagRule, error) {
	const query = `
SELECT
    tr.when_tag_id,
    tr.then_tag_id,
    t.name AS then_tag_name,
    tr.is_active
FROM data.tag_rules tr
JOIN data.tags t ON t.id = tr.then_tag_id
WHERE tr.when_tag_id = $1
ORDER BY t.name`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, query, tagID)
	if err != nil {
		return nil, fmt.Errorf("TagRuleRepo.ListByTag: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[tagRuleRow])
	if err != nil {
		return nil, fmt.Errorf("TagRuleRepo.ListByTag scan: %w", err)
	}
	rules := make([]domain.TagRule, len(collected))
	for i, row := range collected {
		rules[i] = toTagRule(row)
	}
	return rules, nil
}

func (r *TagRuleRepo) Create(ctx context.Context, rule domain.TagRule) (*domain.TagRule, error) {
	const query = `
WITH ins AS (
    INSERT INTO data.tag_rules (when_tag_id, then_tag_id, is_active)
    VALUES ($1, $2, $3)
    RETURNING *
)
SELECT ins.when_tag_id, ins.then_tag_id, t.name AS then_tag_name, ins.is_active
FROM ins
JOIN data.tags t ON t.id = ins.then_tag_id`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, query, rule.WhenTagID, rule.ThenTagID, rule.IsActive)
	if err != nil {
		return nil, fmt.Errorf("TagRuleRepo.Create: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[tagRuleRow])
	if err != nil {
		if isPgUniqueViolation(err) {
			return nil, domain.ErrConflict
		}
		return nil, fmt.Errorf("TagRuleRepo.Create scan: %w", err)
	}
	result := toTagRule(row)
	return &result, nil
}

func (r *TagRuleRepo) SetActive(ctx context.Context, whenTagID, thenTagID uuid.UUID, active, applyToExisting bool) error {
	const updateQuery = `
UPDATE data.tag_rules SET is_active = $3
WHERE when_tag_id = $1 AND then_tag_id = $2`

	q := connOrTx(ctx, r.pool)
	ct, err := q.Exec(ctx, updateQuery, whenTagID, thenTagID, active)
	if err != nil {
		return fmt.Errorf("TagRuleRepo.SetActive: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	if !active || !applyToExisting {
		return nil
	}

	// Retroactively apply the full transitive expansion of thenTagID to all
	// files that already carry whenTagID. The recursive CTE walks active rules
	// starting from thenTagID (mirrors the Go expandTagSet BFS).
	const retroQuery = `
WITH RECURSIVE expansion(tag_id) AS (
    SELECT $2::uuid
    UNION
    SELECT r.then_tag_id
    FROM data.tag_rules r
    JOIN expansion e ON r.when_tag_id = e.tag_id
    WHERE r.is_active = true
)
INSERT INTO data.file_tag (file_id, tag_id)
SELECT ft.file_id, e.tag_id
FROM data.file_tag ft
CROSS JOIN expansion e
WHERE ft.tag_id = $1
ON CONFLICT DO NOTHING`

	if _, err := q.Exec(ctx, retroQuery, whenTagID, thenTagID); err != nil {
		return fmt.Errorf("TagRuleRepo.SetActive retroactive apply: %w", err)
	}
	return nil
}

func (r *TagRuleRepo) Delete(ctx context.Context, whenTagID, thenTagID uuid.UUID) error {
	const query = `
DELETE FROM data.tag_rules
WHERE when_tag_id = $1 AND then_tag_id = $2`

	q := connOrTx(ctx, r.pool)
	ct, err := q.Exec(ctx, query, whenTagID, thenTagID)
	if err != nil {
		return fmt.Errorf("TagRuleRepo.Delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}