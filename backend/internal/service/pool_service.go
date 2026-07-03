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

// PoolParams holds the fields for creating or patching a pool. SortKey and
// SortOrder are pointers so a patch can leave them unchanged (nil) vs set them.
type PoolParams struct {
	Name      string
	Notes     *string
	Metadata  json.RawMessage
	IsPublic  *bool
	SortKey   *string
	SortOrder *string
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

// RecordView appends a view-history entry for the current user, enforcing view
// ACL (you can only record a view of a pool you may see).
func (s *PoolService) RecordView(ctx context.Context, id uuid.UUID) error {
	userID, _, _ := domain.UserFromContext(ctx)
	if err := s.authorizeView(ctx, id); err != nil {
		return err
	}
	return s.pools.RecordView(ctx, id, userID)
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
	if p.SortKey != nil {
		if !domain.ValidPoolSortKey(*p.SortKey) {
			return nil, domain.ErrValidation
		}
		patch.SortKey = *p.SortKey
	}
	if p.SortOrder != nil {
		if !domain.ValidSortOrder(*p.SortOrder) {
			return nil, domain.ErrValidation
		}
		patch.SortOrder = *p.SortOrder
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

// ListFiles returns cursor-paginated files within a pool, enforcing view ACL.
// The ordering is the pool's own stored sort setting (manual position order or
// an automatic file-field sort), not a request parameter.
func (s *PoolService) ListFiles(ctx context.Context, poolID uuid.UUID, params port.PoolFileListParams) (*domain.PoolFilePage, error) {
	userID, isAdmin, _ := domain.UserFromContext(ctx)
	pool, err := s.pools.GetByID(ctx, poolID)
	if err != nil {
		return nil, err
	}
	ok, err := s.acl.CanView(ctx, userID, isAdmin, pool.CreatorID, pool.IsPublic, poolObjectTypeID, poolID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}
	params.SortKey = pool.SortKey
	params.SortOrder = pool.SortOrder
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
// ACL on the pool. Manual ordering only applies when the pool's sort key is
// "manual"; reordering an auto-sorted pool is rejected as a validation error.
func (s *PoolService) Reorder(ctx context.Context, poolID uuid.UUID, fileIDs []uuid.UUID) error {
	userID, isAdmin, _ := domain.UserFromContext(ctx)
	pool, err := s.pools.GetByID(ctx, poolID)
	if err != nil {
		return err
	}
	ok, err := s.acl.CanEdit(ctx, userID, isAdmin, pool.CreatorID, poolObjectTypeID, poolID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	if pool.SortKey != domain.PoolSortManual {
		return domain.ErrValidation
	}
	if err := s.pools.Reorder(ctx, poolID, fileIDs); err != nil {
		return err
	}
	objType := poolObjectType
	_ = s.audit.Log(ctx, "file_pool_reorder", &objType, &poolID, map[string]any{"count": len(fileIDs)})
	return nil
}
