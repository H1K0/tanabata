package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewRouter builds and returns a configured Gin engine.
func NewRouter(
	auth *AuthMiddleware,
	authHandler *AuthHandler,
	fileHandler *FileHandler,
	tagHandler *TagHandler,
	categoryHandler *CategoryHandler,
) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Health check — no auth required.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")

	// -------------------------------------------------------------------------
	// Auth
	// -------------------------------------------------------------------------
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

	// -------------------------------------------------------------------------
	// Files (all require auth)
	// -------------------------------------------------------------------------
	files := v1.Group("/files", auth.Handle())
	{
		files.GET("", fileHandler.List)
		files.POST("", fileHandler.Upload)

		// Bulk + import routes registered before /:id to prevent param collision.
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

		// File–tag relations — served by TagHandler for auto-rule support.
		files.GET("/:id/tags", tagHandler.FileListTags)
		files.PUT("/:id/tags", tagHandler.FileSetTags)
		files.PUT("/:id/tags/:tag_id", tagHandler.FileAddTag)
		files.DELETE("/:id/tags/:tag_id", tagHandler.FileRemoveTag)
	}

	// -------------------------------------------------------------------------
	// Tags (all require auth)
	// -------------------------------------------------------------------------
	tags := v1.Group("/tags", auth.Handle())
	{
		tags.GET("", tagHandler.List)
		tags.POST("", tagHandler.Create)

		tags.GET("/:tag_id", tagHandler.Get)
		tags.PATCH("/:tag_id", tagHandler.Update)
		tags.DELETE("/:tag_id", tagHandler.Delete)

		tags.GET("/:tag_id/files", tagHandler.ListFiles)

		tags.GET("/:tag_id/rules", tagHandler.ListRules)
		tags.POST("/:tag_id/rules", tagHandler.CreateRule)
		tags.DELETE("/:tag_id/rules/:then_tag_id", tagHandler.DeleteRule)
	}

	// -------------------------------------------------------------------------
	// Categories (all require auth)
	// -------------------------------------------------------------------------
	categories := v1.Group("/categories", auth.Handle())
	{
		categories.GET("", categoryHandler.List)
		categories.POST("", categoryHandler.Create)

		categories.GET("/:category_id", categoryHandler.Get)
		categories.PATCH("/:category_id", categoryHandler.Update)
		categories.DELETE("/:category_id", categoryHandler.Delete)

		categories.GET("/:category_id/tags", categoryHandler.ListTags)
	}

	return r
}