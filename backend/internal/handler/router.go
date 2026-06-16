package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// securityHeaders sets conservative response headers on every response: prevent
// MIME sniffing of served file content, forbid framing, and suppress the
// Referer header on outbound navigations.
func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		c.Next()
	}
}

// NewRouter builds and returns a configured Gin engine.
func NewRouter(
	auth *AuthMiddleware,
	authHandler *AuthHandler,
	fileHandler *FileHandler,
	duplicateHandler *DuplicateHandler,
	tagHandler *TagHandler,
	categoryHandler *CategoryHandler,
	poolHandler *PoolHandler,
	userHandler *UserHandler,
	aclHandler *ACLHandler,
	auditHandler *AuditHandler,
	staticDir string,
	trustedProxies []string,
) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), securityHeaders())

	// Behind a reverse proxy the client's real IP arrives in X-Forwarded-For.
	// Trust only the proxy hop(s) so c.ClientIP() — used by the auth rate
	// limiter — reflects the real client and can't be spoofed by a forged
	// header from a direct caller. An empty list trusts no proxy (ClientIP is
	// the immediate peer).
	if err := r.SetTrustedProxies(trustedProxies); err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
	}

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
		// Throttle credential endpoints per client IP to slow brute force.
		authLimiter := newRateLimiter(10, time.Minute).Middleware()
		authGroup.POST("/login", authLimiter, authHandler.Login)
		authGroup.POST("/refresh", authLimiter, authHandler.Refresh)

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

		// Bulk + import + duplicates routes registered before /:id to prevent
		// param collision (e.g. "duplicates" being captured as :id).
		files.GET("/duplicates", duplicateHandler.List)
		files.POST("/duplicates/dismiss", duplicateHandler.Dismiss)
		files.POST("/duplicates/resolve", duplicateHandler.Resolve)
		files.POST("/bulk/tags", fileHandler.BulkSetTags)
		files.POST("/bulk/delete", fileHandler.BulkDelete)
		files.POST("/bulk/review", fileHandler.BulkReview)
		files.POST("/bulk/common-tags", fileHandler.CommonTags)
		files.POST("/import", fileHandler.Import)

		// Per-file routes.
		files.GET("/:id", fileHandler.GetMeta)
		files.PATCH("/:id", fileHandler.UpdateMeta)
		files.DELETE("/:id", fileHandler.SoftDelete)

		files.PUT("/:id/content", fileHandler.ReplaceContent)
		// Mints a content token (strict auth) for the GET /:id/content route below.
		files.POST("/:id/content-token", fileHandler.CreateContentToken)
		files.GET("/:id/thumbnail", fileHandler.GetThumbnail)
		files.GET("/:id/preview", fileHandler.GetPreview)
		files.POST("/:id/views", fileHandler.RecordView)
		files.POST("/:id/restore", fileHandler.Restore)
		files.DELETE("/:id/permanent", fileHandler.PermanentDelete)

		// File–tag relations — served by TagHandler for auto-rule support.
		files.GET("/:id/tags", tagHandler.FileListTags)
		files.PUT("/:id/tags", tagHandler.FileSetTags)
		files.PUT("/:id/tags/:tag_id", tagHandler.FileAddTag)
		files.DELETE("/:id/tags/:tag_id", tagHandler.FileRemoveTag)
	}

	// Serving an original is the one read that can outlive a 15-minute access
	// token — a long video streams via repeated Range requests over many minutes.
	// So this route alone also accepts a file-scoped content token (see
	// HandleContent), letting the media URL stay valid for the whole playback.
	media := v1.Group("/files", auth.HandleContent())
	{
		media.GET("/:id/content", fileHandler.GetContent)
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
		tags.PATCH("/:tag_id/rules/:then_tag_id", tagHandler.PatchRule)
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

	// -------------------------------------------------------------------------
	// Pools (all require auth)
	// -------------------------------------------------------------------------
	pools := v1.Group("/pools", auth.Handle())
	{
		pools.GET("", poolHandler.List)
		pools.POST("", poolHandler.Create)

		pools.GET("/:pool_id", poolHandler.Get)
		pools.PATCH("/:pool_id", poolHandler.Update)
		pools.DELETE("/:pool_id", poolHandler.Delete)
		pools.POST("/:pool_id/views", poolHandler.RecordView)

		// Sub-routes registered before /:pool_id/files to avoid param conflicts.
		pools.POST("/:pool_id/files/remove", poolHandler.RemoveFiles)
		pools.PUT("/:pool_id/files/reorder", poolHandler.Reorder)

		pools.GET("/:pool_id/files", poolHandler.ListFiles)
		pools.POST("/:pool_id/files", poolHandler.AddFiles)
	}

	// -------------------------------------------------------------------------
	// Users (auth required; admin checks enforced in handler)
	// -------------------------------------------------------------------------
	users := v1.Group("/users", auth.Handle())
	{
		// /users/me must be registered before /:user_id to avoid param capture.
		users.GET("/me", userHandler.GetMe)
		users.PATCH("/me", userHandler.UpdateMe)

		users.GET("", userHandler.List)
		users.POST("", userHandler.Create)

		users.GET("/:user_id", userHandler.Get)
		users.PATCH("/:user_id", userHandler.UpdateAdmin)
		users.DELETE("/:user_id", userHandler.Delete)
	}

	// -------------------------------------------------------------------------
	// ACL (auth required)
	// -------------------------------------------------------------------------
	acl := v1.Group("/acl", auth.Handle())
	{
		acl.GET("/:object_type/:object_id", aclHandler.GetPermissions)
		acl.PUT("/:object_type/:object_id", aclHandler.SetPermissions)
	}

	// -------------------------------------------------------------------------
	// Audit (auth required; admin check enforced in handler)
	// -------------------------------------------------------------------------
	v1.GET("/audit", auth.Handle(), auditHandler.List)

	// Serve the built single-page app on the same port as the API. When
	// staticDir is empty (local development) the Vite dev server serves the UI
	// instead, so the API runs standalone and unknown routes 404 normally.
	if staticDir != "" {
		r.NoRoute(spaHandler(staticDir))
	}

	return r, nil
}
