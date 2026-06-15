package service

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

const tagObjectType = "tag"
const tagObjectTypeID int16 = 2 // second row in 007_seed_data.sql object_types

// TagParams holds the fields for creating or patching a tag.
type TagParams struct {
	Name       string
	Notes      *string
	Color      *string    // nil = no change; pointer to empty string = clear
	CategoryID *uuid.UUID // nil = no change; Nil UUID = unassign
	Metadata   json.RawMessage
	IsPublic   *bool
}

// TagService handles tag CRUD, tag-rule management, and file–tag operations
// including automatic recursive rule application.
type TagService struct {
	tags  port.TagRepo
	rules port.TagRuleRepo
	acl   *ACLService
	audit *AuditService
	tx    port.Transactor
}

// NewTagService creates a TagService.
func NewTagService(
	tags port.TagRepo,
	rules port.TagRuleRepo,
	acl *ACLService,
	audit *AuditService,
	tx port.Transactor,
) *TagService {
	return &TagService{
		tags:  tags,
		rules: rules,
		acl:   acl,
		audit: audit,
		tx:    tx,
	}
}

// ---------------------------------------------------------------------------
// Tag CRUD
// ---------------------------------------------------------------------------

// List returns a paginated, optionally filtered list of tags the caller may see.
func (s *TagService) List(ctx context.Context, params port.OffsetParams) (*domain.TagOffsetPage, error) {
	params.ViewerID, params.ViewerIsAdmin, _ = domain.UserFromContext(ctx)
	return s.tags.List(ctx, params)
}

// Get returns a tag by ID, enforcing view ACL.
func (s *TagService) Get(ctx context.Context, id uuid.UUID) (*domain.Tag, error) {
	userID, isAdmin, _ := domain.UserFromContext(ctx)
	t, err := s.tags.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	ok, err := s.acl.CanView(ctx, userID, isAdmin, t.CreatorID, t.IsPublic, tagObjectTypeID, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}
	return t, nil
}

// Create inserts a new tag record.
func (s *TagService) Create(ctx context.Context, p TagParams) (*domain.Tag, error) {
	userID, _, _ := domain.UserFromContext(ctx)

	t := &domain.Tag{
		Name:       p.Name,
		Notes:      p.Notes,
		Color:      p.Color,
		CategoryID: p.CategoryID,
		Metadata:   p.Metadata,
		CreatorID:  userID,
	}
	if p.IsPublic != nil {
		t.IsPublic = *p.IsPublic
	}

	created, err := s.tags.Create(ctx, t)
	if err != nil {
		return nil, err
	}

	objType := tagObjectType
	_ = s.audit.Log(ctx, "tag_create", &objType, &created.ID, nil)
	return created, nil
}

// Update applies a partial patch to a tag.
// The service reads the current tag first so the caller only needs to supply
// the fields that should change.
func (s *TagService) Update(ctx context.Context, id uuid.UUID, p TagParams) (*domain.Tag, error) {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	current, err := s.tags.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	ok, err := s.acl.CanEdit(ctx, userID, isAdmin, current.CreatorID, tagObjectTypeID, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}

	// Merge patch into current.
	patch := *current // copy
	if p.Name != "" {
		patch.Name = p.Name
	}
	if p.Notes != nil {
		patch.Notes = p.Notes
	}
	if p.Color != nil {
		patch.Color = p.Color
	}
	if p.CategoryID != nil {
		if *p.CategoryID == uuid.Nil {
			patch.CategoryID = nil // explicit unassign
		} else {
			patch.CategoryID = p.CategoryID
		}
	}
	if len(p.Metadata) > 0 {
		patch.Metadata = p.Metadata
	}
	if p.IsPublic != nil {
		patch.IsPublic = *p.IsPublic
	}

	updated, err := s.tags.Update(ctx, id, &patch)
	if err != nil {
		return nil, err
	}

	objType := tagObjectType
	_ = s.audit.Log(ctx, "tag_edit", &objType, &id, nil)
	return updated, nil
}

// Delete removes a tag by ID, enforcing edit ACL.
func (s *TagService) Delete(ctx context.Context, id uuid.UUID) error {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	t, err := s.tags.GetByID(ctx, id)
	if err != nil {
		return err
	}

	ok, err := s.acl.CanEdit(ctx, userID, isAdmin, t.CreatorID, tagObjectTypeID, id)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}

	if err := s.tags.Delete(ctx, id); err != nil {
		return err
	}

	objType := tagObjectType
	_ = s.audit.Log(ctx, "tag_delete", &objType, &id, nil)
	return nil
}

