package config

import (
	"strings"
	"testing"
)

// setValidEnv sets every required variable to a valid dummy value, so a test can
// then override one var to exercise a single validation path.
func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("ADMIN_PASSWORD", "test-password")
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("FILES_PATH", "/tmp/files")
	t.Setenv("THUMBS_CACHE_PATH", "/tmp/thumbs")
	t.Setenv("IMPORT_PATH", "/tmp/import")
	// Pin the TTLs to valid values so an ambient env var can't perturb the case
	// under test; individual tests override the one they exercise.
	t.Setenv("JWT_ACCESS_TTL", "15m")
	t.Setenv("JWT_REFRESH_TTL", "720h")
	t.Setenv("CONTENT_TOKEN_TTL", "6h")
}

func TestLoadValid(t *testing.T) {
	setValidEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JWTAccessTTL <= 0 || cfg.JWTRefreshTTL <= 0 || cfg.ContentTokenTTL <= 0 {
		t.Fatalf("TTLs should be positive: access=%v refresh=%v content=%v",
			cfg.JWTAccessTTL, cfg.JWTRefreshTTL, cfg.ContentTokenTTL)
	}
}

func TestLoadRejectsNonPositiveTTL(t *testing.T) {
	cases := []struct{ key, val string }{
		{"JWT_ACCESS_TTL", "0"},
		{"JWT_REFRESH_TTL", "-1h"},
		{"CONTENT_TOKEN_TTL", "0s"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			setValidEnv(t)
			t.Setenv(tc.key, tc.val)

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error for %s=%q", tc.key, tc.val)
			}
			if !strings.Contains(err.Error(), tc.key) || !strings.Contains(err.Error(), "must be positive") {
				t.Fatalf("error should name %s and mention positivity, got: %v", tc.key, err)
			}
		})
	}
}
