package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
)

// Transactor executes fn inside a single database transaction.
// All repository calls made within fn receive the transaction via context.
type Transactor interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// OffsetParams holds common offset-pagination and sort parameters.
type OffsetParams struct {
	Sort   string
	Order  string // "asc" | "desc"
	Search string
	Offset int
	Limit  int
}

// PoolFileListParams holds parameters for listing files inside a pool.
type PoolFileListParams struct {
	Cursor string
	Limit  int
	Filter string // filter DSL expression
}

// FileRepo is the persistence interface for file records.
type FileRepo interface {
	// List returns a cursor-based page of files.
	List(ctx context.Context, params domain.FileListParams) (*domain.FilePage, error)
	// GetByID returns the file with its tags loaded.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.File, error)
	// Create inserts a new file record and returns it.
	Create(ctx context.Context, f *domain.File) (*domain.File, error)
	// Update applies partial metadata changes and returns the updated record.
	Update(ctx context.Context, id uuid.UUID, f *domain.File) (*domain.File, error)
	// SoftDelete moves a file to trash (sets is_deleted = true).
	SoftDelete(ctx context.Context, id uuid.UUID) error
	// Restore moves a file out of trash (sets is_deleted = false).
	Restore(ctx context.Context, id uuid.UUID) (*domain.File, error)
	// DeletePermanent removes a file record. Only allowed when is_deleted = true.
	DeletePermanent(ctx context.Context, id uuid.UUID) error

	// ListTags returns all tags assigned to a file.
	ListTags(ctx context.Context, fileID uuid.UUID) ([]domain.Tag, error)
	// SetTags replaces all tags on a file (full replace semantics).
	SetTags(ctx context.Context, fileID uuid.UUID, tagIDs []uuid.UUID) error
}

