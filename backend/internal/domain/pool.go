package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Pool is an ordered collection of files.
type Pool struct {
	ID          uuid.UUID
	Name        string
	Notes       *string
	Metadata    json.RawMessage
	CreatorID   int16
	CreatorName string // denormalized
	IsPublic    bool
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
