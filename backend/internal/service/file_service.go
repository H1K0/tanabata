package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
	"github.com/rwcarlsen/goexif/exif"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

const fileObjectType = "file"

// fileObjectTypeID is the primary key of the "file" row in core.object_types.
// It matches the first value inserted in 007_seed_data.sql.
const fileObjectTypeID int16 = 1

// UploadParams holds the parameters for uploading a new file.
type UploadParams struct {
	Reader          io.Reader
	MIMEType        string
	OriginalName    *string
	Notes           *string
	Metadata        json.RawMessage
	ContentDatetime *time.Time
	IsPublic        bool
	TagIDs          []uuid.UUID
}

// UpdateParams holds the parameters for updating file metadata.
type UpdateParams struct {
	OriginalName    *string
	Notes           *string
	Metadata        json.RawMessage
	ContentDatetime *time.Time
	IsPublic        *bool
	TagIDs          *[]uuid.UUID // nil means don't change tags
}

// ContentResult holds the open reader and metadata for a file download.
type ContentResult struct {
	Body         io.ReadCloser
	MIMEType     string
	OriginalName *string
}

// ImportFileError records a failed file during an import operation.
type ImportFileError struct {
	Filename string `json:"filename"`
	Reason   string `json:"reason"`
}

// ImportResult summarises a directory import.
type ImportResult struct {
	Imported int               `json:"imported"`
	Skipped  int               `json:"skipped"`
	Errors   []ImportFileError `json:"errors"`
}

// FileService handles business logic for file records.
type FileService struct {
	files      port.FileRepo
	mimes      port.MimeRepo
	storage    port.FileStorage
	acl        *ACLService
	audit      *AuditService
	tx         port.Transactor
	importPath string // default server-side import directory
}

// NewFileService creates a FileService.
func NewFileService(
	files port.FileRepo,
	mimes port.MimeRepo,
	storage port.FileStorage,
	acl *ACLService,
	audit *AuditService,
	tx port.Transactor,
	importPath string,
) *FileService {
	return &FileService{
		files:      files,
		mimes:      mimes,
		storage:    storage,
		acl:        acl,
		audit:      audit,
		tx:         tx,
		importPath: importPath,
	}
}

// ---------------------------------------------------------------------------
// Core CRUD
// ---------------------------------------------------------------------------

// Upload validates the MIME type, saves the file to storage, creates the DB
// record, and applies any initial tags — all within a single transaction.
// If ContentDatetime is nil and EXIF DateTimeOriginal is present, it is used.
func (s *FileService) Upload(ctx context.Context, p UploadParams) (*domain.File, error) {
	userID, _, _ := domain.UserFromContext(ctx)

	// Validate MIME type against the whitelist.
	mime, err := s.mimes.GetByName(ctx, p.MIMEType)
	if err != nil {
		return nil, err // ErrUnsupportedMIME or DB error
	}

	// Buffer the upload so we can extract EXIF without re-reading storage.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, p.Reader); err != nil {
		return nil, fmt.Errorf("FileService.Upload: read body: %w", err)
	}
	data := buf.Bytes()

	// Extract EXIF metadata (best-effort; non-image files will error silently).
	exifData, exifDatetime := extractEXIFWithDatetime(data)

	// Resolve content datetime: explicit > EXIF > zero value.
	var contentDatetime time.Time
	if p.ContentDatetime != nil {
		contentDatetime = *p.ContentDatetime
	} else if exifDatetime != nil {
		contentDatetime = *exifDatetime
	}

	// Assign UUID v7 so CreatedAt can be derived from it later.
	fileID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("FileService.Upload: generate UUID: %w", err)
	}

	// Save file bytes to disk before opening the transaction so that a disk
	// failure does not abort an otherwise healthy DB transaction.
	if _, err := s.storage.Save(ctx, fileID, bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("FileService.Upload: save to storage: %w", err)
	}

	var created *domain.File
	txErr := s.tx.WithTx(ctx, func(ctx context.Context) error {
		f := &domain.File{
			ID:              fileID,
			OriginalName:    p.OriginalName,
			MIMEType:        mime.Name,
			MIMEExtension:   mime.Extension,
			ContentDatetime: contentDatetime,
			Notes:           p.Notes,
			Metadata:        p.Metadata,
			EXIF:            exifData,
			CreatorID:       userID,
			IsPublic:        p.IsPublic,
		}

		var createErr error
		created, createErr = s.files.Create(ctx, f)
		if createErr != nil {
			return createErr
		}

		if len(p.TagIDs) > 0 {
			if err := s.files.SetTags(ctx, created.ID, p.TagIDs); err != nil {
				return err
			}
			tags, err := s.files.ListTags(ctx, created.ID)
			if err != nil {
				return err
			}
			created.Tags = tags
		}
		return nil
	})
	if txErr != nil {
		// Attempt to clean up the orphaned file; ignore cleanup errors.
		_ = s.storage.Delete(ctx, fileID)
		return nil, txErr
	}

	objType := fileObjectType
	_ = s.audit.Log(ctx, "file_create", &objType, &created.ID, nil)
	return created, nil
}

