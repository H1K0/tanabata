package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Category is a logical grouping of tags.
type Category struct {
	ID          uuid.UUID
	Name        string
	Notes       *string
	Color       *string // 6-char hex
	Metadata    json.RawMessage
	CreatorID   int16
	CreatorName string // denormalized
	IsPublic    bool
	CreatedAt   time.Time // extracted from UUID v7 via UUIDCreatedAt
}

// CategoryOffsetPage is an offset-based page of categories.
type CategoryOffsetPage struct {
	Items  []Category
	Total  int
	Offset int
	Limit  int
}
