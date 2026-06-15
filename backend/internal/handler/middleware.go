package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/service"
)

// AuthMiddleware validates Bearer JWTs and injects user identity into context.
type AuthMiddleware struct {
	authSvc *service.AuthService
}

// NewAuthMiddleware creates an AuthMiddleware backed by authSvc.
func NewAuthMiddleware(authSvc *service.AuthService) *AuthMiddleware {
	return &AuthMiddleware{authSvc: authSvc}
}

// Handle returns a Gin handler function that enforces authentication.
// On success it calls c.Next(); on failure it aborts with 401 JSON.
func (m *AuthMiddleware) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, errorBody{
				Code:    domain.ErrUnauthorized.Code(),
				Message: "authorization header missing or malformed",
			})
			c.Abort()
			return
		}

		claims, err := m.authSvc.ValidateAccessToken(c.Request.Context(), token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, errorBody{
				Code:    domain.ErrUnauthorized.Code(),
				Message: "invalid or expired token",
			})
			c.Abort()
			return
		}

		ctx := domain.WithUser(c.Request.Context(), claims.UserID, claims.IsAdmin, claims.SessionID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// HandleContent authenticates a file-content GET, accepting either a normal
// access token or a content token scoped (by its fid claim) to the :id in the
// path. The content token is what keeps a long media stream playing after the
// short access token would have expired. View permission is still enforced in
// the handler against the resolved user, so a content token only widens *when*
// a file may be read by URL, never *which* files.
func (m *AuthMiddleware) HandleContent() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c)
		if token == "" {
			contentUnauthorized(c)
			return
		}

		// A regular access token grants access to everything as usual.
		if claims, err := m.authSvc.ValidateAccessToken(c.Request.Context(), token); err == nil {
			ctx := domain.WithUser(c.Request.Context(), claims.UserID, claims.IsAdmin, claims.SessionID)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		// Otherwise accept a content token minted for exactly this file. Normalise
		// the path id to canonical form so it matches the minted fid claim.
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			contentUnauthorized(c)
			return
		}
		claims, err := m.authSvc.ValidateContentToken(token, id.String())
		if err != nil {
			contentUnauthorized(c)
			return
		}
		// A content token carries no session (sid 0); it is session-independent.
		ctx := domain.WithUser(c.Request.Context(), claims.UserID, claims.IsAdmin, claims.SessionID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func contentUnauthorized(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, errorBody{
		Code:    domain.ErrUnauthorized.Code(),
		Message: "invalid or expired token",
	})
	c.Abort()
}

// bearerToken extracts the access token from the Authorization header. As a
// fallback it accepts an ?access_token= query parameter, but only for GET
// requests — this lets the browser open media (e.g. /files/{id}/content) via a
// plain link/new tab, where it can't send the header, without allowing a crafted
// link to drive a state-changing request.
func bearerToken(c *gin.Context) string {
	if raw := c.GetHeader("Authorization"); strings.HasPrefix(raw, "Bearer ") {
		return strings.TrimPrefix(raw, "Bearer ")
	}
	if c.Request.Method == http.MethodGet {
		return c.Query("access_token")
	}
	return ""
}