// Get returns a file by ID, enforcing view ACL.
func (s *FileService) Get(ctx context.Context, id uuid.UUID) (*domain.File, error) {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	f, err := s.files.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	ok, err := s.acl.CanView(ctx, userID, isAdmin, f.CreatorID, f.IsPublic, fileObjectTypeID, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}
	return f, nil
}

// Update applies metadata changes to a file, enforcing edit ACL.
func (s *FileService) Update(ctx context.Context, id uuid.UUID, p UpdateParams) (*domain.File, error) {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	f, err := s.files.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	ok, err := s.acl.CanEdit(ctx, userID, isAdmin, f.CreatorID, fileObjectTypeID, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}

	patch := &domain.File{}
	if p.OriginalName != nil {
		patch.OriginalName = p.OriginalName
	}
	if p.Notes != nil {
		patch.Notes = p.Notes
	}
	if p.Metadata != nil {
		patch.Metadata = p.Metadata
	}
	if p.ContentDatetime != nil {
		patch.ContentDatetime = *p.ContentDatetime
	}
	if p.IsPublic != nil {
		patch.IsPublic = *p.IsPublic
	}

	var updated *domain.File
	txErr := s.tx.WithTx(ctx, func(ctx context.Context) error {
		var updateErr error
		updated, updateErr = s.files.Update(ctx, id, patch)
		if updateErr != nil {
			return updateErr
		}
		if p.TagIDs != nil {
			if err := s.files.SetTags(ctx, id, *p.TagIDs); err != nil {
				return err
			}
			tags, err := s.files.ListTags(ctx, id)
			if err != nil {
				return err
			}
			updated.Tags = tags
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	objType := fileObjectType
	_ = s.audit.Log(ctx, "file_edit", &objType, &id, nil)
	return updated, nil
}

// Delete soft-deletes a file (moves to trash), enforcing edit ACL.
func (s *FileService) Delete(ctx context.Context, id uuid.UUID) error {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	f, err := s.files.GetByID(ctx, id)
	if err != nil {
		return err
	}

	ok, err := s.acl.CanEdit(ctx, userID, isAdmin, f.CreatorID, fileObjectTypeID, id)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}

	if err := s.files.SoftDelete(ctx, id); err != nil {
		return err
	}

	objType := fileObjectType
	_ = s.audit.Log(ctx, "file_delete", &objType, &id, nil)
	return nil
}

// Restore moves a soft-deleted file out of trash, enforcing edit ACL.
func (s *FileService) Restore(ctx context.Context, id uuid.UUID) (*domain.File, error) {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	f, err := s.files.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	ok, err := s.acl.CanEdit(ctx, userID, isAdmin, f.CreatorID, fileObjectTypeID, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}

	restored, err := s.files.Restore(ctx, id)
	if err != nil {
		return nil, err
	}

	objType := fileObjectType
	_ = s.audit.Log(ctx, "file_restore", &objType, &id, nil)
	return restored, nil
}

// PermanentDelete removes the file record and its stored bytes. Only allowed
// when the file is already in trash.
func (s *FileService) PermanentDelete(ctx context.Context, id uuid.UUID) error {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	f, err := s.files.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !f.IsDeleted {
		return domain.ErrConflict
	}

	ok, err := s.acl.CanEdit(ctx, userID, isAdmin, f.CreatorID, fileObjectTypeID, id)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}

	if err := s.files.DeletePermanent(ctx, id); err != nil {
		return err
	}
	_ = s.storage.Delete(ctx, id)

	objType := fileObjectType
	_ = s.audit.Log(ctx, "file_permanent_delete", &objType, &id, nil)
	return nil
}

// Replace swaps the stored bytes for a file with new content.
func (s *FileService) Replace(ctx context.Context, id uuid.UUID, p UploadParams) (*domain.File, error) {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	f, err := s.files.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	ok, err := s.acl.CanEdit(ctx, userID, isAdmin, f.CreatorID, fileObjectTypeID, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}

	mime, err := s.mimes.GetByName(ctx, p.MIMEType)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, p.Reader); err != nil {
		return nil, fmt.Errorf("FileService.Replace: read body: %w", err)
	}
	data := buf.Bytes()
	exifData, _ := extractEXIFWithDatetime(data)

	if _, err := s.storage.Save(ctx, id, bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("FileService.Replace: save to storage: %w", err)
	}

	patch := &domain.File{
		MIMEType:      mime.Name,
		MIMEExtension: mime.Extension,
		EXIF:          exifData,
	}
	if p.OriginalName != nil {
		patch.OriginalName = p.OriginalName
	}

	updated, err := s.files.Update(ctx, id, patch)
	if err != nil {
		return nil, err
	}

	objType := fileObjectType
	_ = s.audit.Log(ctx, "file_replace", &objType, &id, nil)
	return updated, nil
}

