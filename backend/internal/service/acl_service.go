package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

// ACLService handles access control checks and permission management.
type ACLService struct {
	aclRepo port.ACLRepo
}

func NewACLService(aclRepo port.ACLRepo) *ACLService {
	return &ACLService{aclRepo: aclRepo}
}

// CanView returns true if the user may view the object.
// isAdmin, creatorID, isPublic must be populated from the object record by the caller.
func (s *ACLService) CanView(
	ctx context.Context,
	userID int16, isAdmin bool,
	creatorID int16, isPublic bool,
	objectTypeID int16, objectID uuid.UUID,
) (bool, error) {
	if isAdmin {
		return true, nil
	}
	if isPublic {
		return true, nil
	}
	if userID == creatorID {
		return true, nil
	}
	perm, err := s.aclRepo.Get(ctx, userID, objectTypeID, objectID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return perm.CanView, nil
}

// CanEdit returns true if the user may edit the object.
// is_public does not grant edit access; only admins, creators, and explicit grants.
func (s *ACLService) CanEdit(
	ctx context.Context,
	userID int16, isAdmin bool,
	creatorID int16,
	objectTypeID int16, objectID uuid.UUID,
) (bool, error) {
	if isAdmin {
		return true, nil
	}
	if userID == creatorID {
		return true, nil
	}
	perm, err := s.aclRepo.Get(ctx, userID, objectTypeID, objectID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return perm.CanEdit, nil
}

// GetPermissions returns all explicit ACL entries for an object.
func (s *ACLService) GetPermissions(ctx context.Context, objectTypeID int16, objectID uuid.UUID) ([]domain.Permission, error) {
	return s.aclRepo.List(ctx, objectTypeID, objectID)
}

// SetPermissions replaces all ACL entries for an object (full replace semantics).
func (s *ACLService) SetPermissions(ctx context.Context, objectTypeID int16, objectID uuid.UUID, perms []domain.Permission) error {
	return s.aclRepo.Set(ctx, objectTypeID, objectID, perms)
}
