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
	aclRepo    port.ACLRepo
	files      port.FileRepo
	tags       port.TagRepo
	categories port.CategoryRepo
	pools      port.PoolRepo
	tx         port.Transactor
}

// NewACLService creates an ACLService. The object repositories are used to
// resolve an object's owner when authorizing permission management.
func NewACLService(
	aclRepo port.ACLRepo,
	files port.FileRepo,
	tags port.TagRepo,
	categories port.CategoryRepo,
	pools port.PoolRepo,
	tx port.Transactor,
) *ACLService {
	return &ACLService{
		aclRepo:    aclRepo,
		files:      files,
		tags:       tags,
		categories: categories,
		pools:      pools,
		tx:         tx,
	}
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

// GetPermissions returns all explicit ACL entries for an object. Only the
// object's owner or an admin may inspect its permission list.
func (s *ACLService) GetPermissions(
	ctx context.Context,
	userID int16, isAdmin bool,
	objectTypeID int16, objectID uuid.UUID,
) ([]domain.Permission, error) {
	if err := s.authorizeManage(ctx, userID, isAdmin, objectTypeID, objectID); err != nil {
		return nil, err
	}
	return s.aclRepo.List(ctx, objectTypeID, objectID)
}

// SetPermissions replaces all ACL entries for an object (full replace semantics).
// Only the object's owner or an admin may change its permissions. The replace is
// performed atomically inside a single transaction.
func (s *ACLService) SetPermissions(
	ctx context.Context,
	userID int16, isAdmin bool,
	objectTypeID int16, objectID uuid.UUID,
	perms []domain.Permission,
) error {
	if err := s.authorizeManage(ctx, userID, isAdmin, objectTypeID, objectID); err != nil {
		return err
	}
	return s.tx.WithTx(ctx, func(ctx context.Context) error {
		return s.aclRepo.Set(ctx, objectTypeID, objectID, perms)
	})
}

// authorizeManage returns nil if the user may manage the object's ACL
// (admin or owner), ErrForbidden otherwise, or a propagated lookup error
// (including ErrNotFound when the object does not exist).
func (s *ACLService) authorizeManage(
	ctx context.Context,
	userID int16, isAdmin bool,
	objectTypeID int16, objectID uuid.UUID,
) error {
	if isAdmin {
		return nil
	}
	owner, err := s.objectOwner(ctx, objectTypeID, objectID)
	if err != nil {
		return err
	}
	if owner != userID {
		return domain.ErrForbidden
	}
	return nil
}

// objectOwner resolves the creator ID of the object identified by
// (objectTypeID, objectID). Returns ErrNotFound if the object does not exist.
func (s *ACLService) objectOwner(ctx context.Context, objectTypeID int16, objectID uuid.UUID) (int16, error) {
	switch objectTypeID {
	case fileObjectTypeID:
		obj, err := s.files.GetByID(ctx, objectID)
		if err != nil {
			return 0, err
		}
		return obj.CreatorID, nil
	case tagObjectTypeID:
		obj, err := s.tags.GetByID(ctx, objectID)
		if err != nil {
			return 0, err
		}
		return obj.CreatorID, nil
	case categoryObjectTypeID:
		obj, err := s.categories.GetByID(ctx, objectID)
		if err != nil {
			return 0, err
		}
		return obj.CreatorID, nil
	case poolObjectTypeID:
		obj, err := s.pools.GetByID(ctx, objectID)
		if err != nil {
			return 0, err
		}
		return obj.CreatorID, nil
	default:
		return 0, domain.ErrValidation
	}
}
