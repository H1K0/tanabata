package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	ListenAddr    string
	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration

	// Initial admin bootstrap (applied on startup if the user does not exist)
	AdminUsername string
	AdminPassword string

	// Database
	DatabaseURL string

	// Storage
	FilesPath       string
	ThumbsCachePath string
	MaxUploadBytes  int64 // reject uploads larger than this (bytes)

	// Thumbnails
	ThumbWidth    int
	ThumbHeight   int
	PreviewWidth  int
	PreviewHeight int
	// ThumbMaxPixels caps the pixel count of a source image decoded in-process by
	// the pure-Go fallback (a decompression-bomb guard and memory bound); larger
	// images then get a placeholder. It does not apply when vipsthumbnail is
	// installed, which shrinks on load regardless of source size.
	ThumbMaxPixels int
	// ThumbConcurrency bounds how many thumbnails/previews are generated at once,
	// so a burst of large images can't saturate every core or exhaust RAM. 0 =
	// auto (half the available CPUs).
	ThumbConcurrency int

	// Import
	ImportPath string

	// Static SPA. When set, the server serves the built frontend (and falls
	// back to index.html for client routes) on the same port as the API. Empty
	// in local development, where the Vite dev server serves the UI separately.
	StaticDir string
}

// Load reads a .env file (if present) then loads all configuration from
// environment variables. Returns an error listing every missing or invalid var.
func Load() (*Config, error) {
	// Non-fatal: .env may not exist in production.
	_ = godotenv.Load()

	var errs []error

	requireStr := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", key))
		}
		return v
	}

	defaultStr := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}

	parseDuration := func(key, def string) time.Duration {
		raw := defaultStr(key, def)
		d, err := time.ParseDuration(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: invalid duration %q: %w", key, raw, err))
			return 0
		}
		return d
	}

	parseInt := func(key string, def int) int {
		raw := os.Getenv(key)
		if raw == "" {
			return def
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: invalid integer %q: %w", key, raw, err))
			return def
		}
		return n
	}

	parseInt64 := func(key string, def int64) int64 {
		raw := os.Getenv(key)
		if raw == "" {
			return def
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: invalid integer %q: %w", key, raw, err))
			return def
		}
		return n
	}

	cfg := &Config{
		ListenAddr:    defaultStr("LISTEN_ADDR", ":42776"),
		JWTSecret:     requireStr("JWT_SECRET"),
		JWTAccessTTL:  parseDuration("JWT_ACCESS_TTL", "15m"),
		JWTRefreshTTL: parseDuration("JWT_REFRESH_TTL", "720h"),

		AdminUsername: defaultStr("ADMIN_USERNAME", "admin"),
		AdminPassword: requireStr("ADMIN_PASSWORD"),

		DatabaseURL: requireStr("DATABASE_URL"),

		FilesPath:       requireStr("FILES_PATH"),
		ThumbsCachePath: requireStr("THUMBS_CACHE_PATH"),
		MaxUploadBytes:  parseInt64("MAX_UPLOAD_BYTES", 500<<20), // 500 MiB

		ThumbWidth:       parseInt("THUMB_WIDTH", 160),
		ThumbHeight:      parseInt("THUMB_HEIGHT", 160),
		PreviewWidth:     parseInt("PREVIEW_WIDTH", 1920),
		PreviewHeight:    parseInt("PREVIEW_HEIGHT", 1080),
		ThumbMaxPixels:   parseInt("THUMB_MAX_PIXELS", 300_000_000), // ~300 Mpx (e.g. 13000×17000)
		ThumbConcurrency: parseInt("THUMB_CONCURRENCY", 0),          // 0 = auto

		ImportPath: requireStr("IMPORT_PATH"),

		StaticDir: defaultStr("STATIC_DIR", ""),
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return cfg, nil
}
