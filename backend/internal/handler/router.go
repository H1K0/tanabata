package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewRouter builds and returns a configured Gin engine.
// Additional handlers will be added here as they are implemented.
func NewRouter(auth *AuthMiddleware, authHandler *AuthHandler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Health check — no auth required.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")

	// Auth endpoints — login and refresh are public; others require a valid token.
	authGroup := v1.Group("/auth")
	{
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/refresh", authHandler.Refresh)

		protected := authGroup.Group("", auth.Handle())
		{
			protected.POST("/logout", authHandler.Logout)
			protected.GET("/sessions", authHandler.ListSessions)
			protected.DELETE("/sessions/:id", authHandler.TerminateSession)
		}
	}

	return r
}
