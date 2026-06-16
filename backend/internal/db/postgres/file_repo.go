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

type fileRow struct {
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
	NeedsReview     bool            `db:"needs_review"`
}

// fileTagRow is used for both single-file and batch tag loading.
// file_id is always selected so the same struct works for both cases.
type fileTagRow struct {
	FileID        uuid.UUID       `db:"file_id"`
	ID            uuid.UUID       `db:"id"`
	Name          string          `db:"name"`
	Notes         *string         `db:"notes"`
	Color         *string         `db:"color"`
	CategoryID    *uuid.UUID      `db:"category_id"`
	CategoryName  *string         `db:"category_name"`
	CategoryColor *string         `db:"category_color"`
	Metadata      json.RawMessage `db:"metadata"`
	CreatorID     int16           `db:"creator_id"`
	CreatorName   string          `db:"creator_name"`
	IsPublic      bool            `db:"is_public"`
}

// anchorValRow holds the sort-column values fetched for an anchor file.
type anchorValRow struct {
	ContentDatetime time.Time `db:"content_datetime"`
	OriginalName    string    `db:"original_name"` // COALESCE(original_name,'') applied in SQL
	MIMEType        string    `db:"mime_type"`
}

// ---------------------------------------------------------------------------
// Converters
// ---------------------------------------------------------------------------

func toFile(r fileRow) domain.File {
	return domain.File{
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
		NeedsReview:     r.NeedsReview,
		CreatedAt:       domain.UUIDCreatedAt(r.ID),
	}
}

func toTagFromFileTag(r fileTagRow) domain.Tag {
	return domain.Tag{
		ID:            r.ID,
		Name:          r.Name,
		Notes:         r.Notes,
		Color:         r.Color,
		CategoryID:    r.CategoryID,
		CategoryName:  r.CategoryName,
		CategoryColor: r.CategoryColor,
		Metadata:      r.Metadata,
		CreatorID:     r.CreatorID,
		CreatorName:   r.CreatorName,
		IsPublic:      r.IsPublic,
		CreatedAt:     domain.UUIDCreatedAt(r.ID),
	}
}

// ---------------------------------------------------------------------------
// Cursor
// ---------------------------------------------------------------------------

type fileCursor struct {
	Sort  string `json:"s"`  // canonical sort name
	Order string `json:"o"`  // "ASC" or "DESC"
	ID    string `json:"id"` // UUID of the boundary file
	Val   string `json:"v"`  // sort column value; empty for "created" (id IS the key)
}

func encodeCursor(c fileCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (fileCursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return fileCursor{}, fmt.Errorf("cursor: invalid encoding")
	}
	var c fileCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return fileCursor{}, fmt.Errorf("cursor: invalid format")
	}
	return c, nil
}

// makeCursor builds a fileCursor from a boundary row and the current sort/order.
func makeCursor(r fileRow, sort, order string) fileCursor {
	var val string
	switch sort {
	case "content_datetime":
		val = r.ContentDatetime.UTC().Format(time.RFC3339Nano)
	case "original_name":
		if r.OriginalName != nil {
			val = *r.OriginalName
		}
	case "mime":
		val = r.MIMEType
		// "created": val is empty; f.id is the sort key.
	}
	return fileCursor{Sort: sort, Order: order, ID: r.ID.String(), Val: val}
}

// ---------------------------------------------------------------------------
// Sort helpers
// ---------------------------------------------------------------------------

func normSort(s string) string {
	switch s {
	case "content_datetime", "original_name", "mime":
		return s
	default:
		return "created"
	}
}

func normOrder(o string) string {
	if strings.EqualFold(o, "asc") {
		return "ASC"
	}
	return "DESC"
}

