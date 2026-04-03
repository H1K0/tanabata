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
	CreatedAt       time.Time // extracted from UUID v7
	Tags            []Tag     // loaded with the file
}

// FileListParams holds all parameters for listing/filtering files.
type FileListParams struct {
	Filter    string
	Sort      string
	Order     string
	Cursor    string
	Anchor    *uuid.UUID
	Direction string // "forward" or "backward"
	Limit     int
	Trash     bool
	Search    string
}

// FilePage is the result of a cursor-based file listing.
type FilePage struct {
	Items      []File
	NextCursor *string
	PrevCursor *string
}
