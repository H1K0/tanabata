package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

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