// ---------------------------------------------------------------------------
// Tag rules
// ---------------------------------------------------------------------------

// ListRules returns all rules for a tag (when this tag is applied, these follow).
func (s *TagService) ListRules(ctx context.Context, tagID uuid.UUID) ([]domain.TagRule, error) {
	return s.rules.ListByTag(ctx, tagID)
}

// CreateRule adds a tag rule. When the rule is active and applyToExisting is
// true, the full transitive expansion of thenTagID is retroactively applied to
// every file already carrying whenTagID — same semantics as activating an
// existing rule via SetRuleActive. The insert and retroactive apply run in one
// transaction so a file is never left half-tagged.
func (s *TagService) CreateRule(ctx context.Context, whenTagID, thenTagID uuid.UUID, isActive, applyToExisting bool) (*domain.TagRule, error) {
	var created *domain.TagRule
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		rule, err := s.rules.Create(ctx, domain.TagRule{
			WhenTagID: whenTagID,
			ThenTagID: thenTagID,
			IsActive:  isActive,
		})
		if err != nil {
			return err
		}
		created = rule
		if isActive && applyToExisting {
			return s.rules.ApplyToExisting(ctx, whenTagID, thenTagID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// SetRuleActive toggles a rule's is_active flag and returns the updated rule.
// When active and applyToExisting are both true, the full transitive expansion
// of thenTagID is retroactively applied to files already carrying whenTagID.
func (s *TagService) SetRuleActive(ctx context.Context, whenTagID, thenTagID uuid.UUID, active, applyToExisting bool) (*domain.TagRule, error) {
	if err := s.rules.SetActive(ctx, whenTagID, thenTagID, active, applyToExisting); err != nil {
		return nil, err
	}
	rules, err := s.rules.ListByTag(ctx, whenTagID)
	if err != nil {
		return nil, err
	}
	for _, r := range rules {
		if r.ThenTagID == thenTagID {
			return &r, nil
		}
	}
	return nil, domain.ErrNotFound
}

// DeleteRule removes a tag rule.
func (s *TagService) DeleteRule(ctx context.Context, whenTagID, thenTagID uuid.UUID) error {
	return s.rules.Delete(ctx, whenTagID, thenTagID)
}

// ---------------------------------------------------------------------------
// File–tag operations (with auto-rule expansion)
// ---------------------------------------------------------------------------

// ListFileTags returns the tags on a file.
func (s *TagService) ListFileTags(ctx context.Context, fileID uuid.UUID) ([]domain.Tag, error) {
	return s.tags.ListByFile(ctx, fileID)
}

// SetFileTags replaces all tags on a file, then applies active rules for all
// newly set tags (BFS expansion). Returns the full resulting tag set.
func (s *TagService) SetFileTags(ctx context.Context, fileID uuid.UUID, tagIDs []uuid.UUID) ([]domain.Tag, error) {
	expanded, err := s.expandTagSet(ctx, tagIDs)
	if err != nil {
		return nil, err
	}

	if err := s.tags.SetFileTags(ctx, fileID, expanded); err != nil {
		return nil, err
	}

	objType := fileObjectType
	_ = s.audit.Log(ctx, "file_tag_add", &objType, &fileID, nil)
	return s.tags.ListByFile(ctx, fileID)
}

// AddFileTag adds a single tag to a file, then recursively applies active rules.
// Returns the full resulting tag set.
func (s *TagService) AddFileTag(ctx context.Context, fileID, tagID uuid.UUID) ([]domain.Tag, error) {
	// Compute the full set including rule-expansion from tagID.
	extra, err := s.expandTagSet(ctx, []uuid.UUID{tagID})
	if err != nil {
		return nil, err
	}

	// Fetch current tags so we don't lose them.
	current, err := s.tags.ListByFile(ctx, fileID)
	if err != nil {
		return nil, err
	}

	// Union: existing + expanded new tags.
	seen := make(map[uuid.UUID]bool, len(current)+len(extra))
	for _, t := range current {
		seen[t.ID] = true
	}
	merged := make([]uuid.UUID, len(current))
	for i, t := range current {
		merged[i] = t.ID
	}
	for _, id := range extra {
		if !seen[id] {
			seen[id] = true
			merged = append(merged, id)
		}
	}

	if err := s.tags.SetFileTags(ctx, fileID, merged); err != nil {
		return nil, err
	}

	objType := fileObjectType
	_ = s.audit.Log(ctx, "file_tag_add", &objType, &fileID, map[string]any{"tag_id": tagID})
	return s.tags.ListByFile(ctx, fileID)
}

// RemoveFileTag removes a single tag from a file.
func (s *TagService) RemoveFileTag(ctx context.Context, fileID, tagID uuid.UUID) error {
	if err := s.tags.RemoveFileTag(ctx, fileID, tagID); err != nil {
		return err
	}

	objType := fileObjectType
	_ = s.audit.Log(ctx, "file_tag_remove", &objType, &fileID, map[string]any{"tag_id": tagID})
	return nil
}

// BulkSetTags adds or removes tags on multiple files (with rule expansion for add).
// Returns the tagIDs that were applied (the expanded input set for add; empty for remove).
func (s *TagService) BulkSetTags(ctx context.Context, fileIDs []uuid.UUID, action string, tagIDs []uuid.UUID) ([]uuid.UUID, error) {
	if action != "add" && action != "remove" {
		return nil, domain.ErrValidation
	}

	// Pre-expand tag set once; all files get the same expansion.
	var expanded []uuid.UUID
	if action == "add" {
		var err error
		expanded, err = s.expandTagSet(ctx, tagIDs)
		if err != nil {
			return nil, err
		}
	}

	for _, fileID := range fileIDs {
		switch action {
		case "add":
			current, err := s.tags.ListByFile(ctx, fileID)
			if err != nil {
				if err == domain.ErrNotFound {
					continue
				}
				return nil, err
			}
			seen := make(map[uuid.UUID]bool, len(current))
			merged := make([]uuid.UUID, len(current))
			for i, t := range current {
				seen[t.ID] = true
				merged[i] = t.ID
			}
			for _, id := range expanded {
				if !seen[id] {
					seen[id] = true
					merged = append(merged, id)
				}
			}
			if err := s.tags.SetFileTags(ctx, fileID, merged); err != nil {
				return nil, err
			}
		case "remove":
			current, err := s.tags.ListByFile(ctx, fileID)
			if err != nil {
				if err == domain.ErrNotFound {
					continue
				}
				return nil, err
			}
			remove := make(map[uuid.UUID]bool, len(tagIDs))
			for _, id := range tagIDs {
				remove[id] = true
			}
			kept := make([]uuid.UUID, 0, len(current))
			for _, t := range current {
				if !remove[t.ID] {
					kept = append(kept, t.ID)
				}
			}
			if err := s.tags.SetFileTags(ctx, fileID, kept); err != nil {
				return nil, err
			}
		}
	}

	if action == "add" {
		return expanded, nil
	}
	return []uuid.UUID{}, nil
}

// CommonTags returns tags present on ALL given files and tags present on SOME.
func (s *TagService) CommonTags(ctx context.Context, fileIDs []uuid.UUID) (common, partial []domain.Tag, err error) {
	common, err = s.tags.CommonTagsForFiles(ctx, fileIDs)
	if err != nil {
		return nil, nil, err
	}
	partial, err = s.tags.PartialTagsForFiles(ctx, fileIDs)
	if err != nil {
		return nil, nil, err
	}
	return common, partial, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// expandTagSet runs a BFS from the given seed tags, following active tag rules,
// and returns the full set of tag IDs that should be applied (seeds + auto-applied).
func (s *TagService) expandTagSet(ctx context.Context, seeds []uuid.UUID) ([]uuid.UUID, error) {
	visited := make(map[uuid.UUID]bool, len(seeds))
	queue := make([]uuid.UUID, 0, len(seeds))

	for _, id := range seeds {
		if !visited[id] {
			visited[id] = true
			queue = append(queue, id)
		}
	}

	for i := 0; i < len(queue); i++ {
		tagID := queue[i]
		rules, err := s.rules.ListByTag(ctx, tagID)
		if err != nil {
			return nil, err
		}
		for _, r := range rules {
			if r.IsActive && !visited[r.ThenTagID] {
				visited[r.ThenTagID] = true
				queue = append(queue, r.ThenTagID)
			}
		}
	}

	return queue, nil
}
