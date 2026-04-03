package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tanabata/internal/domain"
)

type FileStore struct {
	db *pgxpool.Pool
}

func NewFileStore(db *pgxpool.Pool) *FileStore {
	return &FileStore{db: db}
}

// Get user's access rights to file
func (s *FileStore) getAccess(user_id int, file_id string) (canView, canEdit bool, err error) {
	ctx := context.Background()
	row := connPool.QueryRow(ctx, `
		SELECT 
			COALESCE(a.view, FALSE) OR f.creator_id=$1 OR COALESCE(u.is_admin, FALSE),
			COALESCE(a.edit, FALSE) OR f.creator_id=$1 OR COALESCE(u.is_admin, FALSE)
		FROM data.files f
		LEFT JOIN acl.files a ON a.file_id=f.id AND a.user_id=$1
		LEFT JOIN system.users u ON u.id=$1
		WHERE f.id=$2
		`, user_id, file_id)
	err = row.Scan(&canView, &canEdit)
	return
}

// Get a set of files
func (s *FileStore) GetSlice(user_id int, filter, sort string, limit, offset int) (files domain.Slice[domain.FileItem], statusCode int, err error) {
	filterCond, statusCode, err := filterToSQL(filter)
	if err != nil {
		return
	}
	sortExpr, statusCode, err := sortToSQL(sort)
	if err != nil {
		return
	}
	// prepare query
	query := `
	SELECT
		f.id,
		f.name,
		m.name,
		m.extension,
		uuid_extract_timestamp(f.id),
		u.name,
		u.is_admin
	FROM data.files f
	JOIN system.mime m ON m.id=f.mime_id
	JOIN system.users u ON u.id=f.creator_id
	WHERE NOT f.is_deleted AND (f.creator_id=$1 OR (SELECT view FROM acl.files WHERE file_id=f.id AND user_id=$1) OR (SELECT is_admin FROM system.users WHERE id=$1)) AND
	`
	query += filterCond
	queryCount := query
	query += sortExpr
	if limit >= 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", offset)
	}
	// execute query
	statusCode, err = transaction(func(ctx context.Context, tx pgx.Tx) (statusCode int, err error) {
		rows, err := tx.Query(ctx, query, user_id)
		if err != nil {
			statusCode, err = handleDBError(err)
			return
		}
		defer rows.Close()
		count := 0
		for rows.Next() {
			var file domain.FileItem
			err = rows.Scan(&file.ID, &file.Name, &file.MIME.Name, &file.MIME.Extension, &file.CreatedAt, &file.Creator.Name, &file.Creator.IsAdmin)
			if err != nil {
				statusCode = http.StatusInternalServerError
				return
			}
			files.Data = append(files.Data, file)
			count++
		}
		err = rows.Err()
		if err != nil {
			statusCode = http.StatusInternalServerError
			return
		}
		files.Pagination.Limit = limit
		files.Pagination.Offset = offset
		files.Pagination.Count = count
		row := tx.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM (%s) tmp", queryCount), user_id)
		err = row.Scan(&files.Pagination.Total)
		if err != nil {
			statusCode = http.StatusInternalServerError
		}
		return
	})
	if err == nil {
		statusCode = http.StatusOK
	}
	return
}

