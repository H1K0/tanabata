package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/service"
)

// AuditHandler handles GET /audit.
type AuditHandler struct {
	auditSvc *service.AuditService
}

// NewAuditHandler creates an AuditHandler.
func NewAuditHandler(auditSvc *service.AuditService) *AuditHandler {
	return &AuditHandler{auditSvc: auditSvc}
}

// ---------------------------------------------------------------------------
// Response type
// ---------------------------------------------------------------------------

type auditEntryJSON struct {
	ID          int64   `json:"id"`
	UserID      int16   `json:"user_id"`
	UserName    string  `json:"user_name"`
	Action      string  `json:"action"`
	ObjectType  *string `json:"object_type"`
	ObjectID    *string `json:"object_id"`
	PerformedAt string  `json:"performed_at"`
}

func toAuditEntryJSON(e domain.AuditEntry) auditEntryJSON {
	j := auditEntryJSON{
		ID:          e.ID,
		UserID:      e.UserID,
		UserName:    e.UserName,
		Action:      e.Action,
		ObjectType:  e.ObjectType,
		PerformedAt: e.PerformedAt.UTC().Format(time.RFC3339),
	}
	if e.ObjectID != nil {
		s := e.ObjectID.String()
		j.ObjectID = &s
	}
	return j
}

// ---------------------------------------------------------------------------
// GET /audit  (admin)
// ---------------------------------------------------------------------------

func (h *AuditHandler) List(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	filter := domain.AuditFilter{}

	if s := c.Query("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			filter.Limit = n
		}
	}
	if s := c.Query("offset"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			filter.Offset = n
		}
	}
	if s := c.Query("user_id"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 16); err == nil {
			id := int16(n)
			filter.UserID = &id
		}
	}
	if s := c.Query("action"); s != "" {
		filter.Action = s
	}
	if s := c.Query("object_type"); s != "" {
		filter.ObjectType = s
	}
	if s := c.Query("object_id"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			filter.ObjectID = &id
		}
	}
	if s := c.Query("from"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			filter.From = &t
		}
	}
	if s := c.Query("to"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			filter.To = &t
		}
	}

	page, err := h.auditSvc.Query(c.Request.Context(), filter)
	if err != nil {
		respondError(c, err)
		return
	}

	items := make([]auditEntryJSON, len(page.Items))
	for i, e := range page.Items {
		items[i] = toAuditEntryJSON(e)
	}
	respondJSON(c, http.StatusOK, gin.H{
		"items":  items,
		"total":  page.Total,
		"offset": page.Offset,
		"limit":  page.Limit,
	})
}