package domain

import "github.com/google/uuid"

// ObjectType is a reference entity (file, tag, category, pool).
type ObjectType struct {
	ID   int16
	Name string
}

// Permission represents a per-object access entry for a user.
type Permission struct {
	UserID        int16
	UserName      string // denormalized
	ObjectTypeID  int16
	ObjectID      uuid.UUID
	CanView       bool
	CanEdit       bool
}