// List delegates to FileRepo with the given params.
func (s *FileService) List(ctx context.Context, params domain.FileListParams) (*domain.FilePage, error) {
	return s.files.List(ctx, params)
}

// ---------------------------------------------------------------------------
// Content / thumbnail / preview streaming
// ---------------------------------------------------------------------------

// GetContent opens the raw file for download, enforcing view ACL.
func (s *FileService) GetContent(ctx context.Context, id uuid.UUID) (*ContentResult, error) {
	f, err := s.Get(ctx, id) // ACL checked inside Get
	if err != nil {
		return nil, err
	}
	rc, err := s.storage.Read(ctx, id)
	if err != nil {
		return nil, err
	}
	return &ContentResult{
		Body:         rc,
		MIMEType:     f.MIMEType,
		OriginalName: f.OriginalName,
	}, nil
}

// GetThumbnail returns the thumbnail JPEG, enforcing view ACL.
func (s *FileService) GetThumbnail(ctx context.Context, id uuid.UUID) (io.ReadCloser, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return nil, err
	}
	return s.storage.Thumbnail(ctx, id)
}

// GetPreview returns the preview JPEG, enforcing view ACL.
func (s *FileService) GetPreview(ctx context.Context, id uuid.UUID) (io.ReadCloser, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return nil, err
	}
	return s.storage.Preview(ctx, id)
}

// ---------------------------------------------------------------------------
// Tag operations
// ---------------------------------------------------------------------------

// ListFileTags returns the tags on a file, enforcing view ACL.
func (s *FileService) ListFileTags(ctx context.Context, fileID uuid.UUID) ([]domain.Tag, error) {
	if _, err := s.Get(ctx, fileID); err != nil {
		return nil, err
	}
	return s.files.ListTags(ctx, fileID)
}

