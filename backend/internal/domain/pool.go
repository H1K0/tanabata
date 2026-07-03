package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Pool file sort keys. PoolSortManual keeps the user-defined order in
// file_pool.position; the others sort the pool's files by that file field.
const (
	PoolSortManual          = "manual"
	PoolSortContentDatetime = "content_datetime"
	PoolSortCreated         = "created"
	PoolSortOriginalName    = "original_name"

	SortOrderAsc  = "asc"
	SortOrderDesc = "desc"
)

// ValidPoolSortKey reports whether s is an accepted pool sort key.
func ValidPoolSortKey(s string) bool {
	switch s {
	case PoolSortManual, PoolSortContentDatetime, PoolSortCreated, PoolSortOriginalName:
		return true
	}
	return false
}

// ValidSortOrder reports whether s is an accepted sort direction.
func ValidSortOrder(s string) bool {
	return s == SortOrderAsc || s == SortOrderDesc
}

// Pool is an ordered collection of files.
type Pool struct {
	ID          uuid.UUID
	Name        string
	Notes       *string
	Metadata    json.RawMessage
	CreatorID   int16
	CreatorName string // denormalized
	IsPublic    bool
	// SortKey / SortOrder control how the pool's files are ordered. When SortKey
	// is PoolSortManual, files follow the manual position order and can be
	// reordered; otherwise they are sorted automatically and reordering is a no-op.
	SortKey     string
	SortOrder   string
	FileCount   int
	CreatedAt   time.Time // extracted from UUID v7 via UUIDCreatedAt
}

// PoolFile is a File with its ordering position within a pool.
type PoolFile struct {
	File
	Position int
}

// PoolFilePage is the result of a cursor-based pool file listing.
type PoolFilePage struct {
	Items      []PoolFile
	NextCursor *string
}

// PoolOffsetPage is an offset-based page of pools.
type PoolOffsetPage struct {
	Items  []Pool
	Total  int
	Offset int
	Limit  int
}
