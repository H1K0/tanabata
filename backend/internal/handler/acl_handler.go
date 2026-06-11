package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/service"
)

// objectTypeIDs maps the URL segment to the object_type PK in core.object_types.
// Row order matches 007_seed_data.sql: file=1, tag=2, category=3, pool=4.
var objectTypeIDs = map[string]int16{
	"file":     1,
	"tag":      2,
	"category": 3,
	"pool":     4,
}

// ACLHandler handles GET/PUT /acl/:object_type/:object_id.
type ACLHandler struct {
	aclSvc *service.ACLService
}

// NewACLHandler creates an ACLHandler.
func NewACLHandler(aclSvc *service.ACLService) *ACLHandler {
	return &ACLHandler{aclSvc: aclSvc}
}

// ---------------------------------------------------------------------------
// Response type
// ---------------------------------------------------------------------------

type permissionJSON struct {
	UserID   int16  `json:"user_id"`
	UserName string `json:"user_name"`
	CanView  bool   `json:"can_view"`
	CanEdit  bool   `json:"can_edit"`
}

func toPermissionJSON(p domain.Permission) permissionJSON {
	return permissionJSON{
		UserID:   p.UserID,
		UserName: p.UserName,
		CanView:  p.CanView,
		CanEdit:  p.CanEdit,
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseACLPath(c *gin.Context) (objectTypeID int16, objectID uuid.UUID, ok bool) {
	typeStr := c.Param("object_type")
	id, exists := objectTypeIDs[typeStr]
	if !exists {
		respondError(c, domain.ErrValidation)
		return 0, uuid.UUID{}, false
	}

	objectID, err := uuid.Parse(c.Param("object_id"))
	if err != nil {
		respondError(c, domain.ErrValidation)
		return 0, uuid.UUID{}, false
	}

	return id, objectID, true
}

// ---------------------------------------------------------------------------
// GET /acl/:object_type/:object_id
// ---------------------------------------------------------------------------

func (h *ACLHandler) GetPermissions(c *gin.Context) {
	objectTypeID, objectID, ok := parseACLPath(c)
	if !ok {
		return
	}

	userID, isAdmin, _ := domain.UserFromContext(c.Request.Context())
	perms, err := h.aclSvc.GetPermissions(c.Request.Context(), userID, isAdmin, objectTypeID, objectID)
	if err != nil {
		respondError(c, err)
		return
	}

	out := make([]permissionJSON, len(perms))
	for i, p := range perms {
		out[i] = toPermissionJSON(p)
	}
	respondJSON(c, http.StatusOK, out)
}

// ---------------------------------------------------------------------------
// PUT /acl/:object_type/:object_id
// ---------------------------------------------------------------------------

func (h *ACLHandler) SetPermissions(c *gin.Context) {
	objectTypeID, objectID, ok := parseACLPath(c)
	if !ok {
		return
	}

	var body struct {
		Permissions []struct {
			UserID  int16 `json:"user_id"  binding:"required"`
			CanView bool  `json:"can_view"`
			CanEdit bool  `json:"can_edit"`
		} `json:"permissions" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	perms := make([]domain.Permission, len(body.Permissions))
	for i, p := range body.Permissions {
		perms[i] = domain.Permission{
			UserID:  p.UserID,
			CanView: p.CanView,
			CanEdit: p.CanEdit,
		}
	}

	userID, isAdmin, _ := domain.UserFromContext(c.Request.Context())
	if err := h.aclSvc.SetPermissions(c.Request.Context(), userID, isAdmin, objectTypeID, objectID, perms); err != nil {
		respondError(c, err)
		return
	}

	// Re-read to return the stored permissions (with UserName denormalized).
	stored, err := h.aclSvc.GetPermissions(c.Request.Context(), userID, isAdmin, objectTypeID, objectID)
	if err != nil {
		respondError(c, err)
		return
	}

	out := make([]permissionJSON, len(stored))
	for i, p := range stored {
		out[i] = toPermissionJSON(p)
	}
	respondJSON(c, http.StatusOK, out)
}
