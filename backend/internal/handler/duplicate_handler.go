package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/service"
)

// DuplicateHandler handles the /files/duplicates endpoints.
type DuplicateHandler struct {
	dupSvc *service.DuplicateService
}

// NewDuplicateHandler creates a DuplicateHandler.
func NewDuplicateHandler(dupSvc *service.DuplicateService) *DuplicateHandler {
	return &DuplicateHandler{dupSvc: dupSvc}
}

// List handles GET /files/duplicates — an offset-paginated list of duplicate
// clusters, each a group of files within the perceptual-hash threshold.
func (h *DuplicateHandler) List(c *gin.Context) {
	limit, offset := 20, 0
	if n, err := strconv.Atoi(c.Query("limit")); err == nil {
		limit = n
	}
	if n, err := strconv.Atoi(c.Query("offset")); err == nil {
		offset = n
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	clusters, total, err := h.dupSvc.Clusters(c.Request.Context(), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}

	items := make([]gin.H, len(clusters))
	for i, files := range clusters {
		fs := make([]fileJSON, len(files))
		for j, f := range files {
			fs[j] = toFileJSON(f)
		}
		items[i] = gin.H{"files": fs}
	}
	respondJSON(c, http.StatusOK, gin.H{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// Dismiss handles POST /files/duplicates/dismiss — mark a pair "not a duplicate".
func (h *DuplicateHandler) Dismiss(c *gin.Context) {
	var body struct {
		FileIDA string `json:"file_id_a" binding:"required"`
		FileIDB string `json:"file_id_b" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}
	ids, err := parseUUIDs([]string{body.FileIDA, body.FileIDB})
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}
	if err := h.dupSvc.Dismiss(c.Request.Context(), ids[0], ids[1]); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Resolve handles POST /files/duplicates/resolve — merge a duplicate pair,
// keeping one file and folding the chosen fields in from the other. Returns the
// updated survivor. delete_discarded defaults to true.
func (h *DuplicateHandler) Resolve(c *gin.Context) {
	var body struct {
		Keep            string              `json:"keep"    binding:"required"`
		Discard         string              `json:"discard" binding:"required"`
		Fields          service.MergeFields `json:"fields"`
		DeleteDiscarded *bool               `json:"delete_discarded"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}
	ids, err := parseUUIDs([]string{body.Keep, body.Discard})
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	del := true
	if body.DeleteDiscarded != nil {
		del = *body.DeleteDiscarded
	}
	f, err := h.dupSvc.Resolve(c.Request.Context(), service.MergeSpec{
		Keep:            ids[0],
		Discard:         ids[1],
		Fields:          body.Fields,
		DeleteDiscarded: del,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, toFileJSON(*f))
}
