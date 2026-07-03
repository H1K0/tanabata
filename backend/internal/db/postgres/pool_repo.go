package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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

// ---------------------------------------------------------------------------
// Row structs
// ---------------------------------------------------------------------------

type poolRow struct {
	ID          uuid.UUID `db:"id"`
	Name        string    `db:"name"`
	Notes       *string   `db:"notes"`
	Metadata    []byte    `db:"metadata"`
	CreatorID   int16     `db:"creator_id"`
	CreatorName string    `db:"creator_name"`
	IsPublic    bool      `db:"is_public"`
	SortKey     string    `db:"sort_key"`
	SortOrder   string    `db:"sort_order"`
	FileCount   int       `db:"file_count"`
}

type poolRowWithTotal struct {
	poolRow
	Total int `db:"total"`
}

// poolFileRow is a flat struct combining all file columns plus pool position.
type poolFileRow struct {
	ID              uuid.UUID       `db:"id"`
	OriginalName    *string         `db:"original_name"`
	MIMEType        string          `db:"mime_type"`
	MIMEExtension   string          `db:"mime_extension"`
	ContentDatetime time.Time       `db:"content_datetime"`
	Notes           *string         `db:"notes"`
	Metadata        json.RawMessage `db:"metadata"`
	EXIF            json.RawMessage `db:"exif"`
	PHash           *int64          `db:"phash"`
	CreatorID       int16           `db:"creator_id"`
	CreatorName     string          `db:"creator_name"`
	IsPublic        bool            `db:"is_public"`
	IsDeleted       bool            `db:"is_deleted"`
	Position        int             `db:"position"`
}

// ---------------------------------------------------------------------------
// Converters
// ---------------------------------------------------------------------------

func toPool(r poolRow) domain.Pool {
	p := domain.Pool{
		ID:          r.ID,
		Name:        r.Name,
		Notes:       r.Notes,
		CreatorID:   r.CreatorID,
		CreatorName: r.CreatorName,
		IsPublic:    r.IsPublic,
		SortKey:     r.SortKey,
		SortOrder:   r.SortOrder,
		FileCount:   r.FileCount,
		CreatedAt:   domain.UUIDCreatedAt(r.ID),
	}
	if len(r.Metadata) > 0 && string(r.Metadata) != "null" {
		p.Metadata = json.RawMessage(r.Metadata)
	}
	return p
}

func toPoolFile(r poolFileRow) domain.PoolFile {
	return domain.PoolFile{
		File: domain.File{
			ID:              r.ID,
			OriginalName:    r.OriginalName,
			MIMEType:        r.MIMEType,
			MIMEExtension:   r.MIMEExtension,
			ContentDatetime: r.ContentDatetime,
			Notes:           r.Notes,
			Metadata:        r.Metadata,
			EXIF:            r.EXIF,
			PHash:           r.PHash,
			CreatorID:       r.CreatorID,
			CreatorName:     r.CreatorName,
			IsPublic:        r.IsPublic,
			IsDeleted:       r.IsDeleted,
			CreatedAt:       domain.UUIDCreatedAt(r.ID),
		},
		Position: r.Position,
	}
}

// ---------------------------------------------------------------------------
// Cursor
// ---------------------------------------------------------------------------

// poolFileCursor is the keyset paging cursor for pool files. Which fields are
// populated depends on the pool's sort key: Pos for manual (position), Val for
// content_datetime (RFC3339Nano) / original_name (the coalesced name); the
// "created" sort orders by file id alone, so only FileID matters. FileID is
// always the final tiebreak.
type poolFileCursor struct {
	Pos    int    `json:"p,omitempty"`
	Val    string `json:"v,omitempty"`
	FileID string `json:"id"`
}

func encodePoolCursor(c poolFileCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodePoolCursor(s string) (poolFileCursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return poolFileCursor{}, fmt.Errorf("cursor: invalid encoding")
	}
	var c poolFileCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return poolFileCursor{}, fmt.Errorf("cursor: invalid format")
	}
	return c, nil
}

// ---------------------------------------------------------------------------
// Shared SQL
// ---------------------------------------------------------------------------

