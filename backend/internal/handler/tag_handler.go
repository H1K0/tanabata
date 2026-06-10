package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
	"tanabata/backend/internal/service"
)

// TagHandler handles all /tags endpoints.
type TagHandler struct {
	tagSvc  *service.TagService
	fileSvc *service.FileService
}

// NewTagHandler creates a TagHandler.
func NewTagHandler(tagSvc *service.TagService, fileSvc *service.FileService) *TagHandler {
	return &TagHandler{tagSvc: tagSvc, fileSvc: fileSvc}
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

type tagRuleJSON struct {
	WhenTagID   string `json:"when_tag_id"`
	ThenTagID   string `json:"then_tag_id"`
	ThenTagName string `json:"then_tag_name"`
	IsActive    bool   `json:"is_active"`
}

func toTagRuleJSON(r domain.TagRule) tagRuleJSON {
	return tagRuleJSON{
		WhenTagID:   r.WhenTagID.String(),
		ThenTagID:   r.ThenTagID.String(),
		ThenTagName: r.ThenTagName,
		IsActive:    r.IsActive,
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseTagID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("tag_id"))
	if err != nil {
		respondError(c, domain.ErrValidation)
		return uuid.UUID{}, false
	}
	return id, true
}

func parseOffsetParams(c *gin.Context, defaultSort string) port.OffsetParams {
	limit := 50
	if s := c.Query("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	offset := 0
	if s := c.Query("offset"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			offset = n
		}
	}
	sort := c.DefaultQuery("sort", defaultSort)
	order := c.DefaultQuery("order", "desc")
	search := c.Query("search")
	return port.OffsetParams{Sort: sort, Order: order, Search: search, Limit: limit, Offset: offset}
}

// ---------------------------------------------------------------------------
// GET /tags
// ---------------------------------------------------------------------------

func (h *TagHandler) List(c *gin.Context) {
	params := parseOffsetParams(c, "created")

	page, err := h.tagSvc.List(c.Request.Context(), params)
	if err != nil {
		respondError(c, err)
		return
	}

	items := make([]tagJSON, len(page.Items))
	for i, t := range page.Items {
		items[i] = toTagJSON(t)
	}
	respondJSON(c, http.StatusOK, gin.H{
		"items":  items,
		"total":  page.Total,
		"offset": page.Offset,
		"limit":  page.Limit,
	})
}

// ---------------------------------------------------------------------------
// POST /tags
// ---------------------------------------------------------------------------

func (h *TagHandler) Create(c *gin.Context) {
	var body struct {
		Name       string   `json:"name"       binding:"required"`
		Notes      *string  `json:"notes"`
		Color      *string  `json:"color"`
		CategoryID *string  `json:"category_id"`
		IsPublic   *bool    `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	params := service.TagParams{
		Name:     body.Name,
		Notes:    body.Notes,
		Color:    body.Color,
		IsPublic: body.IsPublic,
	}
	if body.CategoryID != nil {
		id, err := uuid.Parse(*body.CategoryID)
		if err != nil {
			respondError(c, domain.ErrValidation)
			return
		}
		params.CategoryID = &id
	}

	t, err := h.tagSvc.Create(c.Request.Context(), params)
	if err != nil {
		respondError(c, err)
		return
	}

	respondJSON(c, http.StatusCreated, toTagJSON(*t))
}

// ---------------------------------------------------------------------------
// GET /tags/:tag_id
// ---------------------------------------------------------------------------

func (h *TagHandler) Get(c *gin.Context) {
	id, ok := parseTagID(c)
	if !ok {
		return
	}

	t, err := h.tagSvc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}

	respondJSON(c, http.StatusOK, toTagJSON(*t))
}

// ---------------------------------------------------------------------------
// PATCH /tags/:tag_id
// ---------------------------------------------------------------------------

func (h *TagHandler) Update(c *gin.Context) {
	id, ok := parseTagID(c)
	if !ok {
		return
	}

	// Use a raw map to distinguish "field absent" from "field = null".
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	params := service.TagParams{}

	if v, ok := raw["name"]; ok {
		if s, ok := v.(string); ok {
			params.Name = s
		}
	}
	if _, ok := raw["notes"]; ok {
		if raw["notes"] == nil {
			params.Notes = ptr("")
		} else if s, ok := raw["notes"].(string); ok {
			params.Notes = &s
		}
	}
	if _, ok := raw["color"]; ok {
		if raw["color"] == nil {
			nilStr := ""
			params.Color = &nilStr
		} else if s, ok := raw["color"].(string); ok {
			params.Color = &s
		}
	}
	if _, ok := raw["category_id"]; ok {
		if raw["category_id"] == nil {
			nilID := uuid.Nil
			params.CategoryID = &nilID // signals "unassign"
		} else if s, ok := raw["category_id"].(string); ok {
			cid, err := uuid.Parse(s)
			if err != nil {
				respondError(c, domain.ErrValidation)
				return
			}
			params.CategoryID = &cid
		}
	}
	if v, ok := raw["is_public"]; ok {
		if b, ok := v.(bool); ok {
			params.IsPublic = &b
		}
	}

	t, err := h.tagSvc.Update(c.Request.Context(), id, params)
	if err != nil {
		respondError(c, err)
		return
	}

	respondJSON(c, http.StatusOK, toTagJSON(*t))
}

// ---------------------------------------------------------------------------
// DELETE /tags/:tag_id
// ---------------------------------------------------------------------------

func (h *TagHandler) Delete(c *gin.Context) {
	id, ok := parseTagID(c)
	if !ok {
		return
	}

	if err := h.tagSvc.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// GET /tags/:tag_id/files
// ---------------------------------------------------------------------------

func (h *TagHandler) ListFiles(c *gin.Context) {
	id, ok := parseTagID(c)
	if !ok {
		return
	}

	limit := 50
	if s := c.Query("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	// Delegate to file service with a tag filter.
	page, err := h.fileSvc.List(c.Request.Context(), domain.FileListParams{
		Cursor:    c.Query("cursor"),
		Direction: "forward",
		Limit:     limit,
		Sort:      "created",
		Order:     "desc",
		Filter:    "{t=" + id.String() + "}",
	})
	if err != nil {
		respondError(c, err)
		return
	}

	items := make([]fileJSON, len(page.Items))
	for i, f := range page.Items {
		items[i] = toFileJSON(f)
	}
	respondJSON(c, http.StatusOK, gin.H{
		"items":       items,
		"next_cursor": page.NextCursor,
		"prev_cursor": page.PrevCursor,
	})
}

// ---------------------------------------------------------------------------
// GET /tags/:tag_id/rules
// ---------------------------------------------------------------------------

func (h *TagHandler) ListRules(c *gin.Context) {
	id, ok := parseTagID(c)
	if !ok {
		return
	}

	rules, err := h.tagSvc.ListRules(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}

	items := make([]tagRuleJSON, len(rules))
	for i, r := range rules {
		items[i] = toTagRuleJSON(r)
	}
	respondJSON(c, http.StatusOK, items)
}

// ---------------------------------------------------------------------------
// POST /tags/:tag_id/rules
// ---------------------------------------------------------------------------

func (h *TagHandler) CreateRule(c *gin.Context) {
	whenTagID, ok := parseTagID(c)
	if !ok {
		return
	}

	var body struct {
		ThenTagID       string `json:"then_tag_id" binding:"required"`
		IsActive        *bool  `json:"is_active"`
		ApplyToExisting *bool  `json:"apply_to_existing"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	thenTagID, err := uuid.Parse(body.ThenTagID)
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	isActive := true
	if body.IsActive != nil {
		isActive = *body.IsActive
	}
	applyToExisting := true
	if body.ApplyToExisting != nil {
		applyToExisting = *body.ApplyToExisting
	}

	rule, err := h.tagSvc.CreateRule(c.Request.Context(), whenTagID, thenTagID, isActive, applyToExisting)
	if err != nil {
		respondError(c, err)
		return
	}

	respondJSON(c, http.StatusCreated, toTagRuleJSON(*rule))
}

// ---------------------------------------------------------------------------
// PATCH /tags/:tag_id/rules/:then_tag_id
// ---------------------------------------------------------------------------

func (h *TagHandler) PatchRule(c *gin.Context) {
	whenTagID, ok := parseTagID(c)
	if !ok {
		return
	}

	thenTagID, err := uuid.Parse(c.Param("then_tag_id"))
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	var body struct {
		IsActive        *bool `json:"is_active"`
		ApplyToExisting *bool `json:"apply_to_existing"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.IsActive == nil {
		respondError(c, domain.ErrValidation)
		return
	}

	applyToExisting := false
	if body.ApplyToExisting != nil {
		applyToExisting = *body.ApplyToExisting
	}

	rule, err := h.tagSvc.SetRuleActive(c.Request.Context(), whenTagID, thenTagID, *body.IsActive, applyToExisting)
	if err != nil {
		respondError(c, err)
		return
	}

	respondJSON(c, http.StatusOK, toTagRuleJSON(*rule))
}

// ---------------------------------------------------------------------------
// DELETE /tags/:tag_id/rules/:then_tag_id
// ---------------------------------------------------------------------------

func (h *TagHandler) DeleteRule(c *gin.Context) {
	whenTagID, ok := parseTagID(c)
	if !ok {
		return
	}

	thenTagID, err := uuid.Parse(c.Param("then_tag_id"))
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	if err := h.tagSvc.DeleteRule(c.Request.Context(), whenTagID, thenTagID); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// File-tag endpoints wired through TagService
// (called from file routes, shared handler logic lives here)
// ---------------------------------------------------------------------------

// FileListTags handles GET /files/:id/tags.
func (h *TagHandler) FileListTags(c *gin.Context) {
	fileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	if err := h.fileSvc.AuthorizeView(c.Request.Context(), fileID); err != nil {
		respondError(c, err)
		return
	}

	tags, err := h.tagSvc.ListFileTags(c.Request.Context(), fileID)
	if err != nil {
		respondError(c, err)
		return
	}

	items := make([]tagJSON, len(tags))
	for i, t := range tags {
		items[i] = toTagJSON(t)
	}
	respondJSON(c, http.StatusOK, items)
}

// FileSetTags handles PUT /files/:id/tags.
func (h *TagHandler) FileSetTags(c *gin.Context) {
	fileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	var body struct {
		TagIDs []string `json:"tag_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	tagIDs, err := parseUUIDs(body.TagIDs)
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	if err := h.fileSvc.AuthorizeEdit(c.Request.Context(), fileID); err != nil {
		respondError(c, err)
		return
	}

	tags, err := h.tagSvc.SetFileTags(c.Request.Context(), fileID, tagIDs)
	if err != nil {
		respondError(c, err)
		return
	}

	items := make([]tagJSON, len(tags))
	for i, t := range tags {
		items[i] = toTagJSON(t)
	}
	respondJSON(c, http.StatusOK, items)
}

// FileAddTag handles PUT /files/:id/tags/:tag_id.
func (h *TagHandler) FileAddTag(c *gin.Context) {
	fileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}
	tagID, err := uuid.Parse(c.Param("tag_id"))
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	if err := h.fileSvc.AuthorizeEdit(c.Request.Context(), fileID); err != nil {
		respondError(c, err)
		return
	}

	tags, err := h.tagSvc.AddFileTag(c.Request.Context(), fileID, tagID)
	if err != nil {
		respondError(c, err)
		return
	}

	items := make([]tagJSON, len(tags))
	for i, t := range tags {
		items[i] = toTagJSON(t)
	}
	respondJSON(c, http.StatusOK, items)
}

// FileRemoveTag handles DELETE /files/:id/tags/:tag_id.
func (h *TagHandler) FileRemoveTag(c *gin.Context) {
	fileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}
	tagID, err := uuid.Parse(c.Param("tag_id"))
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	if err := h.fileSvc.AuthorizeEdit(c.Request.Context(), fileID); err != nil {
		respondError(c, err)
		return
	}

	if err := h.tagSvc.RemoveFileTag(c.Request.Context(), fileID, tagID); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func ptr(s string) *string { return &s }