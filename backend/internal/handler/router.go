package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewRouter builds and returns a configured Gin engine.
func NewRouter(auth *AuthMiddleware, authHandler *AuthHandler, fileHandler *FileHandler) *gin.Engine {
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

	// File endpoints — all require authentication.
	files := v1.Group("/files", auth.Handle())
	{
		files.GET("", fileHandler.List)
		files.POST("", fileHandler.Upload)

		// Bulk routes must be registered before /:id to avoid ambiguity.
		files.POST("/bulk/tags", fileHandler.BulkSetTags)
		files.POST("/bulk/delete", fileHandler.BulkDelete)
		files.POST("/bulk/common-tags", fileHandler.CommonTags)
		files.POST("/import", fileHandler.Import)

		// Per-file routes.
		files.GET("/:id", fileHandler.GetMeta)
		files.PATCH("/:id", fileHandler.UpdateMeta)
		files.DELETE("/:id", fileHandler.SoftDelete)

		files.GET("/:id/content", fileHandler.GetContent)
		files.PUT("/:id/content", fileHandler.ReplaceContent)
		files.GET("/:id/thumbnail", fileHandler.GetThumbnail)
		files.GET("/:id/preview", fileHandler.GetPreview)
		files.POST("/:id/restore", fileHandler.Restore)
		files.DELETE("/:id/permanent", fileHandler.PermanentDelete)

		files.GET("/:id/tags", fileHandler.ListTags)
		files.PUT("/:id/tags", fileHandler.SetTags)
		files.PUT("/:id/tags/:tag_id", fileHandler.AddTag)
		files.DELETE("/:id/tags/:tag_id", fileHandler.RemoveTag)
	}

	return r
}