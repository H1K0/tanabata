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

	// Thumbnails
	ThumbWidth    int
	ThumbHeight   int
	PreviewWidth  int
	PreviewHeight int

	// Import
	ImportPath string
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

	cfg := &Config{
		ListenAddr:    defaultStr("LISTEN_ADDR", ":8080"),
		JWTSecret:     requireStr("JWT_SECRET"),
		JWTAccessTTL:  parseDuration("JWT_ACCESS_TTL", "15m"),
		JWTRefreshTTL: parseDuration("JWT_REFRESH_TTL", "720h"),

		AdminUsername: defaultStr("ADMIN_USERNAME", "admin"),
		AdminPassword: requireStr("ADMIN_PASSWORD"),

		DatabaseURL: requireStr("DATABASE_URL"),

		FilesPath:       requireStr("FILES_PATH"),
		ThumbsCachePath: requireStr("THUMBS_CACHE_PATH"),

		ThumbWidth:    parseInt("THUMB_WIDTH", 160),
		ThumbHeight:   parseInt("THUMB_HEIGHT", 160),
		PreviewWidth:  parseInt("PREVIEW_WIDTH", 1920),
		PreviewHeight: parseInt("PREVIEW_HEIGHT", 1080),

		ImportPath: requireStr("IMPORT_PATH"),
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return cfg, nil
}
