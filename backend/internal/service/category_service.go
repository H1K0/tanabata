package service

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

const categoryObjectType          = "category"
const categoryObjectTypeID int16  = 3 // third row in 007_seed_data.sql object_types

// CategoryParams holds the fields for creating or patching a category.
type CategoryParams struct {
	Name     string
	Notes    *string
	Color    *string         // nil = no change; pointer to empty string = clear
	Metadata json.RawMessage
	IsPublic *bool
}

// CategoryService handles category CRUD with ACL enforcement and audit logging.
type CategoryService struct {
	categories port.CategoryRepo
	tags       port.TagRepo
	acl        *ACLService
	audit      *AuditService
}

// NewCategoryService creates a CategoryService.
func NewCategoryService(
	categories port.CategoryRepo,
	tags port.TagRepo,
	acl *ACLService,
	audit *AuditService,
) *CategoryService {
	return &CategoryService{
		categories: categories,
		tags:       tags,
		acl:        acl,
		audit:      audit,
	}
}

// ---------------------------------------------------------------------------
// CRUD
// ---------------------------------------------------------------------------

// List returns a paginated list of categories the caller may see.
func (s *CategoryService) List(ctx context.Context, params port.OffsetParams) (*domain.CategoryOffsetPage, error) {
	params.ViewerID, params.ViewerIsAdmin, _ = domain.UserFromContext(ctx)
	return s.categories.List(ctx, params)
}

// Get returns a category by ID, enforcing view ACL.
func (s *CategoryService) Get(ctx context.Context, id uuid.UUID) (*domain.Category, error) {
	userID, isAdmin, _ := domain.UserFromContext(ctx)
	c, err := s.categories.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	ok, err := s.acl.CanView(ctx, userID, isAdmin, c.CreatorID, c.IsPublic, categoryObjectTypeID, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}
	return c, nil
}

// Create inserts a new category record.
func (s *CategoryService) Create(ctx context.Context, p CategoryParams) (*domain.Category, error) {
	userID, _, _ := domain.UserFromContext(ctx)

	c := &domain.Category{
		Name:      p.Name,
		Notes:     p.Notes,
		Color:     p.Color,
		Metadata:  p.Metadata,
		CreatorID: userID,
	}
	if p.IsPublic != nil {
		c.IsPublic = *p.IsPublic
	}

	created, err := s.categories.Create(ctx, c)
	if err != nil {
		return nil, err
	}

	objType := categoryObjectType
	_ = s.audit.Log(ctx, "category_create", &objType, &created.ID, nil)
	return created, nil
}

// Update applies a partial patch to a category.
func (s *CategoryService) Update(ctx context.Context, id uuid.UUID, p CategoryParams) (*domain.Category, error) {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	current, err := s.categories.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	ok, err := s.acl.CanEdit(ctx, userID, isAdmin, current.CreatorID, categoryObjectTypeID, id)
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
	if p.Color != nil {
		patch.Color = p.Color
	}
	if len(p.Metadata) > 0 {
		patch.Metadata = p.Metadata
	}
	if p.IsPublic != nil {
		patch.IsPublic = *p.IsPublic
	}

	updated, err := s.categories.Update(ctx, id, &patch)
	if err != nil {
		return nil, err
	}

	objType := categoryObjectType
	_ = s.audit.Log(ctx, "category_edit", &objType, &id, nil)
	return updated, nil
}

// Delete removes a category by ID, enforcing edit ACL.
func (s *CategoryService) Delete(ctx context.Context, id uuid.UUID) error {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	c, err := s.categories.GetByID(ctx, id)
	if err != nil {
		return err
	}

	ok, err := s.acl.CanEdit(ctx, userID, isAdmin, c.CreatorID, categoryObjectTypeID, id)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}

	if err := s.categories.Delete(ctx, id); err != nil {
		return err
	}

	objType := categoryObjectType
	_ = s.audit.Log(ctx, "category_delete", &objType, &id, nil)
	return nil
}

// ---------------------------------------------------------------------------
// Tags in category
// ---------------------------------------------------------------------------

// ListTags returns a paginated list of tags in this category that the caller
// may see.
func (s *CategoryService) ListTags(ctx context.Context, categoryID uuid.UUID, params port.OffsetParams) (*domain.TagOffsetPage, error) {
	params.ViewerID, params.ViewerIsAdmin, _ = domain.UserFromContext(ctx)
	return s.tags.ListByCategory(ctx, categoryID, params)
}