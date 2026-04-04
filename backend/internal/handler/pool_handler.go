package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
	"tanabata/backend/internal/service"
)

// PoolHandler handles all /pools endpoints.
type PoolHandler struct {
	poolSvc *service.PoolService
}

// NewPoolHandler creates a PoolHandler.
func NewPoolHandler(poolSvc *service.PoolService) *PoolHandler {
	return &PoolHandler{poolSvc: poolSvc}
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

type poolJSON struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Notes       *string `json:"notes"`
	CreatorID   int16   `json:"creator_id"`
	CreatorName string  `json:"creator_name"`
	IsPublic    bool    `json:"is_public"`
	FileCount   int     `json:"file_count"`
	CreatedAt   string  `json:"created_at"`
}

type poolFileJSON struct {
	fileJSON
	Position int `json:"position"`
}

func toPoolJSON(p domain.Pool) poolJSON {
	return poolJSON{
		ID:          p.ID.String(),
		Name:        p.Name,
		Notes:       p.Notes,
		CreatorID:   p.CreatorID,
		CreatorName: p.CreatorName,
		IsPublic:    p.IsPublic,
		FileCount:   p.FileCount,
		CreatedAt:   p.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toPoolFileJSON(pf domain.PoolFile) poolFileJSON {
	return poolFileJSON{
		fileJSON: toFileJSON(pf.File),
		Position: pf.Position,
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parsePoolID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("pool_id"))
	if err != nil {
		respondError(c, domain.ErrValidation)
		return uuid.UUID{}, false
	}
	return id, true
}

func parsePoolFileParams(c *gin.Context) port.PoolFileListParams {
	limit := 50
	if s := c.Query("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	return port.PoolFileListParams{
		Cursor: c.Query("cursor"),
		Limit:  limit,
		Filter: c.Query("filter"),
	}
}

// ---------------------------------------------------------------------------
// GET /pools
// ---------------------------------------------------------------------------

func (h *PoolHandler) List(c *gin.Context) {
	params := parseOffsetParams(c, "created")

	page, err := h.poolSvc.List(c.Request.Context(), params)
	if err != nil {
		respondError(c, err)
		return
	}

	items := make([]poolJSON, len(page.Items))
	for i, p := range page.Items {
		items[i] = toPoolJSON(p)
	}
	respondJSON(c, http.StatusOK, gin.H{
		"items":  items,
		"total":  page.Total,
		"offset": page.Offset,
		"limit":  page.Limit,
	})
}

// ---------------------------------------------------------------------------
// POST /pools
// ---------------------------------------------------------------------------

func (h *PoolHandler) Create(c *gin.Context) {
	var body struct {
		Name     string  `json:"name"      binding:"required"`
		Notes    *string `json:"notes"`
		IsPublic *bool   `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	created, err := h.poolSvc.Create(c.Request.Context(), service.PoolParams{
		Name:     body.Name,
		Notes:    body.Notes,
		IsPublic: body.IsPublic,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusCreated, toPoolJSON(*created))
}

// ---------------------------------------------------------------------------
// GET /pools/:pool_id
// ---------------------------------------------------------------------------

func (h *PoolHandler) Get(c *gin.Context) {
	id, ok := parsePoolID(c)
	if !ok {
		return
	}

	p, err := h.poolSvc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, toPoolJSON(*p))
}

// ---------------------------------------------------------------------------
// PATCH /pools/:pool_id
// ---------------------------------------------------------------------------

func (h *PoolHandler) Update(c *gin.Context) {
	id, ok := parsePoolID(c)
	if !ok {
		return
	}

	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	params := service.PoolParams{}
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
	if v, ok := raw["is_public"]; ok {
		if b, ok := v.(bool); ok {
			params.IsPublic = &b
		}
	}

	updated, err := h.poolSvc.Update(c.Request.Context(), id, params)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, toPoolJSON(*updated))
}

// ---------------------------------------------------------------------------
// DELETE /pools/:pool_id
// ---------------------------------------------------------------------------

func (h *PoolHandler) Delete(c *gin.Context) {
	id, ok := parsePoolID(c)
	if !ok {
		return
	}

	if err := h.poolSvc.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// GET /pools/:pool_id/files
// ---------------------------------------------------------------------------

func (h *PoolHandler) ListFiles(c *gin.Context) {
	poolID, ok := parsePoolID(c)
	if !ok {
		return
	}

	params := parsePoolFileParams(c)

	page, err := h.poolSvc.ListFiles(c.Request.Context(), poolID, params)
	if err != nil {
		respondError(c, err)
		return
	}

	items := make([]poolFileJSON, len(page.Items))
	for i, pf := range page.Items {
		items[i] = toPoolFileJSON(pf)
	}
	respondJSON(c, http.StatusOK, gin.H{
		"items":       items,
		"next_cursor": page.NextCursor,
	})
}

// ---------------------------------------------------------------------------
// POST /pools/:pool_id/files
// ---------------------------------------------------------------------------

func (h *PoolHandler) AddFiles(c *gin.Context) {
	poolID, ok := parsePoolID(c)
	if !ok {
		return
	}

	var body struct {
		FileIDs  []string `json:"file_ids" binding:"required"`
		Position *int     `json:"position"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	fileIDs := make([]uuid.UUID, 0, len(body.FileIDs))
	for _, s := range body.FileIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			respondError(c, domain.ErrValidation)
			return
		}
		fileIDs = append(fileIDs, id)
	}

	if err := h.poolSvc.AddFiles(c.Request.Context(), poolID, fileIDs, body.Position); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusCreated)
}

// ---------------------------------------------------------------------------
// POST /pools/:pool_id/files/remove
// ---------------------------------------------------------------------------

func (h *PoolHandler) RemoveFiles(c *gin.Context) {
	poolID, ok := parsePoolID(c)
	if !ok {
		return
	}

	var body struct {
		FileIDs []string `json:"file_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	fileIDs := make([]uuid.UUID, 0, len(body.FileIDs))
	for _, s := range body.FileIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			respondError(c, domain.ErrValidation)
			return
		}
		fileIDs = append(fileIDs, id)
	}

	if err := h.poolSvc.RemoveFiles(c.Request.Context(), poolID, fileIDs); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// PUT /pools/:pool_id/files/reorder
// ---------------------------------------------------------------------------

func (h *PoolHandler) Reorder(c *gin.Context) {
	poolID, ok := parsePoolID(c)
	if !ok {
		return
	}

	var body struct {
		FileIDs []string `json:"file_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	fileIDs := make([]uuid.UUID, 0, len(body.FileIDs))
	for _, s := range body.FileIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			respondError(c, domain.ErrValidation)
			return
		}
		fileIDs = append(fileIDs, id)
	}

	if err := h.poolSvc.Reorder(c.Request.Context(), poolID, fileIDs); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}