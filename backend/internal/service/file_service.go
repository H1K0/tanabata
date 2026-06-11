package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"

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
	// ContentDatetimeFallback is used for content_datetime only when neither an
	// explicit ContentDatetime nor an EXIF date is available (e.g. the source
	// file's mtime on a server-side import).
	ContentDatetimeFallback *time.Time
	IsPublic                bool
	TagIDs                  []uuid.UUID
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

// ImportEvent is one progress message streamed during an import, letting the UI
// show a live progress bar and a per-file status list. Type is the discriminator:
//
//	"start" — total is the number of entries about to be processed.
//	"file"  — one entry finished: index (1-based), filename, status, optional reason.
//	"done"  — final tallies (imported/skipped/errors).
type ImportEvent struct {
	Type     string `json:"type"`
	Total    int    `json:"total,omitempty"`
	Index    int    `json:"index,omitempty"`
	Filename string `json:"filename,omitempty"`
	Status   string `json:"status,omitempty"` // "imported" | "skipped" | "error"
	Reason   string `json:"reason,omitempty"`
	Imported int    `json:"imported,omitempty"`
	Skipped  int    `json:"skipped,omitempty"`
	Errors   int    `json:"errors,omitempty"`
}

// FileService handles business logic for file records.
type FileService struct {
	files      port.FileRepo
	mimes      port.MimeRepo
	storage    port.FileStorage
	acl        *ACLService
	audit      *AuditService
	tags       *TagService
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
	tags *TagService,
	tx port.Transactor,
	importPath string,
) *FileService {
	return &FileService{
		files:      files,
		mimes:      mimes,
		storage:    storage,
		acl:        acl,
		audit:      audit,
		tags:       tags,
		tx:         tx,
		importPath: importPath,
	}
}

// ---------------------------------------------------------------------------
// Core CRUD
// ---------------------------------------------------------------------------

