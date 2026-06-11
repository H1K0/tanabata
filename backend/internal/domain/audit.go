package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ActionType is a reference entity for auditable user actions.
type ActionType struct {
	ID   int16
	Name string
}

// AuditEntry is a single audit log record.
type AuditEntry struct {
	ID          int64
	UserID      int16
	UserName    string // denormalized
	Action      string // action type name, e.g. "file_create"
	ObjectType  *string
	ObjectID    *uuid.UUID
	Details     json.RawMessage
	PerformedAt time.Time
}

// AuditPage is an offset-based page of audit log entries.
type AuditPage struct {
	Items  []AuditEntry
	Total  int
	Offset int
	Limit  int
}

// AuditFilter holds filter parameters for querying the audit log.
type AuditFilter struct {
	UserID     *int16
	Action     string
	ObjectType string
	ObjectID   *uuid.UUID
	From       *time.Time
	To         *time.Time
	Offset     int
	Limit      int
}
