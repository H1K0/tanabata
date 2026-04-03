package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Tag represents a file label.
type Tag struct {
	ID            uuid.UUID
	Name          string
	Notes         *string
	Color         *string // 6-char hex, e.g. "5DCAA5"
	CategoryID    *uuid.UUID
	CategoryName  *string // denormalized
	CategoryColor *string // denormalized
	Metadata      json.RawMessage
	CreatorID     int16
	CreatorName   string // denormalized
	IsPublic      bool
	CreatedAt     time.Time // extracted from UUID v7 via UUIDCreatedAt
}

// TagRule defines an auto-tagging rule: when WhenTagID is applied,
// ThenTagID is automatically applied as well.
type TagRule struct {
	WhenTagID   uuid.UUID
	ThenTagID   uuid.UUID
	ThenTagName string // denormalized
	IsActive    bool
}

// TagOffsetPage is an offset-based page of tags.
type TagOffsetPage struct {
	Items  []Tag
	Total  int
	Offset int
	Limit  int
}