// poolCountSubquery computes per-pool file counts.
const poolCountSubquery = `(SELECT pool_id, COUNT(*) AS cnt FROM data.file_pool GROUP BY pool_id)`

const poolSelectFrom = `
SELECT p.id, p.name, p.notes, p.metadata,
       p.creator_id, u.name AS creator_name, p.is_public,
       p.sort_key, p.sort_order,
       COALESCE(fc.cnt, 0) AS file_count
FROM data.pools p
JOIN core.users u ON u.id = p.creator_id
LEFT JOIN ` + poolCountSubquery + ` fc ON fc.pool_id = p.id`

func poolSortColumn(s string) string {
	if s == "name" {
		return "p.name"
	}
	return "p.id" // "created"
}

// poolFileSort maps a pool's stored sort settings to the SQL column expression,
// the ORDER BY direction, and the keyset comparison operator used for paging.
// The column is chosen so it is never NULL (original_name is coalesced), which
// keeps the keyset comparison total.
func poolFileSort(sortKey, sortOrder string) (col, dir, cmp string) {
	dir, cmp = "ASC", ">"
	if strings.EqualFold(sortOrder, domain.SortOrderDesc) {
		dir, cmp = "DESC", "<"
	}
	switch sortKey {
	case domain.PoolSortContentDatetime:
		col = "f.content_datetime"
	case domain.PoolSortOriginalName:
		col = "COALESCE(f.original_name, '')"
	case domain.PoolSortCreated:
		col = "f.id"
	default: // manual — the user-arranged sequence; direction does not apply
		col = "fp.position"
		dir, cmp = "ASC", ">"
	}
	return col, dir, cmp
}

// ---------------------------------------------------------------------------
// PoolRepo
// ---------------------------------------------------------------------------

// PoolRepo implements port.PoolRepo using PostgreSQL.
type PoolRepo struct {
	pool *pgxpool.Pool
}

var _ port.PoolRepo = (*PoolRepo)(nil)