// SetFileTags replaces all tags on a file (full replace semantics), enforcing edit ACL.
func (s *FileService) SetFileTags(ctx context.Context, fileID uuid.UUID, tagIDs []uuid.UUID) ([]domain.Tag, error) {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	f, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	ok, err := s.acl.CanEdit(ctx, userID, isAdmin, f.CreatorID, fileObjectTypeID, fileID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}

	if err := s.files.SetTags(ctx, fileID, tagIDs); err != nil {
		return nil, err
	}

	objType := fileObjectType
	_ = s.audit.Log(ctx, "file_tag_add", &objType, &fileID, nil)
	return s.files.ListTags(ctx, fileID)
}

// AddTag adds a single tag to a file, enforcing edit ACL.
func (s *FileService) AddTag(ctx context.Context, fileID, tagID uuid.UUID) ([]domain.Tag, error) {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	f, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	ok, err := s.acl.CanEdit(ctx, userID, isAdmin, f.CreatorID, fileObjectTypeID, fileID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}

	current, err := s.files.ListTags(ctx, fileID)
	if err != nil {
		return nil, err
	}
	// Only add if not already present.
	for _, t := range current {
		if t.ID == tagID {
			return current, nil
		}
	}
	ids := make([]uuid.UUID, 0, len(current)+1)
	for _, t := range current {
		ids = append(ids, t.ID)
	}
	ids = append(ids, tagID)

	if err := s.files.SetTags(ctx, fileID, ids); err != nil {
		return nil, err
	}

	objType := fileObjectType
	_ = s.audit.Log(ctx, "file_tag_add", &objType, &fileID, map[string]any{"tag_id": tagID})
	return s.files.ListTags(ctx, fileID)
}

// RemoveTag removes a single tag from a file, enforcing edit ACL.
func (s *FileService) RemoveTag(ctx context.Context, fileID, tagID uuid.UUID) error {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	f, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return err
	}
	ok, err := s.acl.CanEdit(ctx, userID, isAdmin, f.CreatorID, fileObjectTypeID, fileID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}

	current, err := s.files.ListTags(ctx, fileID)
	if err != nil {
		return err
	}
	ids := make([]uuid.UUID, 0, len(current))
	for _, t := range current {
		if t.ID != tagID {
			ids = append(ids, t.ID)
		}
	}

	if err := s.files.SetTags(ctx, fileID, ids); err != nil {
		return err
	}

	objType := fileObjectType
	_ = s.audit.Log(ctx, "file_tag_remove", &objType, &fileID, map[string]any{"tag_id": tagID})
	return nil
}

// ---------------------------------------------------------------------------
// Bulk operations
// ---------------------------------------------------------------------------

// BulkDelete soft-deletes multiple files. Files the caller cannot edit are silently skipped.
func (s *FileService) BulkDelete(ctx context.Context, fileIDs []uuid.UUID) error {
	for _, id := range fileIDs {
		if err := s.Delete(ctx, id); err != nil {
			// Skip files not found or forbidden; surface real errors.
			if err == domain.ErrNotFound || err == domain.ErrForbidden {
				continue
			}
			return err
		}
	}
	return nil
}

// BulkSetTags adds or removes the given tags on multiple files.
// For "add": tags are appended to each file's existing set.
// For "remove": tags are removed from each file's existing set.
// Returns the tag IDs that were applied (the input tagIDs, for add).
func (s *FileService) BulkSetTags(ctx context.Context, fileIDs []uuid.UUID, action string, tagIDs []uuid.UUID) ([]uuid.UUID, error) {
	for _, fileID := range fileIDs {
		switch action {
		case "add":
			for _, tagID := range tagIDs {
				if _, err := s.AddTag(ctx, fileID, tagID); err != nil {
					if err == domain.ErrNotFound || err == domain.ErrForbidden {
						continue
					}
					return nil, err
				}
			}
		case "remove":
			for _, tagID := range tagIDs {
				if err := s.RemoveTag(ctx, fileID, tagID); err != nil {
					if err == domain.ErrNotFound || err == domain.ErrForbidden {
						continue
					}
					return nil, err
				}
			}
		default:
			return nil, domain.ErrValidation
		}
	}
	if action == "add" {
		return tagIDs, nil
	}
	return []uuid.UUID{}, nil
}

