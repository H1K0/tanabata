package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MIMEType holds MIME whitelist data.
type MIMEType struct {
	ID        int16
	Name      string
	Extension string
}

// File represents a managed file record.
type File struct {
	ID              uuid.UUID
	OriginalName    *string
	MIMEType        string // denormalized from core.mime_types
	MIMEExtension   string // denormalized from core.mime_types
	ContentDatetime time.Time
	Notes           *string
	Metadata        json.RawMessage
	EXIF            json.RawMessage
	PHash           *int64
	CreatorID       int16
	CreatorName     string // denormalized from core.users
	IsPublic        bool
	IsDeleted       bool
	CreatedAt       time.Time // extracted from UUID v7 via UUIDCreatedAt
	Tags            []Tag     // loaded with the file
}

// FileListParams holds all parameters for listing/filtering files.
type FileListParams struct {
	// Pagination
	Cursor    string
	Direction string // "forward" or "backward"
	Anchor    *uuid.UUID
	Limit     int

	// Sorting
	Sort  string // "content_datetime" | "created" | "original_name" | "mime"
	Order string // "asc" | "desc"

	// Filtering
	Filter string // filter DSL expression
	Search string // substring match on original_name
	Trash  bool   // if true, return only soft-deleted files

	// Visibility — populated by the service from the request context. When
	// ViewerIsAdmin is false the repository restricts results to files the
	// viewer may see (public, owned, or explicitly granted).
	ViewerID      int16
	ViewerIsAdmin bool
}

// FilePage is the result of a cursor-based file listing.
type FilePage struct {
	Items      []File
	NextCursor *string
	PrevCursor *string
}

// UUIDCreatedAt extracts the creation timestamp embedded in a UUID v7.
// UUID v7 stores Unix milliseconds in the most-significant 48 bits.
func UUIDCreatedAt(id uuid.UUID) time.Time {
	ms := int64(id[0])<<40 | int64(id[1])<<32 | int64(id[2])<<24 |
		int64(id[3])<<16 | int64(id[4])<<8 | int64(id[5])
	return time.Unix(ms/1000, (ms%1000)*int64(time.Millisecond)).UTC()
}