// buildKeysetCond returns a keyset WHERE fragment and an ORDER BY fragment.
//
//   - forward=true:  items after the cursor in the sort order (standard next-page)
//   - forward=false: items before the cursor (previous-page); ORDER BY is reversed,
//     caller must reverse the result slice after fetching
//   - incl=true: include the cursor file itself (anchor case; uses ≤ / ≥)
//
// All user values are bound as parameters — no SQL injection possible.
func buildKeysetCond(
	sort, order string,
	forward, incl bool,
	cursorID uuid.UUID, cursorVal string,
	n int, args []any,
) (where, orderBy string, nextN int, outArgs []any) {
	// goDown=true → want smaller values → primary comparison is "<".
	// Applies for DESC+forward and ASC+backward.
	goDown := (order == "DESC") == forward

	var op, idOp string
	if goDown {
		op = "<"
		if incl {
			idOp = "<="
		} else {
			idOp = "<"
		}
	} else {
		op = ">"
		if incl {
			idOp = ">="
		} else {
			idOp = ">"
		}
	}

	// Effective ORDER BY direction: reversed for backward so the DB returns
	// the closest items first (the ones we keep after trimming the extra).
	dir := order
	if !forward {
		if order == "DESC" {
			dir = "ASC"
		} else {
			dir = "DESC"
		}
	}

	switch sort {
	case "created":
		// Single-column keyset: f.id (UUID v7, so ordering = chronological).
		where = fmt.Sprintf("f.id %s $%d", idOp, n)
		orderBy = fmt.Sprintf("f.id %s", dir)
		outArgs = append(args, cursorID)
		n++

	case "content_datetime":
		// Two-column keyset: (content_datetime, id).
		// $n is referenced twice in the SQL (< and =) but passed once in args —
		// PostgreSQL extended protocol allows multiple references to $N.
		t, _ := time.Parse(time.RFC3339Nano, cursorVal)
		where = fmt.Sprintf(
			"(f.content_datetime %s $%d OR (f.content_datetime = $%d AND f.id %s $%d))",
			op, n, n, idOp, n+1)
		orderBy = fmt.Sprintf("f.content_datetime %s, f.id %s", dir, dir)
		outArgs = append(args, t, cursorID)
		n += 2

	case "original_name":
		// COALESCE treats NULL names as '' for stable pagination.
		where = fmt.Sprintf(
			"(COALESCE(f.original_name,'') %s $%d OR (COALESCE(f.original_name,'') = $%d AND f.id %s $%d))",
			op, n, n, idOp, n+1)
		orderBy = fmt.Sprintf("COALESCE(f.original_name,'') %s, f.id %s", dir, dir)
		outArgs = append(args, cursorVal, cursorID)
		n += 2

	default: // "mime"
		where = fmt.Sprintf(
			"(mt.name %s $%d OR (mt.name = $%d AND f.id %s $%d))",
			op, n, n, idOp, n+1)
		orderBy = fmt.Sprintf("mt.name %s, f.id %s", dir, dir)
		outArgs = append(args, cursorVal, cursorID)
		n += 2
	}

	nextN = n
	return
}

// defaultOrderBy returns the natural ORDER BY for the first page (no cursor).
func defaultOrderBy(sort, order string) string {
	switch sort {
	case "created":
		return fmt.Sprintf("f.id %s", order)
	case "content_datetime":
		return fmt.Sprintf("f.content_datetime %s, f.id %s", order, order)
	case "original_name":
		return fmt.Sprintf("COALESCE(f.original_name,'') %s, f.id %s", order, order)
	default: // "mime"
		return fmt.Sprintf("mt.name %s, f.id %s", order, order)
	}
}

// ---------------------------------------------------------------------------
// FileRepo
// ---------------------------------------------------------------------------

// FileRepo implements port.FileRepo using PostgreSQL.
type FileRepo struct {
	pool *pgxpool.Pool
}

// NewFileRepo creates a FileRepo backed by pool.
func NewFileRepo(pool *pgxpool.Pool) *FileRepo {
	return &FileRepo{pool: pool}
}

var _ port.FileRepo = (*FileRepo)(nil)

// fileSelectCTE is the SELECT appended after a CTE named "r" that exposes
// all file columns (including mime_id). Used by Create, Update, and Restore
// to get the full denormalized record in a single round-trip.
const fileSelectCTE = `
    SELECT r.id, r.original_name,
           mt.name AS mime_type, mt.extension AS mime_extension,
           r.content_datetime, r.notes, r.metadata, r.exif, r.phash,
           r.creator_id, u.name AS creator_name,
           r.is_public, r.is_deleted, r.needs_review
    FROM r
    JOIN core.mime_types mt ON mt.id = r.mime_id
    JOIN core.users u       ON u.id  = r.creator_id`

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

