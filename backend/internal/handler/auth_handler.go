package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/service"
)

// AuthHandler handles all /auth endpoints.
type AuthHandler struct {
	authSvc *service.AuthService
}

// NewAuthHandler creates an AuthHandler backed by authSvc.
func NewAuthHandler(authSvc *service.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

// Login handles POST /auth/login.
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Name     string `json:"name"     binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	pair, err := h.authSvc.Login(c.Request.Context(), req.Name, req.Password, c.GetHeader("User-Agent"))
	if err != nil {
		respondError(c, err)
		return
	}

	respondJSON(c, http.StatusOK, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    pair.ExpiresIn,
	})
}

// Refresh handles POST /auth/refresh.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	pair, err := h.authSvc.Refresh(c.Request.Context(), req.RefreshToken, c.GetHeader("User-Agent"))
	if err != nil {
		respondError(c, err)
		return
	}

	respondJSON(c, http.StatusOK, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    pair.ExpiresIn,
	})
}

// Logout handles POST /auth/logout. Requires authentication.
func (h *AuthHandler) Logout(c *gin.Context) {
	_, _, sessionID := domain.UserFromContext(c.Request.Context())

	if err := h.authSvc.Logout(c.Request.Context(), sessionID); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ListSessions handles GET /auth/sessions. Requires authentication.
func (h *AuthHandler) ListSessions(c *gin.Context) {
	userID, _, sessionID := domain.UserFromContext(c.Request.Context())

	list, err := h.authSvc.ListSessions(c.Request.Context(), userID, sessionID)
	if err != nil {
		respondError(c, err)
		return
	}

	type sessionItem struct {
		ID           int    `json:"id"`
		UserAgent    string `json:"user_agent"`
		StartedAt    string `json:"started_at"`
		ExpiresAt    any    `json:"expires_at"`
		LastActivity string `json:"last_activity"`
		IsCurrent    bool   `json:"is_current"`
	}

	items := make([]sessionItem, len(list.Items))
	for i, s := range list.Items {
		var expiresAt any
		if s.ExpiresAt != nil {
			expiresAt = s.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
		}
		items[i] = sessionItem{
			ID:           s.ID,
			UserAgent:    s.UserAgent,
			StartedAt:    s.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
			ExpiresAt:    expiresAt,
			LastActivity: s.LastActivity.Format("2006-01-02T15:04:05Z07:00"),
			IsCurrent:    s.IsCurrent,
		}
	}

	respondJSON(c, http.StatusOK, gin.H{
		"items": items,
		"total": list.Total,
	})
}

// TerminateSession handles DELETE /auth/sessions/:id. Requires authentication.
func (h *AuthHandler) TerminateSession(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	userID, isAdmin, _ := domain.UserFromContext(c.Request.Context())

	if err := h.authSvc.TerminateSession(c.Request.Context(), userID, isAdmin, id); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