// Upload validates the MIME type, saves the file to storage, creates the DB
// record, and applies any initial tags — all within a single transaction.
// If ContentDatetime is nil and the metadata carries a capture date, it is used.
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

	// Extract rich metadata (best-effort; covers images, video and audio).
	var origName string
	if p.OriginalName != nil {
		origName = *p.OriginalName
	}
	exifData, exifDatetime := extractMetadata(data, origName, p.ContentDatetimeFallback)

	// Resolve content datetime: explicit > metadata date > fallback (e.g. import mtime) > zero.
	var contentDatetime time.Time
	if p.ContentDatetime != nil {
		contentDatetime = *p.ContentDatetime
	} else if exifDatetime != nil {
		contentDatetime = *exifDatetime
	} else if p.ContentDatetimeFallback != nil {
		contentDatetime = *p.ContentDatetimeFallback
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
			tags, err := s.tags.SetFileTags(ctx, created.ID, p.TagIDs)
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

// RecordView appends a view-history entry for the current user, enforcing view
// ACL (you can only record a view of a file you may see).
func (s *FileService) RecordView(ctx context.Context, id uuid.UUID) error {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	f, err := s.files.GetByID(ctx, id)
	if err != nil {
		return err
	}

	ok, err := s.acl.CanView(ctx, userID, isAdmin, f.CreatorID, f.IsPublic, fileObjectTypeID, id)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return s.files.RecordView(ctx, id, userID)
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
			tags, err := s.tags.SetFileTags(ctx, id, *p.TagIDs)
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
	_ = s.storage.InvalidateCache(ctx, id)

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
	var origName string
	if p.OriginalName != nil {
		origName = *p.OriginalName
	}
	exifData, _ := extractMetadata(data, origName, nil)

	if _, err := s.storage.Save(ctx, id, bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("FileService.Replace: save to storage: %w", err)
	}
	// Drop stale thumbnail/preview so they regenerate from the new content.
	_ = s.storage.InvalidateCache(ctx, id)

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

// List delegates to FileRepo with the given params, restricting results to
// files the caller may see (unless they are an admin).
func (s *FileService) List(ctx context.Context, params domain.FileListParams) (*domain.FilePage, error) {
	params.ViewerID, params.ViewerIsAdmin, _ = domain.UserFromContext(ctx)

	page, err := s.files.List(ctx, params)
	if err != nil {
		return nil, err
	}

	// Log tag usage when a filter is first applied — not on pagination (cursor)
	// or an anchored return, so a single browse counts once. Best-effort
	// analytics; a failed write never breaks the listing.
	if params.Filter != "" && params.Cursor == "" && params.Anchor == nil && params.ViewerID != 0 {
		_ = s.files.RecordTagUses(ctx, params.ViewerID, params.Filter)
	}

	return page, nil
}

// AuthorizeView ensures the caller may view the file. Returns ErrNotFound if the
// file does not exist or ErrForbidden if the caller lacks view access.
func (s *FileService) AuthorizeView(ctx context.Context, id uuid.UUID) error {
	_, err := s.Get(ctx, id)
	return err
}

// AuthorizeEdit ensures the caller may edit the file. Returns ErrNotFound if the
// file does not exist or ErrForbidden if the caller lacks edit access.
func (s *FileService) AuthorizeEdit(ctx context.Context, id uuid.UUID) error {
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
	return nil
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
// Bulk operations
// ---------------------------------------------------------------------------

// BulkDelete soft-deletes multiple files. Files the caller cannot edit are silently skipped.
func (s *FileService) BulkDelete(ctx context.Context, fileIDs []uuid.UUID) error {
	for _, id := range fileIDs {
		if err := s.Delete(ctx, id); err != nil {
			if err == domain.ErrNotFound || err == domain.ErrForbidden {
				continue
			}
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Import
// ---------------------------------------------------------------------------

// Import scans a server-side directory and uploads all supported files.
// If path is empty, the configured default import path is used.
//
// onProgress, when non-nil, receives a "start" event, one "file" event per
// directory entry as it is processed, and a final "done" event — letting a
// caller stream live progress. It is always called from this goroutine (never
// concurrently). The aggregate result is also returned for non-streaming callers.
func (s *FileService) Import(ctx context.Context, path string, onProgress func(ImportEvent)) (*ImportResult, error) {
	if s.importPath == "" {
		return nil, domain.ErrValidation
	}

	dir := s.importPath
	if path != "" {
		// Confine caller-supplied paths to the configured import directory so a
		// directory-traversal value cannot read arbitrary host files.
		confined, err := confineToBase(s.importPath, path)
		if err != nil {
			return nil, err
		}
		dir = confined
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("FileService.Import: read dir %q: %w", dir, err)
	}

	emit := func(ev ImportEvent) {
		if onProgress != nil {
			onProgress(ev)
		}
	}

	result := &ImportResult{Errors: []ImportFileError{}}
	total := len(entries)
	emit(ImportEvent{Type: "start", Total: total})

	for i, entry := range entries {
		name := entry.Name()
		file := func(status, reason string) {
			emit(ImportEvent{
				Type: "file", Index: i + 1, Total: total,
				Filename: name, Status: status, Reason: reason,
			})
		}
		fail := func(reason string) {
			result.Errors = append(result.Errors, ImportFileError{Filename: name, Reason: reason})
			file("error", reason)
		}

		if entry.IsDir() {
			result.Skipped++
			file("skipped", "directory")
			continue
		}

		fullPath := filepath.Join(dir, name)

		mt, err := mimetype.DetectFile(fullPath)
		if err != nil {
			fail(fmt.Sprintf("MIME detection failed: %s", err))
			continue
		}

		mimeStr := mt.String()
		// Strip parameters (e.g. "text/plain; charset=utf-8" → "text/plain").
		if j := strings.IndexByte(mimeStr, ';'); j >= 0 {
			mimeStr = mimeStr[:j]
		}

		if _, err := s.mimes.GetByName(ctx, mimeStr); err != nil {
			result.Skipped++
			file("skipped", "unsupported type: "+mimeStr)
			continue
		}

		f, err := os.Open(fullPath)
		if err != nil {
			fail(fmt.Sprintf("open failed: %s", err))
			continue
		}

		// Preserve the file's mtime as a content_datetime fallback (used only when
		// the file has no EXIF date) — once the source is removed below it's the
		// only date left for non-photo files.
		var mtime *time.Time
		if info, statErr := entry.Info(); statErr == nil {
			t := info.ModTime()
			mtime = &t
		}

		_, uploadErr := s.Upload(ctx, UploadParams{
			Reader:                  f,
			MIMEType:                mimeStr,
			OriginalName:            &name,
			ContentDatetimeFallback: mtime,
		})
		f.Close()

		if uploadErr != nil {
			fail(uploadErr.Error())
			continue
		}
		result.Imported++

		// Remove the source on success so the import folder drains and re-running
		// doesn't duplicate. The file is already safely copied into storage; a
		// removal failure is reported but doesn't undo the import.
		if rmErr := os.Remove(fullPath); rmErr != nil {
			reason := fmt.Sprintf("imported, but failed to remove source: %s", rmErr)
			result.Errors = append(result.Errors, ImportFileError{Filename: name, Reason: reason})
			file("imported", reason) // imported, with a warning
			continue
		}
		file("imported", "")
	}

	emit(ImportEvent{
		Type: "done", Total: total,
		Imported: result.Imported, Skipped: result.Skipped, Errors: len(result.Errors),
	})

	return result, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// confineToBase resolves target and verifies it does not escape base (after
// cleaning and resolving "..") so a caller cannot read files outside the
// configured import directory. Returns the cleaned absolute path on success.
func confineToBase(base, target string) (string, error) {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", domain.ErrValidation
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", domain.ErrValidation
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return "", domain.ErrValidation
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", domain.ErrForbidden
	}
	return absTarget, nil
}
