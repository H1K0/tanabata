package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/service"
)

// CategoryHandler handles all /categories endpoints.
type CategoryHandler struct {
	categorySvc *service.CategoryService
}

// NewCategoryHandler creates a CategoryHandler.
func NewCategoryHandler(categorySvc *service.CategoryService) *CategoryHandler {
	return &CategoryHandler{categorySvc: categorySvc}
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

type categoryJSON struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Notes       *string `json:"notes"`
	Color       *string `json:"color"`
	CreatorID   int16   `json:"creator_id"`
	CreatorName string  `json:"creator_name"`
	IsPublic    bool    `json:"is_public"`
	CreatedAt   string  `json:"created_at"`
}

func toCategoryJSON(c domain.Category) categoryJSON {
	return categoryJSON{
		ID:          c.ID.String(),
		Name:        c.Name,
		Notes:       c.Notes,
		Color:       c.Color,
		CreatorID:   c.CreatorID,
		CreatorName: c.CreatorName,
		IsPublic:    c.IsPublic,
		CreatedAt:   c.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseCategoryID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("category_id"))
	if err != nil {
		respondError(c, domain.ErrValidation)
		return uuid.UUID{}, false
	}
	return id, true
}

// ---------------------------------------------------------------------------
// GET /categories
// ---------------------------------------------------------------------------

func (h *CategoryHandler) List(c *gin.Context) {
	params := parseOffsetParams(c, "created")

	page, err := h.categorySvc.List(c.Request.Context(), params)
	if err != nil {
		respondError(c, err)
		return
	}

	items := make([]categoryJSON, len(page.Items))
	for i, cat := range page.Items {
		items[i] = toCategoryJSON(cat)
	}
	respondJSON(c, http.StatusOK, gin.H{
		"items":  items,
		"total":  page.Total,
		"offset": page.Offset,
		"limit":  page.Limit,
	})
}

// ---------------------------------------------------------------------------
// POST /categories
// ---------------------------------------------------------------------------

func (h *CategoryHandler) Create(c *gin.Context) {
	var body struct {
		Name     string  `json:"name"      binding:"required"`
		Notes    *string `json:"notes"`
		Color    *string `json:"color"`
		IsPublic *bool   `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	created, err := h.categorySvc.Create(c.Request.Context(), service.CategoryParams{
		Name:     body.Name,
		Notes:    body.Notes,
		Color:    body.Color,
		IsPublic: body.IsPublic,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusCreated, toCategoryJSON(*created))
}

// ---------------------------------------------------------------------------
// GET /categories/:category_id
// ---------------------------------------------------------------------------

func (h *CategoryHandler) Get(c *gin.Context) {
	id, ok := parseCategoryID(c)
	if !ok {
		return
	}

	cat, err := h.categorySvc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, toCategoryJSON(*cat))
}

// ---------------------------------------------------------------------------
// PATCH /categories/:category_id
// ---------------------------------------------------------------------------

func (h *CategoryHandler) Update(c *gin.Context) {
	id, ok := parseCategoryID(c)
	if !ok {
		return
	}

	// Use a raw map to detect explicitly-null fields.
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	params := service.CategoryParams{}

	if v, ok := raw["name"]; ok {
		if s, ok := v.(string); ok {
			params.Name = s
		}
	}
	if _, ok := raw["notes"]; ok {
		if raw["notes"] == nil {
			empty := ""
			params.Notes = &empty
		} else if s, ok := raw["notes"].(string); ok {
			params.Notes = &s
		}
	}
	if _, ok := raw["color"]; ok {
		if raw["color"] == nil {
			empty := ""
			params.Color = &empty
		} else if s, ok := raw["color"].(string); ok {
			params.Color = &s
		}
	}
	if v, ok := raw["is_public"]; ok {
		if b, ok := v.(bool); ok {
			params.IsPublic = &b
		}
	}

	updated, err := h.categorySvc.Update(c.Request.Context(), id, params)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, toCategoryJSON(*updated))
}

// ---------------------------------------------------------------------------
// DELETE /categories/:category_id
// ---------------------------------------------------------------------------

func (h *CategoryHandler) Delete(c *gin.Context) {
	id, ok := parseCategoryID(c)
	if !ok {
		return
	}

	if err := h.categorySvc.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// GET /categories/:category_id/tags
// ---------------------------------------------------------------------------

func (h *CategoryHandler) ListTags(c *gin.Context) {
	id, ok := parseCategoryID(c)
	if !ok {
		return
	}

	params := parseOffsetParams(c, "created")

	page, err := h.categorySvc.ListTags(c.Request.Context(), id, params)
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