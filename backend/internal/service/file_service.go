package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

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
	Reader       io.Reader
	MIMEType     string
	OriginalName *string
	Notes        *string
	Metadata     json.RawMessage
	IsPublic     bool
	TagIDs       []uuid.UUID
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

// FileService handles business logic for file records.
type FileService struct {
	files   port.FileRepo
	mimes   port.MimeRepo
	storage port.FileStorage
	acl     *ACLService
	audit   *AuditService
	tx      port.Transactor
}

// NewFileService creates a FileService.
func NewFileService(
	files port.FileRepo,
	mimes port.MimeRepo,
	storage port.FileStorage,
	acl *ACLService,
	audit *AuditService,
	tx port.Transactor,
) *FileService {
	return &FileService{
		files:   files,
		mimes:   mimes,
		storage: storage,
		acl:     acl,
		audit:   audit,
		tx:      tx,
	}
}

// Upload validates the MIME type, saves the file to storage, creates the DB
// record, and applies any initial tags — all within a single transaction.
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
	exifData := extractEXIF(data)

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
			ID:           fileID,
			OriginalName: p.OriginalName,
			MIMEType:     mime.Name,
			MIMEExtension: mime.Extension,
			Notes:        p.Notes,
			Metadata:     p.Metadata,
			EXIF:         exifData,
			CreatorID:    userID,
			IsPublic:     p.IsPublic,
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
			// Re-fetch to populate Tags on the returned value.
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
// when the file is already in trash. Restricted to admins and the creator.
func (s *FileService) PermanentDelete(ctx context.Context, id uuid.UUID) error {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	f, err := s.files.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !f.IsDeleted {
		return domain.ErrValidation
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

// Replace swaps the stored bytes for a file with new content. The MIME type
// may change. Thumbnail/preview caches are not invalidated here — callers
// should handle that if needed.
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
	exifData := extractEXIF(data)

	// Save new bytes, overwriting the existing stored file.
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
// Internal helpers
// ---------------------------------------------------------------------------

// extractEXIF attempts to parse EXIF data from raw bytes and marshal it to
// JSON. Returns nil on any error (non-image files, no EXIF header, etc.).
func extractEXIF(data []byte) json.RawMessage {
	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	b, err := x.MarshalJSON()
	if err != nil {
		return nil
	}
	return json.RawMessage(b)
}