// NewPoolRepo creates a PoolRepo backed by pool.
func NewPoolRepo(pool *pgxpool.Pool) *PoolRepo {
	return &PoolRepo{pool: pool}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func (r *PoolRepo) List(ctx context.Context, params port.OffsetParams) (*domain.PoolOffsetPage, error) {
	order := "ASC"
	if strings.ToLower(params.Order) == "desc" {
		order = "DESC"
	}
	sortCol := poolSortColumn(params.Sort)

	args := []any{}
	n := 1
	var conditions []string

	if params.Search != "" {
		conditions = append(conditions, fmt.Sprintf("lower(p.name) LIKE lower($%d)", n))
		args = append(args, "%"+params.Search+"%")
		n++
	}
	// Restrict to pools the viewer may see (private-by-default), unless admin.
	if !params.ViewerIsAdmin {
		var aclCond string
		aclCond, n, args = aclVisibilityCond("p", objTypePool, params.ViewerID, n, args)
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
SELECT p.id, p.name, p.notes, p.metadata,
       p.creator_id, u.name AS creator_name, p.is_public,
       p.sort_key, p.sort_order,
       COALESCE(fc.cnt, 0) AS file_count,
       COUNT(*) OVER() AS total
FROM data.pools p
JOIN core.users u ON u.id = p.creator_id
LEFT JOIN %s fc ON fc.pool_id = p.id
%s
ORDER BY %s %s NULLS LAST, p.id ASC
LIMIT $%d OFFSET $%d`, poolCountSubquery, where, sortCol, order, n, n+1)

	args = append(args, limit, offset)

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("PoolRepo.List query: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[poolRowWithTotal])
	if err != nil {
		return nil, fmt.Errorf("PoolRepo.List scan: %w", err)
	}

	items := make([]domain.Pool, len(collected))
	total := 0
	for i, row := range collected {
		items[i] = toPool(row.poolRow)
		total = row.Total
	}
	return &domain.PoolOffsetPage{
		Items:  items,
		Total:  total,
		Offset: offset,
		Limit:  limit,
	}, nil
}

// ---------------------------------------------------------------------------
// GetByID
// ---------------------------------------------------------------------------

func (r *PoolRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Pool, error) {
	query := poolSelectFrom + `
WHERE p.id = $1`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("PoolRepo.GetByID: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[poolRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("PoolRepo.GetByID scan: %w", err)
	}
	p := toPool(row)
	return &p, nil
}

// RecordView appends a row to activity.pool_views. viewed_at defaults to
// statement_timestamp(), so each call records a distinct view in the history.
func (r *PoolRepo) RecordView(ctx context.Context, poolID uuid.UUID, userID int16) error {
	const query = `INSERT INTO activity.pool_views (pool_id, user_id) VALUES ($1, $2)`
	if _, err := connOrTx(ctx, r.pool).Exec(ctx, query, poolID, userID); err != nil {
		return fmt.Errorf("PoolRepo.RecordView: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func (r *PoolRepo) Create(ctx context.Context, p *domain.Pool) (*domain.Pool, error) {
	const query = `
WITH ins AS (
    INSERT INTO data.pools (name, notes, metadata, creator_id, is_public)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING *
)
SELECT ins.id, ins.name, ins.notes, ins.metadata,
       ins.creator_id, u.name AS creator_name, ins.is_public,
       ins.sort_key, ins.sort_order,
       0 AS file_count
FROM ins
JOIN core.users u ON u.id = ins.creator_id`

	var meta any
	if len(p.Metadata) > 0 {
		meta = p.Metadata
	}

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, query, p.Name, p.Notes, meta, p.CreatorID, p.IsPublic)
	if err != nil {
		return nil, fmt.Errorf("PoolRepo.Create: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[poolRow])
	if err != nil {
		if isPgUniqueViolation(err) {
			return nil, domain.ErrConflict
		}
		return nil, fmt.Errorf("PoolRepo.Create scan: %w", err)
	}
	created := toPool(row)
	return &created, nil
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (r *PoolRepo) Update(ctx context.Context, id uuid.UUID, p *domain.Pool) (*domain.Pool, error) {
	const query = `
WITH upd AS (
    UPDATE data.pools SET
        name       = $2,
        notes      = $3,
        metadata   = COALESCE($4, metadata),
        is_public  = $5,
        sort_key   = $6,
        sort_order = $7
    WHERE id = $1
    RETURNING *
)
SELECT upd.id, upd.name, upd.notes, upd.metadata,
       upd.creator_id, u.name AS creator_name, upd.is_public,
       upd.sort_key, upd.sort_order,
       COALESCE(fc.cnt, 0) AS file_count
FROM upd
JOIN core.users u ON u.id = upd.creator_id
LEFT JOIN (SELECT pool_id, COUNT(*) AS cnt FROM data.file_pool WHERE pool_id = $1 GROUP BY pool_id) fc
    ON fc.pool_id = upd.id`

	var meta any
	if len(p.Metadata) > 0 {
		meta = p.Metadata
	}

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, query, id, p.Name, p.Notes, meta, p.IsPublic, p.SortKey, p.SortOrder)
	if err != nil {
		return nil, fmt.Errorf("PoolRepo.Update: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[poolRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		if isPgUniqueViolation(err) {
			return nil, domain.ErrConflict
		}
		return nil, fmt.Errorf("PoolRepo.Update scan: %w", err)
	}
	updated := toPool(row)
	return &updated, nil
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func (r *PoolRepo) Delete(ctx context.Context, id uuid.UUID) error {
	const query = `DELETE FROM data.pools WHERE id = $1`
	q := connOrTx(ctx, r.pool)
	ct, err := q.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("PoolRepo.Delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// ListFiles
// ---------------------------------------------------------------------------

// fileSelectForPool is the column list for pool file queries (without position).
const fileSelectForPool = `
    f.id, f.original_name,
    mt.name AS mime_type, mt.extension AS mime_extension,
    f.content_datetime, f.notes, f.metadata, f.exif, f.phash,
    f.creator_id, u.name AS creator_name,
    f.is_public, f.is_deleted`

func (r *PoolRepo) ListFiles(ctx context.Context, poolID uuid.UUID, params port.PoolFileListParams) (*domain.PoolFilePage, error) {
	limit := db.ClampLimit(params.Limit, 50, 200)

	args := []any{poolID}
	n := 2
	var conds []string

	conds = append(conds, "fp.pool_id = $1")
	conds = append(conds, "f.is_deleted = false")

	if params.Filter != "" {
		filterSQL, nextN, filterArgs, err := ParseFilter(params.Filter, n)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", domain.ErrValidation, err)
		}
		if filterSQL != "" {
			conds = append(conds, filterSQL)
			n = nextN
			args = append(args, filterArgs...)
		}
	}

	// Resolve the pool's sort setting (defaulting to manual position order) into
	// the ORDER BY column, direction, and keyset comparison operator.
	sortKey := params.SortKey
	if !domain.ValidPoolSortKey(sortKey) {
		sortKey = domain.PoolSortManual
	}
	col, dir, cmp := poolFileSort(sortKey, params.SortOrder)

	// Keyset cursor condition. For "created" the file id is both the sort key and
	// the tiebreak, so a single comparison suffices; the others compare (col, id).
	if params.Cursor != "" {
		cur, err := decodePoolCursor(params.Cursor)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", domain.ErrValidation, err)
		}
		fileID, err := uuid.Parse(cur.FileID)
		if err != nil {
			return nil, domain.ErrValidation
		}
		if sortKey == domain.PoolSortCreated {
			conds = append(conds, fmt.Sprintf("f.id %s $%d", cmp, n))
			args = append(args, fileID)
			n++
		} else {
			conds = append(conds, fmt.Sprintf(
				"(%s %s $%d OR (%s = $%d AND f.id %s $%d))", col, cmp, n, col, n, cmp, n+1))
			switch sortKey {
			case domain.PoolSortContentDatetime:
				t, err := time.Parse(time.RFC3339Nano, cur.Val)
				if err != nil {
					return nil, domain.ErrValidation
				}
				args = append(args, t)
			case domain.PoolSortOriginalName:
				args = append(args, cur.Val)
			default: // manual
				args = append(args, cur.Pos)
			}
			args = append(args, fileID)
			n += 2
		}
	}

	var orderBy string
	if sortKey == domain.PoolSortCreated {
		orderBy = fmt.Sprintf("f.id %s", dir)
	} else {
		orderBy = fmt.Sprintf("%s %s, f.id %s", col, dir, dir)
	}

	where := "WHERE " + strings.Join(conds, " AND ")
	args = append(args, limit+1)

	sqlStr := fmt.Sprintf(`
SELECT %s, fp.position
FROM data.file_pool fp
JOIN data.files f       ON f.id  = fp.file_id
JOIN core.mime_types mt ON mt.id = f.mime_id
JOIN core.users u       ON u.id  = f.creator_id
%s
ORDER BY %s
LIMIT $%d`, fileSelectForPool, where, orderBy, n)

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("PoolRepo.ListFiles query: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[poolFileRow])
	if err != nil {
		return nil, fmt.Errorf("PoolRepo.ListFiles scan: %w", err)
	}

	hasMore := len(collected) > limit
	if hasMore {
		collected = collected[:limit]
	}

	items := make([]domain.PoolFile, len(collected))
	for i, row := range collected {
		items[i] = toPoolFile(row)
	}

	page := &domain.PoolFilePage{Items: items}

	if hasMore && len(collected) > 0 {
		last := collected[len(collected)-1]
		cursor := poolFileCursor{FileID: last.ID.String()}
		switch sortKey {
		case domain.PoolSortContentDatetime:
			cursor.Val = last.ContentDatetime.UTC().Format(time.RFC3339Nano)
		case domain.PoolSortOriginalName:
			if last.OriginalName != nil {
				cursor.Val = *last.OriginalName
			}
		case domain.PoolSortCreated:
			// file id alone orders; nothing else to carry
		default: // manual
			cursor.Pos = last.Position
		}
		enc := encodePoolCursor(cursor)
		page.NextCursor = &enc
	}

	// Batch-load tags.
	if len(items) > 0 {
		fileIDs := make([]uuid.UUID, len(items))
		for i, pf := range items {
			fileIDs[i] = pf.File.ID
		}
		tagMap, err := r.loadPoolTagsBatch(ctx, fileIDs)
		if err != nil {
			return nil, err
		}
		for i, pf := range items {
			page.Items[i].File.Tags = tagMap[pf.File.ID]
		}
	}

	return page, nil
}

// loadPoolTagsBatch re-uses the same pattern as FileRepo.loadTagsBatch.
func (r *PoolRepo) loadPoolTagsBatch(ctx context.Context, fileIDs []uuid.UUID) (map[uuid.UUID][]domain.Tag, error) {
	if len(fileIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(fileIDs))
	args := make([]any, len(fileIDs))
	for i, id := range fileIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	sqlStr := fmt.Sprintf(`
SELECT ft.file_id,
       t.id, t.name, t.notes, t.color,
       t.category_id,
       c.name  AS category_name,
       c.color AS category_color,
       t.metadata, t.creator_id, u.name AS creator_name, t.is_public
FROM data.file_tag ft
JOIN  data.tags t           ON t.id = ft.tag_id
JOIN  core.users u          ON u.id = t.creator_id
LEFT JOIN data.categories c ON c.id = t.category_id
WHERE ft.file_id IN (%s)
ORDER BY ft.file_id, t.name`, strings.Join(placeholders, ","))

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("PoolRepo.loadPoolTagsBatch: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[fileTagRow])
	if err != nil {
		return nil, fmt.Errorf("PoolRepo.loadPoolTagsBatch scan: %w", err)
	}
	result := make(map[uuid.UUID][]domain.Tag, len(fileIDs))
	for _, fid := range fileIDs {
		result[fid] = []domain.Tag{}
	}
	for _, row := range collected {
		result[row.FileID] = append(result[row.FileID], toTagFromFileTag(row))
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// AddFiles
// ---------------------------------------------------------------------------

// AddFiles inserts files into the pool. When position is nil, files are
// appended after the last existing file (MAX(position) + 1000 * i).
// When position is provided (0-indexed), files are inserted at that index
// and all pool positions are reassigned in one shot.
func (r *PoolRepo) AddFiles(ctx context.Context, poolID uuid.UUID, fileIDs []uuid.UUID, position *int) error {
	if len(fileIDs) == 0 {
		return nil
	}
	q := connOrTx(ctx, r.pool)

	if position == nil {
		// Append: get current max position, then bulk-insert.
		var maxPos int
		row := q.QueryRow(ctx, `SELECT COALESCE(MAX(position), 0) FROM data.file_pool WHERE pool_id = $1`, poolID)
		if err := row.Scan(&maxPos); err != nil {
			return fmt.Errorf("PoolRepo.AddFiles maxPos: %w", err)
		}
		const ins = `INSERT INTO data.file_pool (file_id, pool_id, position) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
		for i, fid := range fileIDs {
			if _, err := q.Exec(ctx, ins, fid, poolID, maxPos+1000*(i+1)); err != nil {
				return fmt.Errorf("PoolRepo.AddFiles insert: %w", err)
			}
		}
		return nil
	}

	// Positional insert: rebuild the full ordered list and reassign.
	return r.insertAtPosition(ctx, q, poolID, fileIDs, *position)
}

// insertAtPosition fetches the current ordered file list, splices in the new
// IDs at index pos (0-indexed, clamped), then does a full position reassign.
func (r *PoolRepo) insertAtPosition(ctx context.Context, q db.Querier, poolID uuid.UUID, newIDs []uuid.UUID, pos int) error {
	// 1. Fetch current order.
	rows, err := q.Query(ctx, `SELECT file_id FROM data.file_pool WHERE pool_id = $1 ORDER BY position ASC, file_id ASC`, poolID)
	if err != nil {
		return fmt.Errorf("PoolRepo.insertAtPosition fetch: %w", err)
	}
	var current []uuid.UUID
	for rows.Next() {
		var fid uuid.UUID
		if err := rows.Scan(&fid); err != nil {
			rows.Close()
			return fmt.Errorf("PoolRepo.insertAtPosition scan: %w", err)
		}
		current = append(current, fid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("PoolRepo.insertAtPosition rows: %w", err)
	}

	// 2. Build new ordered list, skipping already-present IDs from newIDs.
	present := make(map[uuid.UUID]bool, len(current))
	for _, fid := range current {
		present[fid] = true
	}
	toAdd := make([]uuid.UUID, 0, len(newIDs))
	for _, fid := range newIDs {
		if !present[fid] {
			toAdd = append(toAdd, fid)
		}
	}
	if len(toAdd) == 0 {
		return nil // all already present
	}

	if pos < 0 {
		pos = 0
	}
	if pos > len(current) {
		pos = len(current)
	}

	ordered := make([]uuid.UUID, 0, len(current)+len(toAdd))
	ordered = append(ordered, current[:pos]...)
	ordered = append(ordered, toAdd...)
	ordered = append(ordered, current[pos:]...)

	// 3. Full replace.
	return r.reassignPositions(ctx, q, poolID, ordered)
}

// reassignPositions does a DELETE + bulk INSERT for the pool with positions
// 1000, 2000, 3000, ...
func (r *PoolRepo) reassignPositions(ctx context.Context, q db.Querier, poolID uuid.UUID, ordered []uuid.UUID) error {
	if _, err := q.Exec(ctx, `DELETE FROM data.file_pool WHERE pool_id = $1`, poolID); err != nil {
		return fmt.Errorf("PoolRepo.reassignPositions delete: %w", err)
	}
	if len(ordered) == 0 {
		return nil
	}
	const ins = `INSERT INTO data.file_pool (file_id, pool_id, position) VALUES ($1, $2, $3)`
	for i, fid := range ordered {
		if _, err := q.Exec(ctx, ins, fid, poolID, 1000*(i+1)); err != nil {
			return fmt.Errorf("PoolRepo.reassignPositions insert: %w", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// RemoveFiles
// ---------------------------------------------------------------------------

func (r *PoolRepo) RemoveFiles(ctx context.Context, poolID uuid.UUID, fileIDs []uuid.UUID) error {
	if len(fileIDs) == 0 {
		return nil
	}
	placeholders := make([]string, len(fileIDs))
	args := make([]any, len(fileIDs)+1)
	args[0] = poolID
	for i, fid := range fileIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = fid
	}
	query := fmt.Sprintf(
		`DELETE FROM data.file_pool WHERE pool_id = $1 AND file_id IN (%s)`,
		strings.Join(placeholders, ","))

	q := connOrTx(ctx, r.pool)
	if _, err := q.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("PoolRepo.RemoveFiles: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Reorder
// ---------------------------------------------------------------------------

// Reorder applies the requested order to the pool. Files actually in the pool
// are placed in the given order; any pool members the request omitted are kept
// and appended in their current order. This makes a partial request (e.g. a
// paginated client that only loaded the first pages) reorder the visible prefix
// without deleting the rest. Unknown IDs are ignored.
func (r *PoolRepo) Reorder(ctx context.Context, poolID uuid.UUID, fileIDs []uuid.UUID) error {
	q := connOrTx(ctx, r.pool)

	// Current membership, in position order.
	rows, err := q.Query(ctx,
		`SELECT file_id FROM data.file_pool WHERE pool_id = $1 ORDER BY position ASC, file_id ASC`, poolID)
	if err != nil {
		return fmt.Errorf("PoolRepo.Reorder fetch: %w", err)
	}
	var current []uuid.UUID
	for rows.Next() {
		var fid uuid.UUID
		if err := rows.Scan(&fid); err != nil {
			rows.Close()
			return fmt.Errorf("PoolRepo.Reorder scan: %w", err)
		}
		current = append(current, fid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("PoolRepo.Reorder rows: %w", err)
	}

	inPool := make(map[uuid.UUID]bool, len(current))
	for _, fid := range current {
		inPool[fid] = true
	}

	ordered := make([]uuid.UUID, 0, len(current))
	placed := make(map[uuid.UUID]bool, len(current))
	for _, fid := range fileIDs {
		if inPool[fid] && !placed[fid] {
			ordered = append(ordered, fid)
			placed[fid] = true
		}
	}
	// Preserve any members the request did not mention.
	for _, fid := range current {
		if !placed[fid] {
			ordered = append(ordered, fid)
			placed[fid] = true
		}
	}

	return r.reassignPositions(ctx, q, poolID, ordered)
}