// Create inserts a new file record using the ID already set on f.
// The MIME type is resolved from f.MIMEType (name string) via a subquery.
func (r *FileRepo) Create(ctx context.Context, f *domain.File) (*domain.File, error) {
	const sqlStr = `
        WITH r AS (
            INSERT INTO data.files
                (id, original_name, mime_id, content_datetime, notes, metadata, exif, phash, creator_id, is_public)
            VALUES (
                $1,
                $2,
                (SELECT id FROM core.mime_types WHERE name = $3),
                $4, $5, $6, $7, $8, $9, $10
            )
            RETURNING id, original_name, mime_id, content_datetime, notes,
                      metadata, exif, phash, creator_id, is_public, is_deleted,
                      needs_review
        )` + fileSelectCTE

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sqlStr,
		f.ID, f.OriginalName, f.MIMEType, f.ContentDatetime,
		f.Notes, f.Metadata, f.EXIF, f.PHash,
		f.CreatorID, f.IsPublic,
	)
	if err != nil {
		return nil, fmt.Errorf("FileRepo.Create: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[fileRow])
	if err != nil {
		return nil, fmt.Errorf("FileRepo.Create scan: %w", err)
	}
	created := toFile(row)
	return &created, nil
}

// ---------------------------------------------------------------------------
// GetByID
// ---------------------------------------------------------------------------

func (r *FileRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.File, error) {
	const sqlStr = `
        SELECT f.id, f.original_name,
               mt.name AS mime_type, mt.extension AS mime_extension,
               f.content_datetime, f.notes, f.metadata, f.exif, f.phash,
               f.creator_id, u.name AS creator_name,
               f.is_public, f.is_deleted, f.needs_review
        FROM data.files f
        JOIN core.mime_types mt ON mt.id = f.mime_id
        JOIN core.users u       ON u.id  = f.creator_id
        WHERE f.id = $1`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sqlStr, id)
	if err != nil {
		return nil, fmt.Errorf("FileRepo.GetByID: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[fileRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("FileRepo.GetByID scan: %w", err)
	}
	f := toFile(row)
	tags, err := r.ListTags(ctx, id)
	if err != nil {
		return nil, err
	}
	f.Tags = tags
	return &f, nil
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update applies editable metadata fields. MIME type and EXIF are immutable.
func (r *FileRepo) Update(ctx context.Context, id uuid.UUID, f *domain.File) (*domain.File, error) {
	const sqlStr = `
        WITH r AS (
            UPDATE data.files
            SET original_name    = $2,
                content_datetime = $3,
                notes            = $4,
                metadata         = $5,
                is_public        = $6
            WHERE id = $1
            RETURNING id, original_name, mime_id, content_datetime, notes,
                      metadata, exif, phash, creator_id, is_public, is_deleted,
                      needs_review
        )` + fileSelectCTE

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sqlStr,
		id, f.OriginalName, f.ContentDatetime,
		f.Notes, f.Metadata, f.IsPublic,
	)
	if err != nil {
		return nil, fmt.Errorf("FileRepo.Update: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[fileRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("FileRepo.Update scan: %w", err)
	}
	updated := toFile(row)
	tags, err := r.ListTags(ctx, id)
	if err != nil {
		return nil, err
	}
	updated.Tags = tags
	return &updated, nil
}

// SetNeedsReview sets the review status on the given files in one statement.
// Trashed files are left untouched. No-op for an empty id list.
func (r *FileRepo) SetNeedsReview(ctx context.Context, ids []uuid.UUID, value bool) error {
	if len(ids) == 0 {
		return nil
	}
	const sqlStr = `UPDATE data.files SET needs_review = $2 WHERE id = ANY($1) AND is_deleted = false`
	q := connOrTx(ctx, r.pool)
	if _, err := q.Exec(ctx, sqlStr, ids, value); err != nil {
		return fmt.Errorf("FileRepo.SetNeedsReview: %w", err)
	}
	return nil
}

// SetPHash sets (or clears, when phash is nil) the perceptual hash of a file.
// Used by the dedup backfill and on content replacement; phash is non-critical,
// recomputable metadata, so callers may treat failures as best-effort.
func (r *FileRepo) SetPHash(ctx context.Context, id uuid.UUID, phash *int64) error {
	const sqlStr = `UPDATE data.files SET phash = $2 WHERE id = $1`
	q := connOrTx(ctx, r.pool)
	if _, err := q.Exec(ctx, sqlStr, id, phash); err != nil {
		return fmt.Errorf("FileRepo.SetPHash: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// SoftDelete / Restore / DeletePermanent
// ---------------------------------------------------------------------------

// SoftDelete moves a file to trash (is_deleted = true). Returns ErrNotFound
// if the file does not exist or is already in trash.
func (r *FileRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	const sqlStr = `UPDATE data.files SET is_deleted = true WHERE id = $1 AND is_deleted = false`
	q := connOrTx(ctx, r.pool)
	tag, err := q.Exec(ctx, sqlStr, id)
	if err != nil {
		return fmt.Errorf("FileRepo.SoftDelete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Restore moves a file out of trash (is_deleted = false). Returns ErrNotFound
// if the file does not exist or is not in trash.
func (r *FileRepo) Restore(ctx context.Context, id uuid.UUID) (*domain.File, error) {
	const sqlStr = `
        WITH r AS (
            UPDATE data.files
            SET is_deleted = false
            WHERE id = $1 AND is_deleted = true
            RETURNING id, original_name, mime_id, content_datetime, notes,
                      metadata, exif, phash, creator_id, is_public, is_deleted,
                      needs_review
        )` + fileSelectCTE

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sqlStr, id)
	if err != nil {
		return nil, fmt.Errorf("FileRepo.Restore: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[fileRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("FileRepo.Restore scan: %w", err)
	}
	restored := toFile(row)
	tags, err := r.ListTags(ctx, id)
	if err != nil {
		return nil, err
	}
	restored.Tags = tags
	return &restored, nil
}

// DeletePermanent removes a file record permanently. Only allowed when the
// file is already in trash (is_deleted = true).
func (r *FileRepo) DeletePermanent(ctx context.Context, id uuid.UUID) error {
	const sqlStr = `DELETE FROM data.files WHERE id = $1 AND is_deleted = true`
	q := connOrTx(ctx, r.pool)
	tag, err := q.Exec(ctx, sqlStr, id)
	if err != nil {
		return fmt.Errorf("FileRepo.DeletePermanent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// ListTags / SetTags
// ---------------------------------------------------------------------------

// ListTags returns all tags assigned to a file, ordered by tag name.
func (r *FileRepo) ListTags(ctx context.Context, fileID uuid.UUID) ([]domain.Tag, error) {
	m, err := r.loadTagsBatch(ctx, []uuid.UUID{fileID})
	if err != nil {
		return nil, err
	}
	return m[fileID], nil
}

// SetTags replaces all tags on a file (full replace semantics).
func (r *FileRepo) SetTags(ctx context.Context, fileID uuid.UUID, tagIDs []uuid.UUID) error {
	q := connOrTx(ctx, r.pool)
	const del = `DELETE FROM data.file_tag WHERE file_id = $1`
	if _, err := q.Exec(ctx, del, fileID); err != nil {
		return fmt.Errorf("FileRepo.SetTags delete: %w", err)
	}
	if len(tagIDs) == 0 {
		return nil
	}
	const ins = `INSERT INTO data.file_tag (file_id, tag_id) VALUES ($1, $2)`
	for _, tagID := range tagIDs {
		if _, err := q.Exec(ctx, ins, fileID, tagID); err != nil {
			return fmt.Errorf("FileRepo.SetTags insert: %w", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

// List returns a cursor-paginated page of files.
//
// Pagination is keyset-based for stable performance on large tables.
// Cursor encodes the sort position; the caller provides direction.
// Anchor mode centres the result around a specific file UUID.
func (r *FileRepo) List(ctx context.Context, params domain.FileListParams) (*domain.FilePage, error) {
	sort := normSort(params.Sort)
	order := normOrder(params.Order)
	forward := params.Direction != "backward"
	limit := db.ClampLimit(params.Limit, 50, 200)

	// --- resolve cursor / anchor ---
	var (
		cursorID  uuid.UUID
		cursorVal string
		hasCursor bool
		isAnchor  bool
	)

	switch {
	case params.Cursor != "":
		cur, err := decodeCursor(params.Cursor)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", domain.ErrValidation, err)
		}
		id, err := uuid.Parse(cur.ID)
		if err != nil {
			return nil, domain.ErrValidation
		}
		// Lock in the sort/order encoded in the cursor so changing query
		// parameters mid-session doesn't corrupt pagination.
		sort = normSort(cur.Sort)
		order = normOrder(cur.Order)
		cursorID = id
		cursorVal = cur.Val
		hasCursor = true

	case params.Anchor != nil:
		av, err := r.fetchAnchorVals(ctx, *params.Anchor)
		if err != nil {
			return nil, err
		}
		cursorID = *params.Anchor
		switch sort {
		case "content_datetime":
			cursorVal = av.ContentDatetime.UTC().Format(time.RFC3339Nano)
		case "original_name":
			cursorVal = av.OriginalName
		case "mime":
			cursorVal = av.MIMEType
			// "created": cursorVal stays ""; cursorID is the sort key.
		}
		hasCursor = true
		isAnchor = true
	}

	// Without a cursor there is no meaningful "backward" direction.
	if !hasCursor {
		forward = true
	}

	// --- build WHERE and ORDER BY ---
	var conds []string
	args := make([]any, 0, 8)
	n := 1

	conds = append(conds, fmt.Sprintf("f.is_deleted = $%d", n))
	args = append(args, params.Trash)
	n++

	if params.Search != "" {
		conds = append(conds, fmt.Sprintf("f.original_name ILIKE $%d", n))
		args = append(args, "%"+params.Search+"%")
		n++
	}

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

	// Restrict to files the viewer may see (private-by-default), unless admin.
	if !params.ViewerIsAdmin {
		var aclCond string
		aclCond, n, args = aclVisibilityCond("f", objTypeFile, params.ViewerID, n, args)
		conds = append(conds, aclCond)
	}

	var orderBy string
	if hasCursor {
		ksWhere, ksOrder, nextN, ksArgs := buildKeysetCond(
			sort, order, forward, isAnchor, cursorID, cursorVal, n, args)
		conds = append(conds, ksWhere)
		n = nextN
		args = ksArgs
		orderBy = ksOrder
	} else {
		orderBy = defaultOrderBy(sort, order)
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	// Fetch one extra row to detect whether more items exist beyond this page.
	args = append(args, limit+1)
	sqlStr := fmt.Sprintf(`
        SELECT f.id, f.original_name,
               mt.name AS mime_type, mt.extension AS mime_extension,
               f.content_datetime, f.notes, f.metadata, f.exif, f.phash,
               f.creator_id, u.name AS creator_name,
               f.is_public, f.is_deleted, f.needs_review
        FROM data.files f
        JOIN core.mime_types mt ON mt.id = f.mime_id
        JOIN core.users u       ON u.id  = f.creator_id
        %s
        ORDER BY %s
        LIMIT $%d`, where, orderBy, n)

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("FileRepo.List: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[fileRow])
	if err != nil {
		return nil, fmt.Errorf("FileRepo.List scan: %w", err)
	}

	// --- trim extra row and reverse for backward ---
	hasMore := len(collected) > limit
	if hasMore {
		collected = collected[:limit]
	}
	if !forward {
		// Results were fetched in reversed ORDER BY; invert to restore the
		// natural sort order expected by the caller.
		for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
			collected[i], collected[j] = collected[j], collected[i]
		}
	}

	// --- assemble page ---
	page := &domain.FilePage{
		Items: make([]domain.File, len(collected)),
	}
	for i, row := range collected {
		page.Items[i] = toFile(row)
	}

	// --- set next/prev cursors ---
	// next_cursor: navigate further in the forward direction.
	// prev_cursor: navigate further in the backward direction.
	if len(collected) > 0 {
		firstCur := encodeCursor(makeCursor(collected[0], sort, order))
		lastCur := encodeCursor(makeCursor(collected[len(collected)-1], sort, order))

		if forward {
			// We only know a prev page exists if we arrived via cursor.
			if hasCursor {
				page.PrevCursor = &firstCur
			}
			if hasMore {
				page.NextCursor = &lastCur
			}
		} else {
			// Backward: last item (after reversal) is closest to original cursor.
			if hasCursor {
				page.NextCursor = &lastCur
			}
			if hasMore {
				page.PrevCursor = &firstCur
			}
		}
	}

	// --- batch-load tags ---
	if len(page.Items) > 0 {
		fileIDs := make([]uuid.UUID, len(page.Items))
		for i, f := range page.Items {
			fileIDs[i] = f.ID
		}
		tagMap, err := r.loadTagsBatch(ctx, fileIDs)
		if err != nil {
			return nil, err
		}
		for i, f := range page.Items {
			page.Items[i].Tags = tagMap[f.ID] // nil becomes []domain.Tag{} via loadTagsBatch
		}
	}

	return page, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// fetchAnchorVals returns the sort-column values for the given file.
// Used to set up a cursor when the caller provides an anchor UUID.
func (r *FileRepo) fetchAnchorVals(ctx context.Context, fileID uuid.UUID) (*anchorValRow, error) {
	const sqlStr = `
        SELECT f.content_datetime,
               COALESCE(f.original_name, '') AS original_name,
               mt.name AS mime_type
        FROM data.files f
        JOIN core.mime_types mt ON mt.id = f.mime_id
        WHERE f.id = $1`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sqlStr, fileID)
	if err != nil {
		return nil, fmt.Errorf("FileRepo.fetchAnchorVals: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[anchorValRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("FileRepo.fetchAnchorVals scan: %w", err)
	}
	return &row, nil
}

// loadTagsBatch fetches tags for multiple files in a single query and returns
// them as a map keyed by file ID. Every requested file ID appears as a key
// (with an empty slice if the file has no tags).
func (r *FileRepo) loadTagsBatch(ctx context.Context, fileIDs []uuid.UUID) (map[uuid.UUID][]domain.Tag, error) {
	if len(fileIDs) == 0 {
		return nil, nil
	}

	// Build a parameterised IN list. The max page size is 200, so at most 200
	// placeholders — well within PostgreSQL's limits.
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
		return nil, fmt.Errorf("FileRepo.loadTagsBatch: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[fileTagRow])
	if err != nil {
		return nil, fmt.Errorf("FileRepo.loadTagsBatch scan: %w", err)
	}

	result := make(map[uuid.UUID][]domain.Tag, len(fileIDs))
	for _, fid := range fileIDs {
		result[fid] = []domain.Tag{} // guarantee every key has a non-nil slice
	}
	for _, row := range collected {
		result[row.FileID] = append(result[row.FileID], toTagFromFileTag(row))
	}
	return result, nil
}

// RecordView appends a row to activity.file_views. viewed_at defaults to
// statement_timestamp(), so each call records a distinct view in the history.
func (r *FileRepo) RecordView(ctx context.Context, fileID uuid.UUID, userID int16) error {
	const query = `INSERT INTO activity.file_views (file_id, user_id) VALUES ($1, $2)`
	if _, err := connOrTx(ctx, r.pool).Exec(ctx, query, fileID, userID); err != nil {
		return fmt.Errorf("FileRepo.RecordView: %w", err)
	}
	return nil
}

// RecordTagUses appends a row to activity.tag_uses for each tag referenced in a
// filter DSL, flagging it included (positive) or excluded (negated). Tags are
// deduplicated per call, so one statement_timestamp() never collides on the
// (tag_id, used_at, user_id) PK; ON CONFLICT DO NOTHING guards the rest. A
// filter with no tag terms is a no-op.
func (r *FileRepo) RecordTagUses(ctx context.Context, userID int16, filterDSL string) error {
	uses := filterTagUses(filterDSL)
	if len(uses) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("INSERT INTO activity.tag_uses (tag_id, user_id, is_included) VALUES ")
	args := make([]any, 0, len(uses)*3)
	for i, u := range uses {
		if i > 0 {
			sb.WriteString(", ")
		}
		base := i * 3
		fmt.Fprintf(&sb, "($%d, $%d, $%d)", base+1, base+2, base+3)
		args = append(args, u.tagID, userID, u.included)
	}
	sb.WriteString(" ON CONFLICT DO NOTHING")

	if _, err := connOrTx(ctx, r.pool).Exec(ctx, sb.String(), args...); err != nil {
		return fmt.Errorf("FileRepo.RecordTagUses: %w", err)
	}
	return nil
}
