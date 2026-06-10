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
		raw := c.GetHeader("Authorization")
		if !strings.HasPrefix(raw, "Bearer ") {
			c.JSON(http.StatusUnauthorized, errorBody{
				Code:    domain.ErrUnauthorized.Code(),
				Message: "authorization header missing or malformed",
			})
			c.Abort()
			return
		}
		token := strings.TrimPrefix(raw, "Bearer ")

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
