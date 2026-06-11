package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
	"tanabata/backend/internal/service"
)

// UserHandler handles all /users endpoints.
type UserHandler struct {
	userSvc *service.UserService
}

// NewUserHandler creates a UserHandler.
func NewUserHandler(userSvc *service.UserService) *UserHandler {
	return &UserHandler{userSvc: userSvc}
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

type userJSON struct {
	ID        int16  `json:"id"`
	Name      string `json:"name"`
	IsAdmin   bool   `json:"is_admin"`
	CanCreate bool   `json:"can_create"`
	IsBlocked bool   `json:"is_blocked"`
}

func toUserJSON(u domain.User) userJSON {
	return userJSON{
		ID:        u.ID,
		Name:      u.Name,
		IsAdmin:   u.IsAdmin,
		CanCreate: u.CanCreate,
		IsBlocked: u.IsBlocked,
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func requireAdmin(c *gin.Context) bool {
	_, isAdmin, _ := domain.UserFromContext(c.Request.Context())
	if !isAdmin {
		respondError(c, domain.ErrForbidden)
		return false
	}
	return true
}

func parseUserID(c *gin.Context) (int16, bool) {
	n, err := strconv.ParseInt(c.Param("user_id"), 10, 16)
	if err != nil {
		respondError(c, domain.ErrValidation)
		return 0, false
	}
	return int16(n), true
}

// ---------------------------------------------------------------------------
// GET /users/me
// ---------------------------------------------------------------------------

func (h *UserHandler) GetMe(c *gin.Context) {
	u, err := h.userSvc.GetMe(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, toUserJSON(*u))
}

// ---------------------------------------------------------------------------
// PATCH /users/me
// ---------------------------------------------------------------------------

func (h *UserHandler) UpdateMe(c *gin.Context) {
	var body struct {
		Name     string  `json:"name"`
		Password *string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	updated, err := h.userSvc.UpdateMe(c.Request.Context(), service.UpdateMeParams{
		Name:     body.Name,
		Password: body.Password,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, toUserJSON(*updated))
}

// ---------------------------------------------------------------------------
// GET /users  (admin)
// ---------------------------------------------------------------------------

func (h *UserHandler) List(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	params := port.OffsetParams{
		Sort:  c.DefaultQuery("sort", "id"),
		Order: c.DefaultQuery("order", "asc"),
	}
	if s := c.Query("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			params.Limit = n
		}
	}
	if s := c.Query("offset"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			params.Offset = n
		}
	}

	page, err := h.userSvc.List(c.Request.Context(), params)
	if err != nil {
		respondError(c, err)
		return
	}

	items := make([]userJSON, len(page.Items))
	for i, u := range page.Items {
		items[i] = toUserJSON(u)
	}
	respondJSON(c, http.StatusOK, gin.H{
		"items":  items,
		"total":  page.Total,
		"offset": page.Offset,
		"limit":  page.Limit,
	})
}

// ---------------------------------------------------------------------------
// POST /users  (admin)
// ---------------------------------------------------------------------------

func (h *UserHandler) Create(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var body struct {
		Name      string `json:"name"     binding:"required"`
		Password  string `json:"password" binding:"required"`
		IsAdmin   bool   `json:"is_admin"`
		CanCreate bool   `json:"can_create"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	created, err := h.userSvc.Create(c.Request.Context(), service.CreateUserParams{
		Name:      body.Name,
		Password:  body.Password,
		IsAdmin:   body.IsAdmin,
		CanCreate: body.CanCreate,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusCreated, toUserJSON(*created))
}

// ---------------------------------------------------------------------------
// GET /users/:user_id  (admin)
// ---------------------------------------------------------------------------

func (h *UserHandler) Get(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	u, err := h.userSvc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, toUserJSON(*u))
}

// ---------------------------------------------------------------------------
// PATCH /users/:user_id  (admin)
// ---------------------------------------------------------------------------

func (h *UserHandler) UpdateAdmin(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	var body struct {
		IsAdmin   *bool `json:"is_admin"`
		CanCreate *bool `json:"can_create"`
		IsBlocked *bool `json:"is_blocked"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	updated, err := h.userSvc.UpdateAdmin(c.Request.Context(), id, service.UpdateAdminParams{
		IsAdmin:   body.IsAdmin,
		CanCreate: body.CanCreate,
		IsBlocked: body.IsBlocked,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	respondJSON(c, http.StatusOK, toUserJSON(*updated))
}

// ---------------------------------------------------------------------------
// DELETE /users/:user_id  (admin)
// ---------------------------------------------------------------------------

func (h *UserHandler) Delete(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	if err := h.userSvc.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
