package port

import (
	"context"
	"io"

	"github.com/google/uuid"
)

// FileStorage abstracts disk (or object-store) operations for file content,
// thumbnails, and previews.
type FileStorage interface {
	// Save writes the reader's content to storage and returns the number of
	// bytes written.
	Save(ctx context.Context, id uuid.UUID, r io.Reader) (int64, error)

	// Read opens the file content for reading. The caller must close the returned
	// ReadCloser.
	Read(ctx context.Context, id uuid.UUID) (io.ReadCloser, error)

	// Delete removes the file content from storage.
	Delete(ctx context.Context, id uuid.UUID) error

	// InvalidateCache removes any cached thumbnail/preview for the file so they
	// are regenerated from the current content on next request.
	InvalidateCache(ctx context.Context, id uuid.UUID) error

	// Thumbnail opens the pre-generated thumbnail (JPEG). Returns ErrNotFound
	// if the thumbnail has not been generated yet.
	Thumbnail(ctx context.Context, id uuid.UUID) (io.ReadCloser, error)

	// Preview opens the pre-generated preview image (JPEG). Returns ErrNotFound
	// if the preview has not been generated yet.
	Preview(ctx context.Context, id uuid.UUID) (io.ReadCloser, error)
}