// CommonTags loads the tag sets for all given files and splits them into:
//   - common: tag IDs present on every file
//   - partial: tag IDs present on some but not all files
func (s *FileService) CommonTags(ctx context.Context, fileIDs []uuid.UUID) (common, partial []uuid.UUID, err error) {
	if len(fileIDs) == 0 {
		return nil, nil, nil
	}

	// Count how many files each tag appears on.
	counts := map[uuid.UUID]int{}
	for _, fid := range fileIDs {
		tags, err := s.files.ListTags(ctx, fid)
		if err != nil {
			return nil, nil, err
		}
		for _, t := range tags {
			counts[t.ID]++
		}
	}

	n := len(fileIDs)
	for id, cnt := range counts {
		if cnt == n {
			common = append(common, id)
		} else {
			partial = append(partial, id)
		}
	}
	if common == nil {
		common = []uuid.UUID{}
	}
	if partial == nil {
		partial = []uuid.UUID{}
	}
	return common, partial, nil
}

// ---------------------------------------------------------------------------
// Import
// ---------------------------------------------------------------------------

// Import scans a server-side directory and uploads all supported files.
// If path is empty, the configured default import path is used.
func (s *FileService) Import(ctx context.Context, path string) (*ImportResult, error) {
	dir := path
	if dir == "" {
		dir = s.importPath
	}
	if dir == "" {
		return nil, domain.ErrValidation
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("FileService.Import: read dir %q: %w", dir, err)
	}

	result := &ImportResult{Errors: []ImportFileError{}}

	for _, entry := range entries {
		if entry.IsDir() {
			result.Skipped++
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())

		mt, err := mimetype.DetectFile(fullPath)
		if err != nil {
			result.Errors = append(result.Errors, ImportFileError{
				Filename: entry.Name(),
				Reason:   fmt.Sprintf("MIME detection failed: %s", err),
			})
			continue
		}

		mimeStr := mt.String()
		// Strip parameters (e.g. "text/plain; charset=utf-8" → "text/plain").
		if idx := len(mimeStr); idx > 0 {
			for i, c := range mimeStr {
				if c == ';' {
					mimeStr = mimeStr[:i]
					break
				}
			}
		}

		if _, err := s.mimes.GetByName(ctx, mimeStr); err != nil {
			result.Skipped++
			continue
		}

		f, err := os.Open(fullPath)
		if err != nil {
			result.Errors = append(result.Errors, ImportFileError{
				Filename: entry.Name(),
				Reason:   fmt.Sprintf("open failed: %s", err),
			})
			continue
		}

		name := entry.Name()
		_, uploadErr := s.Upload(ctx, UploadParams{
			Reader:       f,
			MIMEType:     mimeStr,
			OriginalName: &name,
		})
		f.Close()

		if uploadErr != nil {
			result.Errors = append(result.Errors, ImportFileError{
				Filename: entry.Name(),
				Reason:   uploadErr.Error(),
			})
			continue
		}
		result.Imported++
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// extractEXIFWithDatetime parses EXIF from raw bytes, returning both the JSON
// representation and the DateTimeOriginal (if present). Both may be nil.
func extractEXIFWithDatetime(data []byte) (json.RawMessage, *time.Time) {
	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, nil
	}
	b, err := x.MarshalJSON()
	if err != nil {
		return nil, nil
	}
	var dt *time.Time
	if t, err := x.DateTime(); err == nil {
		dt = &t
	}
	return json.RawMessage(b), dt
}