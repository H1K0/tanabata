package service

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

const poolObjectType = "pool"
const poolObjectTypeID int16 = 4 // fourth row in 007_seed_data.sql object_types

// PoolParams holds the fields for creating or patching a pool.
type PoolParams struct {
	Name     string
	Notes    *string
	Metadata json.RawMessage
	IsPublic *bool
}

// PoolService handles pool CRUD and pool–file management with ACL + audit.
type PoolService struct {
	pools port.PoolRepo
	acl   *ACLService
	audit *AuditService
}

// NewPoolService creates a PoolService.
func NewPoolService(
	pools port.PoolRepo,
	acl *ACLService,
	audit *AuditService,
) *PoolService {
	return &PoolService{pools: pools, acl: acl, audit: audit}
}

// ---------------------------------------------------------------------------
// CRUD
// ---------------------------------------------------------------------------

// List returns a paginated list of pools the caller may see.
func (s *PoolService) List(ctx context.Context, params port.OffsetParams) (*domain.PoolOffsetPage, error) {
	params.ViewerID, params.ViewerIsAdmin, _ = domain.UserFromContext(ctx)
	return s.pools.List(ctx, params)
}

// Get returns a pool by ID, enforcing view ACL.
func (s *PoolService) Get(ctx context.Context, id uuid.UUID) (*domain.Pool, error) {
	userID, isAdmin, _ := domain.UserFromContext(ctx)
	p, err := s.pools.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	ok, err := s.acl.CanView(ctx, userID, isAdmin, p.CreatorID, p.IsPublic, poolObjectTypeID, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}
	return p, nil
}

// authorizeView returns nil if the caller may view the pool, else ErrForbidden
// (or ErrNotFound if the pool does not exist).
func (s *PoolService) authorizeView(ctx context.Context, poolID uuid.UUID) error {
	userID, isAdmin, _ := domain.UserFromContext(ctx)
	p, err := s.pools.GetByID(ctx, poolID)
	if err != nil {
		return err
	}
	ok, err := s.acl.CanView(ctx, userID, isAdmin, p.CreatorID, p.IsPublic, poolObjectTypeID, poolID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

// authorizeEdit returns nil if the caller may edit the pool, else ErrForbidden
// (or ErrNotFound if the pool does not exist).
func (s *PoolService) authorizeEdit(ctx context.Context, poolID uuid.UUID) error {
	userID, isAdmin, _ := domain.UserFromContext(ctx)
	p, err := s.pools.GetByID(ctx, poolID)
	if err != nil {
		return err
	}
	ok, err := s.acl.CanEdit(ctx, userID, isAdmin, p.CreatorID, poolObjectTypeID, poolID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

// Create inserts a new pool.
func (s *PoolService) Create(ctx context.Context, p PoolParams) (*domain.Pool, error) {
	userID, _, _ := domain.UserFromContext(ctx)

	pool := &domain.Pool{
		Name:      p.Name,
		Notes:     p.Notes,
		Metadata:  p.Metadata,
		CreatorID: userID,
	}
	if p.IsPublic != nil {
		pool.IsPublic = *p.IsPublic
	}

	created, err := s.pools.Create(ctx, pool)
	if err != nil {
		return nil, err
	}

	objType := poolObjectType
	_ = s.audit.Log(ctx, "pool_create", &objType, &created.ID, nil)
	return created, nil
}

// Update applies a partial patch to a pool.
func (s *PoolService) Update(ctx context.Context, id uuid.UUID, p PoolParams) (*domain.Pool, error) {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	current, err := s.pools.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	ok, err := s.acl.CanEdit(ctx, userID, isAdmin, current.CreatorID, poolObjectTypeID, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}

	patch := *current
	if p.Name != "" {
		patch.Name = p.Name
	}
	if p.Notes != nil {
		patch.Notes = p.Notes
	}
	if len(p.Metadata) > 0 {
		patch.Metadata = p.Metadata
	}
	if p.IsPublic != nil {
		patch.IsPublic = *p.IsPublic
	}

	updated, err := s.pools.Update(ctx, id, &patch)
	if err != nil {
		return nil, err
	}

	objType := poolObjectType
	_ = s.audit.Log(ctx, "pool_edit", &objType, &id, nil)
	return updated, nil
}

// Delete removes a pool by ID, enforcing edit ACL.
func (s *PoolService) Delete(ctx context.Context, id uuid.UUID) error {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	pool, err := s.pools.GetByID(ctx, id)
	if err != nil {
		return err
	}

	ok, err := s.acl.CanEdit(ctx, userID, isAdmin, pool.CreatorID, poolObjectTypeID, id)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}

	if err := s.pools.Delete(ctx, id); err != nil {
		return err
	}

	objType := poolObjectType
	_ = s.audit.Log(ctx, "pool_delete", &objType, &id, nil)
	return nil
}

// ---------------------------------------------------------------------------
// Pool–file operations
// ---------------------------------------------------------------------------

// ListFiles returns cursor-paginated files within a pool ordered by position,
// enforcing view ACL on the pool.
func (s *PoolService) ListFiles(ctx context.Context, poolID uuid.UUID, params port.PoolFileListParams) (*domain.PoolFilePage, error) {
	if err := s.authorizeView(ctx, poolID); err != nil {
		return nil, err
	}
	return s.pools.ListFiles(ctx, poolID, params)
}

// AddFiles adds files to a pool at the given position (nil = append), enforcing
// edit ACL on the pool.
func (s *PoolService) AddFiles(ctx context.Context, poolID uuid.UUID, fileIDs []uuid.UUID, position *int) error {
	if err := s.authorizeEdit(ctx, poolID); err != nil {
		return err
	}
	if err := s.pools.AddFiles(ctx, poolID, fileIDs, position); err != nil {
		return err
	}
	objType := poolObjectType
	_ = s.audit.Log(ctx, "file_pool_add", &objType, &poolID, map[string]any{"count": len(fileIDs)})
	return nil
}

// RemoveFiles removes files from a pool, enforcing edit ACL on the pool.
func (s *PoolService) RemoveFiles(ctx context.Context, poolID uuid.UUID, fileIDs []uuid.UUID) error {
	if err := s.authorizeEdit(ctx, poolID); err != nil {
		return err
	}
	if err := s.pools.RemoveFiles(ctx, poolID, fileIDs); err != nil {
		return err
	}
	objType := poolObjectType
	_ = s.audit.Log(ctx, "file_pool_remove", &objType, &poolID, map[string]any{"count": len(fileIDs)})
	return nil
}

// Reorder sets the ordered sequence of file IDs within a pool, enforcing edit
// ACL on the pool.
func (s *PoolService) Reorder(ctx context.Context, poolID uuid.UUID, fileIDs []uuid.UUID) error {
	if err := s.authorizeEdit(ctx, poolID); err != nil {
		return err
	}
	return s.pools.Reorder(ctx, poolID, fileIDs)
}