// Get file
func (s *FileStore) Get(user_id int, file_id string) (file domain.FileFull, statusCode int, err error) {
	ctx := context.Background()
	row := connPool.QueryRow(ctx, `
		SELECT
			f.id,
			f.name,
			m.name,
			m.extension,
			uuid_extract_timestamp(f.id),
			u.name,
			u.is_admin,
			f.notes,
			f.metadata,
			(SELECT COUNT(*) FROM activity.file_views fv WHERE fv.file_id=$2 AND fv.user_id=$1)
		FROM data.files f
		JOIN system.mime m ON m.id=f.mime_id
		JOIN system.users u ON u.id=f.creator_id
		WHERE NOT f.is_deleted AND f.id=$2 AND (f.creator_id=$1 OR (SELECT view FROM acl.files WHERE file_id=$2 AND user_id=$1) OR (SELECT is_admin FROM system.users WHERE id=$1))
		`, user_id, file_id)
	err = row.Scan(&file.ID, &file.Name, &file.MIME.Name, &file.MIME.Extension, &file.CreatedAt, &file.Creator.Name, &file.Creator.IsAdmin, &file.Notes, &file.Metadata, &file.Viewed)
	if err != nil {
		statusCode, err = handleDBError(err)
		return
	}
	rows, err := connPool.Query(ctx, `
		SELECT
			t.id,
			t.name,
			COALESCE(t.color, c.color)
		FROM data.tags t
		LEFT JOIN data.categories c ON c.id=t.category_id
		JOIN data.file_tag ft ON ft.tag_id=t.id
		WHERE ft.file_id=$1
		`, file_id)
	if err != nil {
		statusCode, err = handleDBError(err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var tag domain.TagCore
		err = rows.Scan(&tag.ID, &tag.Name, &tag.Color)
		if err != nil {
			statusCode = http.StatusInternalServerError
			return
		}
		file.Tags = append(file.Tags, tag)
	}
	err = rows.Err()
	if err != nil {
		statusCode = http.StatusInternalServerError
		return
	}
	statusCode = http.StatusOK
	return
}

// Add file
func (s *FileStore) Add(user_id int, name, mime string, datetime time.Time, notes string, metadata json.RawMessage) (file domain.FileCore, statusCode int, err error) {
	ctx := context.Background()
	var mime_id int
	var extension string
	row := connPool.QueryRow(ctx, "SELECT id, extension FROM system.mime WHERE name=$1", mime)
	err = row.Scan(&mime_id, &extension)
	if err != nil {
		if err == pgx.ErrNoRows {
			err = fmt.Errorf("unsupported file type: %q", mime)
			statusCode = http.StatusBadRequest
		} else {
			statusCode, err = handleDBError(err)
		}
		return
	}
	row = connPool.QueryRow(ctx, `
	INSERT INTO data.files (name, mime_id, datetime, creator_id, notes, metadata)
	VALUES (NULLIF($1, ''), $2, $3, $4, NULLIF($5 ,''), $6)
	RETURNING id
	`, name, mime_id, datetime, user_id, notes, metadata)
	err = row.Scan(&file.ID)
	if err != nil {
		statusCode, err = handleDBError(err)
		return
	}
	file.Name.String = name
	file.Name.Valid = (name != "")
	file.MIME.Name = mime
	file.MIME.Extension = extension
	statusCode = http.StatusOK
	return
}

// Update file
func (s *FileStore) Update(user_id int, file_id string, updates map[string]interface{}) (statusCode int, err error) {
	if len(updates) == 0 {
		err = fmt.Errorf("no fields provided for update")
		statusCode = http.StatusBadRequest
		return
	}
	writableFields := map[string]bool{
		"name":     true,
		"datetime": true,
		"notes":    true,
		"metadata": true,
	}
	query := "UPDATE data.files SET"
	newValues := []interface{}{user_id}
	count := 2
	for field, value := range updates {
		if !writableFields[field] {
			err = fmt.Errorf("invalid field: %q", field)
			statusCode = http.StatusBadRequest
			return
		}
		query += fmt.Sprintf(" %s=NULLIF($%d, '')", field, count)
		newValues = append(newValues, value)
		count++
	}
	query += fmt.Sprintf(
		" WHERE id=$%d AND (creator_id=$1 OR (SELECT edit FROM acl.files WHERE file_id=$%d AND user_id=$1) OR (SELECT is_admin FROM system.users WHERE id=$1))",
		count, count)
	newValues = append(newValues, file_id)
	ctx := context.Background()
	commandTag, err := connPool.Exec(ctx, query, newValues...)
	if err != nil {
		statusCode, err = handleDBError(err)
		return
	}
	if commandTag.RowsAffected() == 0 {
		err = fmt.Errorf("not found")
		statusCode = http.StatusNotFound
		return
	}
	statusCode = http.StatusNoContent
	return
}

// Delete file
func (s *FileStore) Delete(user_id int, file_id string) (statusCode int, err error) {
	ctx := context.Background()
	commandTag, err := connPool.Exec(ctx,
		"DELETE FROM data.files WHERE id=$2 AND (creator_id=$1 OR (SELECT edit FROM acl.files WHERE file_id=$2 AND user_id=$1) OR (SELECT is_admin FROM system.users WHERE id=$1))",
		user_id, file_id)
	if err != nil {
		statusCode, err = handleDBError(err)
		return
	}
	if commandTag.RowsAffected() == 0 {
		err = fmt.Errorf("not found")
		statusCode = http.StatusNotFound
		return
	}
	statusCode = http.StatusNoContent
	return
}
