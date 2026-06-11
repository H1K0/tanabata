package handler

import (
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func init() {
	// Go's mime table doesn't know .webmanifest; register it so the PWA manifest
	// is served as JSON and isn't rejected by the X-Content-Type-Options header.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

// spaHandler serves the built single-page app from dir. It is wired as the
// router's NoRoute handler, so it only sees requests that matched no API route.
//
// A request whose path maps to a real file on disk is served directly (with
// cache headers tuned to SvelteKit's adapter-static output). Anything else
// falls back to index.html so the client-side router can resolve deep links
// like /pools/123. Unknown /api/ paths return a JSON 404 instead of the HTML
// shell, keeping API error responses machine-readable.
func spaHandler(dir string) gin.HandlerFunc {
	indexPath := filepath.Join(dir, "index.html")

	return func(c *gin.Context) {
		reqPath := c.Request.URL.Path

		if strings.HasPrefix(reqPath, "/api/") {
			c.JSON(http.StatusNotFound, errorBody{
				Code:    "not_found",
				Message: "resource not found",
			})
			return
		}

		// Resolve the request to a path inside dir. Cleaning an absolute path
		// collapses any "../" segments before the join, so the result can never
		// escape dir — this is the traversal guard.
		clean := path.Clean("/" + reqPath)
		target := filepath.Join(dir, filepath.FromSlash(clean))

		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			c.Header("Cache-Control", cacheControl(clean))
			c.File(target)
			return
		}

		// SPA fallback: serve the shell, never cached so a new deploy is picked
		// up immediately on the next navigation.
		c.Header("Cache-Control", "no-cache")
		c.File(indexPath)
	}
}

// cacheControl returns the Cache-Control value for a served static asset.
// SvelteKit emits content-hashed files under /_app/immutable — those are safe
// to cache forever. The service worker must never be cached, or clients pin to
// a stale shell. Everything else gets a short, revalidated TTL.
func cacheControl(p string) string {
	switch {
	case strings.HasPrefix(p, "/_app/immutable/"):
		return "public, max-age=31536000, immutable"
	case p == "/service-worker.js":
		return "no-cache"
	default:
		return "public, max-age=3600"
	}
}