// TagRepo is the persistence interface for tags.
type TagRepo interface {
	List(ctx context.Context, params OffsetParams) (*domain.TagOffsetPage, error)
	// ListByCategory returns tags belonging to a specific category.
	ListByCategory(ctx context.Context, categoryID uuid.UUID, params OffsetParams) (*domain.TagOffsetPage, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tag, error)
	Create(ctx context.Context, t *domain.Tag) (*domain.Tag, error)
	Update(ctx context.Context, id uuid.UUID, t *domain.Tag) (*domain.Tag, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// ListByFile returns all tags assigned to a specific file, ordered by name.
	ListByFile(ctx context.Context, fileID uuid.UUID) ([]domain.Tag, error)
	// AddFileTag inserts a single file→tag relation. No-op if already present.
	AddFileTag(ctx context.Context, fileID, tagID uuid.UUID) error
	// RemoveFileTag deletes a single file→tag relation.
	RemoveFileTag(ctx context.Context, fileID, tagID uuid.UUID) error
	// SetFileTags replaces all tags on a file (full replace semantics).
	SetFileTags(ctx context.Context, fileID uuid.UUID, tagIDs []uuid.UUID) error
	// CommonTagsForFiles returns tags present on every one of the given files.
	CommonTagsForFiles(ctx context.Context, fileIDs []uuid.UUID) ([]domain.Tag, error)
	// PartialTagsForFiles returns tags present on some but not all of the given files.
	PartialTagsForFiles(ctx context.Context, fileIDs []uuid.UUID) ([]domain.Tag, error)
}

// TagRuleRepo is the persistence interface for auto-tag rules.
type TagRuleRepo interface {
	// ListByTag returns all rules where WhenTagID == tagID.
	ListByTag(ctx context.Context, tagID uuid.UUID) ([]domain.TagRule, error)
	Create(ctx context.Context, r domain.TagRule) (*domain.TagRule, error)
	// SetActive toggles a rule's is_active flag. When active and applyToExisting
	// are both true, the full transitive expansion of thenTagID is retroactively
	// applied to all files that already carry whenTagID.
	SetActive(ctx context.Context, whenTagID, thenTagID uuid.UUID, active, applyToExisting bool) error
	Delete(ctx context.Context, whenTagID, thenTagID uuid.UUID) error
}

// CategoryRepo is the persistence interface for categories.
type CategoryRepo interface {
	List(ctx context.Context, params OffsetParams) (*domain.CategoryOffsetPage, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Category, error)
	Create(ctx context.Context, c *domain.Category) (*domain.Category, error)
	Update(ctx context.Context, id uuid.UUID, c *domain.Category) (*domain.Category, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// PoolRepo is the persistence interface for pools and pool–file membership.
type PoolRepo interface {
	List(ctx context.Context, params OffsetParams) (*domain.PoolOffsetPage, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Pool, error)
	Create(ctx context.Context, p *domain.Pool) (*domain.Pool, error)
	Update(ctx context.Context, id uuid.UUID, p *domain.Pool) (*domain.Pool, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// ListFiles returns pool files ordered by position (cursor-based).
	ListFiles(ctx context.Context, poolID uuid.UUID, params PoolFileListParams) (*domain.PoolFilePage, error)
	// AddFiles appends files starting at position; nil position means append at end.
	AddFiles(ctx context.Context, poolID uuid.UUID, fileIDs []uuid.UUID, position *int) error
	// RemoveFiles removes files from the pool.
	RemoveFiles(ctx context.Context, poolID uuid.UUID, fileIDs []uuid.UUID) error
	// Reorder sets the full ordered sequence of file IDs in the pool.
	Reorder(ctx context.Context, poolID uuid.UUID, fileIDs []uuid.UUID) error
}

// UserRepo is the persistence interface for users.
type UserRepo interface {
	List(ctx context.Context, params OffsetParams) (*domain.UserPage, error)
	GetByID(ctx context.Context, id int16) (*domain.User, error)
	// GetByName is used during login to look up credentials.
	GetByName(ctx context.Context, name string) (*domain.User, error)
	Create(ctx context.Context, u *domain.User) (*domain.User, error)
	Update(ctx context.Context, id int16, u *domain.User) (*domain.User, error)
	Delete(ctx context.Context, id int16) error
}

// SessionRepo is the persistence interface for auth sessions.
type SessionRepo interface {
	// ListByUser returns all active sessions for a user.
	ListByUser(ctx context.Context, userID int16) (*domain.SessionList, error)
	// GetByID returns an active session by its ID, or ErrNotFound if it does not
	// exist or has been deactivated.
	GetByID(ctx context.Context, id int) (*domain.Session, error)
	// GetByTokenHash looks up a session by the hashed refresh token.
	GetByTokenHash(ctx context.Context, hash string) (*domain.Session, error)
	Create(ctx context.Context, s *domain.Session) (*domain.Session, error)
	// UpdateLastActivity refreshes the last_activity timestamp.
	UpdateLastActivity(ctx context.Context, id int, t time.Time) error
	// Delete terminates a single session.
	Delete(ctx context.Context, id int) error
	// DeleteByUserID terminates all sessions for a user (logout everywhere).
	DeleteByUserID(ctx context.Context, userID int16) error
}

// ACLRepo is the persistence interface for per-object permissions.
type ACLRepo interface {
	// List returns all permission entries for a given object.
	List(ctx context.Context, objectTypeID int16, objectID uuid.UUID) ([]domain.Permission, error)
	// Get returns the permission entry for a specific user and object; returns
	// ErrNotFound if no entry exists.
	Get(ctx context.Context, userID int16, objectTypeID int16, objectID uuid.UUID) (*domain.Permission, error)
	// Set replaces all permissions for an object (full replace semantics).
	Set(ctx context.Context, objectTypeID int16, objectID uuid.UUID, perms []domain.Permission) error
}

// AuditRepo is the persistence interface for the audit log.
type AuditRepo interface {
	Log(ctx context.Context, entry domain.AuditEntry) error
	List(ctx context.Context, filter domain.AuditFilter) (*domain.AuditPage, error)
}

// MimeRepo is the persistence interface for the MIME type whitelist.
type MimeRepo interface {
	// List returns all supported MIME types.
	List(ctx context.Context) ([]domain.MIMEType, error)
	// GetByName returns the MIME type record for a given MIME name (e.g. "image/jpeg").
	// Returns ErrUnsupportedMIME if not in the whitelist.
	GetByName(ctx context.Context, name string) (*domain.MIMEType, error)
}
