// Package integration contains end-to-end tests that start a real HTTP server
// against a disposable PostgreSQL database created on the fly.
//
// The test connects to an admin DSN (defaults to the local PG 16 socket) to
// CREATE / DROP an ephemeral database per test suite run, then runs all goose
// migrations on it.
//
// Override the admin DSN with TANABATA_TEST_ADMIN_DSN:
//
//	export TANABATA_TEST_ADMIN_DSN="host=/var/run/postgresql port=5434 user=h1k0 dbname=postgres sslmode=disable"
//	go test -v -timeout 120s tanabata/backend/internal/integration
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tanabata/backend/internal/db/postgres"
	"tanabata/backend/internal/handler"
	"tanabata/backend/internal/service"
	"tanabata/backend/internal/storage"
	"tanabata/backend/migrations"
)

// defaultAdminDSN is the fallback when TANABATA_TEST_ADMIN_DSN is unset.
// Targets the PG 16 cluster on this machine (port 5434, Unix socket).
const defaultAdminDSN = "host=/var/run/postgresql port=5434 user=h1k0 dbname=postgres sslmode=disable"

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

type harness struct {
	t         *testing.T
	server    *httptest.Server
	client    *http.Client
	importDir string
	pool      *pgxpool.Pool
}

// setupSuite creates an ephemeral database, runs migrations, wires the full
// service graph into an httptest.Server, and registers cleanup.
func setupSuite(t *testing.T) *harness {
	t.Helper()
	ctx := context.Background()

	// --- Create an isolated test database ------------------------------------
	adminDSN := os.Getenv("TANABATA_TEST_ADMIN_DSN")
	if adminDSN == "" {
		adminDSN = defaultAdminDSN
	}

	// Use a unique name so parallel test runs don't collide.
	dbName := fmt.Sprintf("tanabata_test_%d", time.Now().UnixNano())

	adminConn, err := pgx.Connect(ctx, adminDSN)
	require.NoError(t, err, "connect to admin DSN: %s", adminDSN)

	_, err = adminConn.Exec(ctx, "CREATE DATABASE "+dbName)
	require.NoError(t, err)
	adminConn.Close(ctx)

	// Build the DSN for the new database (replace dbname= in adminDSN).
	testDSN := replaceDSNDatabase(adminDSN, dbName)

	t.Cleanup(func() {
		// Drop all connections then drop the database.
		conn, err := pgx.Connect(context.Background(), adminDSN)
		if err != nil {
			return
		}
		defer conn.Close(context.Background())
		_, _ = conn.Exec(context.Background(),
			"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1", dbName)
		_, _ = conn.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbName)
	})

	// --- Migrations ----------------------------------------------------------
	pool, err := pgxpool.New(ctx, testDSN)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	migDB := stdlib.OpenDBFromPool(pool)
	goose.SetBaseFS(migrations.FS)
	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.Up(migDB, "."))
	migDB.Close()

	// --- Temp directories for storage ----------------------------------------
	filesDir := t.TempDir()
	thumbsDir := t.TempDir()
	importDir := t.TempDir()

	diskStorage, err := storage.NewDiskStorage(filesDir, thumbsDir, 160, 160, 1920, 1080, 0, 0)
	require.NoError(t, err)

	// --- Repositories --------------------------------------------------------
	userRepo := postgres.NewUserRepo(pool)
	sessionRepo := postgres.NewSessionRepo(pool)
	fileRepo := postgres.NewFileRepo(pool)
	mimeRepo := postgres.NewMimeRepo(pool)
	aclRepo := postgres.NewACLRepo(pool)
	auditRepo := postgres.NewAuditRepo(pool)
	tagRepo := postgres.NewTagRepo(pool)
	tagRuleRepo := postgres.NewTagRuleRepo(pool)
	categoryRepo := postgres.NewCategoryRepo(pool)
	poolRepo := postgres.NewPoolRepo(pool)
	transactor := postgres.NewTransactor(pool)

	// --- Services ------------------------------------------------------------
	authSvc := service.NewAuthService(userRepo, sessionRepo, "test-secret", 15*time.Minute, 720*time.Hour)
	aclSvc := service.NewACLService(aclRepo, fileRepo, tagRepo, categoryRepo, poolRepo, transactor)
	auditSvc := service.NewAuditService(auditRepo)
	tagSvc := service.NewTagService(tagRepo, tagRuleRepo, aclSvc, auditSvc, transactor)
	categorySvc := service.NewCategoryService(categoryRepo, tagRepo, aclSvc, auditSvc)
	poolSvc := service.NewPoolService(poolRepo, aclSvc, auditSvc)
	fileSvc := service.NewFileService(fileRepo, mimeRepo, diskStorage, aclSvc, auditSvc, tagSvc, transactor, importDir)
	userSvc := service.NewUserService(userRepo, sessionRepo, auditSvc)

	// Bootstrap the admin account the suite logs in with (replaces the old
	// hardcoded seed credentials).
	require.NoError(t, userSvc.EnsureAdmin(ctx, "admin", "admin"))

	// --- Handlers ------------------------------------------------------------
	authMiddleware := handler.NewAuthMiddleware(authSvc)
	authHandler := handler.NewAuthHandler(authSvc)
	fileHandler := handler.NewFileHandler(fileSvc, tagSvc, 500<<20)
	tagHandler := handler.NewTagHandler(tagSvc, fileSvc)
	categoryHandler := handler.NewCategoryHandler(categorySvc)
	poolHandler := handler.NewPoolHandler(poolSvc)
	userHandler := handler.NewUserHandler(userSvc)
	aclHandler := handler.NewACLHandler(aclSvc)
	auditHandler := handler.NewAuditHandler(auditSvc)

	r, err := handler.NewRouter(
		authMiddleware, authHandler,
		fileHandler, tagHandler, categoryHandler, poolHandler,
		userHandler, aclHandler, auditHandler,
		"",
		nil,
	)
	require.NoError(t, err)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	return &harness{
		t:         t,
		server:    srv,
		client:    srv.Client(),
		importDir: importDir,
		pool:      pool,
	}
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

// testResponse wraps an HTTP response with the body already read into memory.
// This avoids the "body consumed by error-message arg before decode" pitfall.
type testResponse struct {
	StatusCode int
	bodyBytes  []byte
}

// String returns the body as a string (for use in assertion messages).
func (r *testResponse) String() string { return string(r.bodyBytes) }

// decode unmarshals the body JSON into dst.
func (r *testResponse) decode(t *testing.T, dst any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(r.bodyBytes, dst), "decode body: %s", r.String())
}

func (h *harness) url(path string) string {
	return h.server.URL + "/api/v1" + path
}

// tagUses returns all activity.tag_uses rows as tag_id (text) → is_included.
func (h *harness) tagUses(ctx context.Context) map[string]bool {
	h.t.Helper()
	rows, err := h.pool.Query(ctx, `SELECT tag_id::text, is_included FROM activity.tag_uses`)
	require.NoError(h.t, err)
	defer rows.Close()

	out := make(map[string]bool)
	for rows.Next() {
		var id string
		var included bool
		require.NoError(h.t, rows.Scan(&id, &included))
		out[id] = included
	}
	require.NoError(h.t, rows.Err())
	return out
}

// countTagUses returns the number of rows in activity.tag_uses.
func (h *harness) countTagUses(ctx context.Context) int {
	h.t.Helper()
	var n int
	require.NoError(h.t, h.pool.QueryRow(ctx, `SELECT count(*) FROM activity.tag_uses`).Scan(&n))
	return n
}

func (h *harness) do(method, path string, body io.Reader, token string, contentType string) *testResponse {
	h.t.Helper()
	req, err := http.NewRequest(method, h.url(path), body)
	require.NoError(h.t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	httpResp, err := h.client.Do(req)
	require.NoError(h.t, err)
	b, _ := io.ReadAll(httpResp.Body)
	httpResp.Body.Close()
	return &testResponse{StatusCode: httpResp.StatusCode, bodyBytes: b}
}

func (h *harness) doJSON(method, path string, payload any, token string) *testResponse {
	h.t.Helper()
	var buf io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		require.NoError(h.t, err)
		buf = bytes.NewReader(b)
	}
	return h.do(method, path, buf, token, "application/json")
}

// login posts credentials and returns an access token.
func (h *harness) login(name, password string) string {
	h.t.Helper()
	resp := h.doJSON("POST", "/auth/login", map[string]string{
		"name": name, "password": password,
	}, "")
	require.Equal(h.t, http.StatusOK, resp.StatusCode, "login failed: %s", resp)
	var out struct {
		AccessToken string `json:"access_token"`
	}
	resp.decode(h.t, &out)
	require.NotEmpty(h.t, out.AccessToken)
	return out.AccessToken
}

// uploadJPEG uploads a minimal valid JPEG and returns the created file object.
func (h *harness) uploadJPEG(token, originalName string) map[string]any {
	h.t.Helper()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", originalName)
	require.NoError(h.t, err)
	_, err = fw.Write(minimalJPEG())
	require.NoError(h.t, err)
	require.NoError(h.t, mw.Close())

	resp := h.do("POST", "/files", &buf, token, mw.FormDataContentType())
	require.Equal(h.t, http.StatusCreated, resp.StatusCode, "upload failed: %s", resp)

	var out map[string]any
	resp.decode(h.t, &out)
	return out
}

// ---------------------------------------------------------------------------
// Main integration test
// ---------------------------------------------------------------------------

func TestFullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	h := setupSuite(t)

	// =========================================================================
	// 1. Admin login (seeded by 007_seed_data.sql)
	// =========================================================================
	adminToken := h.login("admin", "admin")

	// =========================================================================
	// 2. Create a regular user
	// =========================================================================
	resp := h.doJSON("POST", "/users", map[string]any{
		"name": "alice", "password": "alicepass", "can_create": true,
	}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
	var aliceUser map[string]any
	resp.decode(t, &aliceUser)
	assert.Equal(t, "alice", aliceUser["name"])

	// Create a second regular user for ACL testing.
	resp = h.doJSON("POST", "/users", map[string]any{
		"name": "bob", "password": "bobpass", "can_create": true,
	}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())

	// =========================================================================
	// 3. Log in as alice
	// =========================================================================
	aliceToken := h.login("alice", "alicepass")
	bobToken := h.login("bob", "bobpass")

	// =========================================================================
	// 4. Alice uploads a private JPEG
	// =========================================================================
	fileObj := h.uploadJPEG(aliceToken, "sunset.jpg")
	fileID, ok := fileObj["id"].(string)
	require.True(t, ok, "file id missing")
	assert.Equal(t, "sunset.jpg", fileObj["original_name"])
	assert.Equal(t, false, fileObj["is_public"])

	// =========================================================================
	// 5. Create a tag and assign it to the file
	// =========================================================================
	resp = h.doJSON("POST", "/tags", map[string]any{
		"name": "nature", "is_public": true,
	}, aliceToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
	var tagObj map[string]any
	resp.decode(t, &tagObj)
	tagID := tagObj["id"].(string)

	resp = h.doJSON("PUT", "/files/"+fileID+"/tags", map[string]any{
		"tag_ids": []string{tagID},
	}, aliceToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())

	// Verify tag is returned with the file.
	resp = h.doJSON("GET", "/files/"+fileID, nil, aliceToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var fileWithTags map[string]any
	resp.decode(t, &fileWithTags)
	tags := fileWithTags["tags"].([]any)
	require.Len(t, tags, 1)
	assert.Equal(t, "nature", tags[0].(map[string]any)["name"])

	// =========================================================================
	// 6. Filter files by tag
	// =========================================================================
	resp = h.doJSON("GET", "/files?filter=%7Bt%3D"+tagID+"%7D", nil, aliceToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var filePage map[string]any
	resp.decode(t, &filePage)
	items := filePage["items"].([]any)
	require.Len(t, items, 1)
	assert.Equal(t, fileID, items[0].(map[string]any)["id"])

	// =========================================================================
	// 7. ACL — Bob cannot see Alice's private file
	// =========================================================================
	resp = h.doJSON("GET", "/files/"+fileID, nil, bobToken)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, resp.String())

	// Grant Bob view access.
	bobUserID := int(aliceUser["id"].(float64)) // alice's id used for reference; get bob's
	// Resolve bob's real ID via admin.
	resp = h.doJSON("GET", "/users", nil, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var usersPage map[string]any
	resp.decode(t, &usersPage)
	var bobID float64
	for _, u := range usersPage["items"].([]any) {
		um := u.(map[string]any)
		if um["name"] == "bob" {
			bobID = um["id"].(float64)
		}
	}
	_ = bobUserID
	require.NotZero(t, bobID)

	resp = h.doJSON("PUT", "/acl/file/"+fileID, map[string]any{
		"permissions": []map[string]any{
			{"user_id": bobID, "can_view": true, "can_edit": false},
		},
	}, aliceToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())

	// Now Bob can view.
	resp = h.doJSON("GET", "/files/"+fileID, nil, bobToken)
	assert.Equal(t, http.StatusOK, resp.StatusCode, resp.String())

	// =========================================================================
	// 8. Create a pool and add the file
	// =========================================================================
	resp = h.doJSON("POST", "/pools", map[string]any{
		"name": "alice's pool", "is_public": false,
	}, aliceToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
	var poolObj map[string]any
	resp.decode(t, &poolObj)
	poolID := poolObj["id"].(string)
	assert.Equal(t, "alice's pool", poolObj["name"])

	resp = h.doJSON("POST", "/pools/"+poolID+"/files", map[string]any{
		"file_ids": []string{fileID},
	}, aliceToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())

	// Pool file count should now be 1.
	resp = h.doJSON("GET", "/pools/"+poolID, nil, aliceToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var poolFull map[string]any
	resp.decode(t, &poolFull)
	assert.Equal(t, float64(1), poolFull["file_count"])

	// List pool files and verify position.
	resp = h.doJSON("GET", "/pools/"+poolID+"/files", nil, aliceToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var poolFiles map[string]any
	resp.decode(t, &poolFiles)
	poolItems := poolFiles["items"].([]any)
	require.Len(t, poolItems, 1)
	assert.Equal(t, fileID, poolItems[0].(map[string]any)["id"])

	// =========================================================================
	// 9. Trash flow: soft-delete → list trash → restore → permanent delete
	// =========================================================================

	// Soft-delete the file.
	resp = h.doJSON("DELETE", "/files/"+fileID, nil, aliceToken)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, resp.String())

	// File no longer appears in normal listing.
	resp = h.doJSON("GET", "/files", nil, aliceToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var normalPage map[string]any
	resp.decode(t, &normalPage)
	normalItems, _ := normalPage["items"].([]any)
	assert.Len(t, normalItems, 0)

	// File appears in trash listing.
	resp = h.doJSON("GET", "/files?trash=true", nil, aliceToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var trashPage map[string]any
	resp.decode(t, &trashPage)
	trashItems := trashPage["items"].([]any)
	require.Len(t, trashItems, 1)
	assert.Equal(t, fileID, trashItems[0].(map[string]any)["id"])
	assert.Equal(t, true, trashItems[0].(map[string]any)["is_deleted"])

	// Restore the file.
	resp = h.doJSON("POST", "/files/"+fileID+"/restore", nil, aliceToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())

	// File is back in normal listing.
	resp = h.doJSON("GET", "/files", nil, aliceToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var restoredPage map[string]any
	resp.decode(t, &restoredPage)
	restoredItems := restoredPage["items"].([]any)
	require.Len(t, restoredItems, 1)
	assert.Equal(t, fileID, restoredItems[0].(map[string]any)["id"])

	// Soft-delete again then permanently delete.
	resp = h.doJSON("DELETE", "/files/"+fileID, nil, aliceToken)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	resp = h.doJSON("DELETE", "/files/"+fileID+"/permanent", nil, aliceToken)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, resp.String())

	// File is gone entirely.
	resp = h.doJSON("GET", "/files/"+fileID, nil, aliceToken)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, resp.String())

	// =========================================================================
	// 10. Audit log records actions (admin only)
	// =========================================================================
	resp = h.doJSON("GET", "/audit", nil, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var auditPage map[string]any
	resp.decode(t, &auditPage)
	auditItems := auditPage["items"].([]any)
	assert.NotEmpty(t, auditItems, "audit log should have entries")

	// Non-admin cannot read the audit log.
	resp = h.doJSON("GET", "/audit", nil, aliceToken)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, resp.String())
}

// ---------------------------------------------------------------------------
// Additional targeted tests
// ---------------------------------------------------------------------------

// TestBlockedUserCannotLogin verifies that blocking a user prevents login.
func TestBlockedUserCannotLogin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	h := setupSuite(t)
	adminToken := h.login("admin", "admin")

	// Create user.
	resp := h.doJSON("POST", "/users", map[string]any{
		"name": "charlie", "password": "charliepass",
	}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var u map[string]any
	resp.decode(t, &u)
	userID := u["id"].(float64)

	// Block charlie.
	resp = h.doJSON("PATCH", fmt.Sprintf("/users/%.0f", userID), map[string]any{
		"is_blocked": true,
	}, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Login attempt should return 403.
	resp = h.doJSON("POST", "/auth/login", map[string]any{
		"name": "charlie", "password": "charliepass",
	}, "")
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestPoolReorder verifies gap-based position reassignment.
func TestPoolReorder(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	h := setupSuite(t)
	adminToken := h.login("admin", "admin")

	// Upload two files.
	f1 := h.uploadJPEG(adminToken, "a.jpg")
	f2 := h.uploadJPEG(adminToken, "b.jpg")
	id1 := f1["id"].(string)
	id2 := f2["id"].(string)

	// Create pool and add both files.
	resp := h.doJSON("POST", "/pools", map[string]any{"name": "reorder-test"}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var pool map[string]any
	resp.decode(t, &pool)
	poolID := pool["id"].(string)

	resp = h.doJSON("POST", "/pools/"+poolID+"/files", map[string]any{
		"file_ids": []string{id1, id2},
	}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Verify initial order: id1 before id2.
	resp = h.doJSON("GET", "/pools/"+poolID+"/files", nil, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var page map[string]any
	resp.decode(t, &page)
	items := page["items"].([]any)
	require.Len(t, items, 2)
	assert.Equal(t, id1, items[0].(map[string]any)["id"])
	assert.Equal(t, id2, items[1].(map[string]any)["id"])

	// Reorder: id2 first.
	resp = h.doJSON("PUT", "/pools/"+poolID+"/files/reorder", map[string]any{
		"file_ids": []string{id2, id1},
	}, adminToken)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, resp.String())

	// Verify new order.
	resp = h.doJSON("GET", "/pools/"+poolID+"/files", nil, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var page2 map[string]any
	resp.decode(t, &page2)
	items2 := page2["items"].([]any)
	require.Len(t, items2, 2)
	assert.Equal(t, id2, items2[0].(map[string]any)["id"])
	assert.Equal(t, id1, items2[1].(map[string]any)["id"])
}

// TestTagRuleActivateApplyToExisting verifies that activating a rule with
// apply_to_existing=true retroactively tags existing files, including
// transitive rules (A→B active+apply, B→C already active → file gets A,B,C).
func TestTagRuleActivateApplyToExisting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	h := setupSuite(t)
	tok := h.login("admin", "admin")

	// Create three tags: A, B, C.
	mkTag := func(name string) string {
		resp := h.doJSON("POST", "/tags", map[string]any{"name": name}, tok)
		require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
		var obj map[string]any
		resp.decode(t, &obj)
		return obj["id"].(string)
	}
	tagA := mkTag("animal")
	tagB := mkTag("living-thing")
	tagC := mkTag("organism")

	// Rule A→B: created inactive so it does NOT fire on assign.
	resp := h.doJSON("POST", "/tags/"+tagA+"/rules", map[string]any{
		"then_tag_id": tagB,
		"is_active":   false,
	}, tok)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())

	// Rule B→C: active, so it fires transitively when B is applied.
	resp = h.doJSON("POST", "/tags/"+tagB+"/rules", map[string]any{
		"then_tag_id": tagC,
		"is_active":   true,
	}, tok)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())

	// Upload a file and assign only tag A. A→B is inactive so only A is set.
	file := h.uploadJPEG(tok, "cat.jpg")
	fileID := file["id"].(string)
	resp = h.doJSON("PUT", "/files/"+fileID+"/tags", map[string]any{
		"tag_ids": []string{tagA},
	}, tok)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())

	tagNames := func() []string {
		r := h.doJSON("GET", "/files/"+fileID+"/tags", nil, tok)
		require.Equal(t, http.StatusOK, r.StatusCode)
		var items []any
		r.decode(t, &items)
		names := make([]string, 0, len(items))
		for _, it := range items {
			names = append(names, it.(map[string]any)["name"].(string))
		}
		return names
	}

	// Before activation: file should only have tag A.
	assert.ElementsMatch(t, []string{"animal"}, tagNames())

	// Activate A→B WITHOUT apply_to_existing — existing file must not change.
	resp = h.doJSON("PATCH", "/tags/"+tagA+"/rules/"+tagB, map[string]any{
		"is_active":         true,
		"apply_to_existing": false,
	}, tok)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())
	assert.ElementsMatch(t, []string{"animal"}, tagNames(), "file should be unchanged when apply_to_existing=false")

	// Deactivate again so we can test the positive case cleanly.
	resp = h.doJSON("PATCH", "/tags/"+tagA+"/rules/"+tagB, map[string]any{
		"is_active": false,
	}, tok)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())

	// Activate A→B WITH apply_to_existing=true.
	// Expectation: file gets B directly, and C transitively via the active B→C rule.
	resp = h.doJSON("PATCH", "/tags/"+tagA+"/rules/"+tagB, map[string]any{
		"is_active":         true,
		"apply_to_existing": true,
	}, tok)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())
	assert.ElementsMatch(t, []string{"animal", "living-thing", "organism"}, tagNames())
}

// TestTagAutoRule verifies that adding a tag automatically applies then_tags.
func TestTagAutoRule(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	h := setupSuite(t)
	adminToken := h.login("admin", "admin")

	// Create two tags: "outdoor" and "nature".
	resp := h.doJSON("POST", "/tags", map[string]any{"name": "outdoor"}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var outdoor map[string]any
	resp.decode(t, &outdoor)
	outdoorID := outdoor["id"].(string)

	resp = h.doJSON("POST", "/tags", map[string]any{"name": "nature"}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var nature map[string]any
	resp.decode(t, &nature)
	natureID := nature["id"].(string)

	// Create rule: when "outdoor" → also apply "nature".
	resp = h.doJSON("POST", "/tags/"+outdoorID+"/rules", map[string]any{
		"then_tag_id": natureID,
	}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())

	// Upload a file and assign only "outdoor".
	file := h.uploadJPEG(adminToken, "park.jpg")
	fileID := file["id"].(string)

	resp = h.doJSON("PUT", "/files/"+fileID+"/tags", map[string]any{
		"tag_ids": []string{outdoorID},
	}, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())

	// Both "outdoor" and "nature" should be on the file.
	resp = h.doJSON("GET", "/files/"+fileID+"/tags", nil, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var tagsResp []any
	resp.decode(t, &tagsResp)
	names := make([]string, 0, len(tagsResp))
	for _, tg := range tagsResp {
		names = append(names, tg.(map[string]any)["name"].(string))
	}
	assert.ElementsMatch(t, []string{"outdoor", "nature"}, names)
}

// TestRecordFileView verifies that viewing a file is logged (POST .../views),
// is repeatable (view history, not a unique flag), and 404s for unknown files.
func TestRecordFileView(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	h := setupSuite(t)
	adminToken := h.login("admin", "admin")

	file := h.uploadJPEG(adminToken, "seen.jpg")
	fileID := file["id"].(string)

	resp := h.doJSON("POST", "/files/"+fileID+"/views", nil, adminToken)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, resp.String())

	// Viewing again logs another history row, not a conflict.
	resp = h.doJSON("POST", "/files/"+fileID+"/views", nil, adminToken)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, resp.String())

	// Unknown file id → 404.
	resp = h.doJSON("POST", "/files/00000000-0000-0000-0000-000000000000/views", nil, adminToken)
	require.Equal(t, http.StatusNotFound, resp.StatusCode, resp.String())
}

// TestRecordTagUses verifies that filtering files by tags logs to
// activity.tag_uses — included tags as is_included=true, negated ones as
// false — while an unfiltered listing and follow-up pagination record nothing.
func TestRecordTagUses(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	h := setupSuite(t)
	ctx := context.Background()
	adminToken := h.login("admin", "admin")

	resp := h.doJSON("POST", "/tags", map[string]any{"name": "sea"}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
	var sea map[string]any
	resp.decode(t, &sea)
	seaID := sea["id"].(string)

	resp = h.doJSON("POST", "/tags", map[string]any{"name": "sky"}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
	var sky map[string]any
	resp.decode(t, &sky)
	skyID := sky["id"].(string)

	// Two files both tagged "sea", so a paged {t=sea} listing has a second page.
	for _, name := range []string{"a.jpg", "b.jpg"} {
		f := h.uploadJPEG(adminToken, name)
		resp = h.doJSON("PUT", "/files/"+f["id"].(string)+"/tags",
			map[string]any{"tag_ids": []string{seaID}}, adminToken)
		require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())
	}

	// An unfiltered listing must not touch tag_uses.
	resp = h.doJSON("GET", "/files", nil, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())
	require.Equal(t, 0, h.countTagUses(ctx), "unfiltered list should record nothing")

	// Include "sea": {t=sea}, one item per page so a next_cursor comes back.
	resp = h.doJSON("GET", "/files?limit=1&filter=%7Bt%3D"+seaID+"%7D", nil, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())
	var page1 map[string]any
	resp.decode(t, &page1)
	nextCursor, _ := page1["next_cursor"].(string)
	require.NotEmpty(t, nextCursor, "expected a next_cursor for page 2")

	// Exclude "sky": {!,t=sky}
	resp = h.doJSON("GET", "/files?filter=%7B%21%2Ct%3D"+skyID+"%7D", nil, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())

	uses := h.tagUses(ctx)
	require.Len(t, uses, 2, "expected one row per filtered tag")
	assert.True(t, uses[seaID], "included tag should be is_included=true")
	assert.False(t, uses[skyID], "negated tag should be is_included=false")

	// Page 2 (cursor present) is pagination, not a fresh filter — no new row.
	resp = h.doJSON("GET", "/files?limit=1&cursor="+nextCursor+"&filter=%7Bt%3D"+seaID+"%7D",
		nil, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())
	assert.Equal(t, 2, h.countTagUses(ctx), "pagination should not add tag_uses rows")
}

// TestTagSortByCategoryThenName verifies the category_name sort groups tags by
// category and orders them by their own name within each category, with
// uncategorized tags last (NULLS LAST).
func TestTagSortByCategoryThenName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	h := setupSuite(t)
	adminToken := h.login("admin", "admin")

	mkCategory := func(name string) string {
		resp := h.doJSON("POST", "/categories", map[string]any{"name": name}, adminToken)
		require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
		var c map[string]any
		resp.decode(t, &c)
		return c["id"].(string)
	}
	mkTag := func(name string, categoryID *string) {
		body := map[string]any{"name": name}
		if categoryID != nil {
			body["category_id"] = *categoryID
		}
		resp := h.doJSON("POST", "/tags", body, adminToken)
		require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
	}

	alpha := mkCategory("Alpha")
	bravo := mkCategory("Bravo")

	// Insert out of order to prove the sort, not insertion order, decides output.
	mkTag("zebra", &alpha)
	mkTag("mid", &bravo)
	mkTag("solo", nil) // uncategorized
	mkTag("ant", &alpha)

	resp := h.doJSON("GET", "/tags?sort=category_name&order=asc", nil, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())
	var page map[string]any
	resp.decode(t, &page)

	items := page["items"].([]any)
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.(map[string]any)["name"].(string)
	}
	// Alpha (ant, zebra) → Bravo (mid) → uncategorized (solo) last.
	assert.Equal(t, []string{"ant", "zebra", "mid", "solo"}, names)
}

// TestRecordPoolView verifies that viewing a pool is logged (POST .../views),
// is repeatable (view history, not a unique flag), and 404s for unknown pools.
func TestRecordPoolView(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	h := setupSuite(t)
	ctx := context.Background()
	adminToken := h.login("admin", "admin")

	resp := h.doJSON("POST", "/pools", map[string]any{"name": "trip"}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
	var pool map[string]any
	resp.decode(t, &pool)
	poolID := pool["id"].(string)

	resp = h.doJSON("POST", "/pools/"+poolID+"/views", nil, adminToken)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, resp.String())

	// Viewing again logs another history row, not a conflict.
	resp = h.doJSON("POST", "/pools/"+poolID+"/views", nil, adminToken)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, resp.String())

	var n int
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT count(*) FROM activity.pool_views WHERE pool_id = $1`, poolID).Scan(&n))
	assert.Equal(t, 2, n, "each view should add a history row")

	// Unknown pool id → 404.
	resp = h.doJSON("POST", "/pools/00000000-0000-0000-0000-000000000000/views", nil, adminToken)
	require.Equal(t, http.StatusNotFound, resp.StatusCode, resp.String())
}

// TestTagColorOptional verifies a tag can be created without a colour (stored as
// NULL rather than the colour input's default) and that an existing colour can
// be cleared back to none.
func TestTagColorOptional(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	h := setupSuite(t)
	adminToken := h.login("admin", "admin")

	// Created without a colour → color is null.
	resp := h.doJSON("POST", "/tags", map[string]any{"name": "plain"}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
	var plain map[string]any
	resp.decode(t, &plain)
	assert.Nil(t, plain["color"], "tag created without a colour should have null color")

	// Created with a colour → kept verbatim.
	resp = h.doJSON("POST", "/tags", map[string]any{"name": "red", "color": "aabbcc"}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
	var red map[string]any
	resp.decode(t, &red)
	assert.Equal(t, "aabbcc", red["color"])
	redID := red["id"].(string)

	// Clearing the colour (color: null) must store NULL — an empty string would
	// violate the hex CHECK constraint and fail the update.
	resp = h.doJSON("PATCH", "/tags/"+redID, map[string]any{"color": nil}, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())
	var cleared map[string]any
	resp.decode(t, &cleared)
	assert.Nil(t, cleared["color"], "cleared colour should be null")
}

// TestBulkTagAutoRule verifies the bulk add path also applies then_tags.
func TestBulkTagAutoRule(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	h := setupSuite(t)
	adminToken := h.login("admin", "admin")

	resp := h.doJSON("POST", "/tags", map[string]any{"name": "outdoor"}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var outdoor map[string]any
	resp.decode(t, &outdoor)
	outdoorID := outdoor["id"].(string)

	resp = h.doJSON("POST", "/tags", map[string]any{"name": "nature"}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var nature map[string]any
	resp.decode(t, &nature)
	natureID := nature["id"].(string)

	resp = h.doJSON("POST", "/tags/"+outdoorID+"/rules", map[string]any{"then_tag_id": natureID}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())

	file := h.uploadJPEG(adminToken, "park.jpg")
	fileID := file["id"].(string)

	// Bulk-add only "outdoor" to the file.
	resp = h.doJSON("POST", "/files/bulk/tags", map[string]any{
		"file_ids": []string{fileID},
		"action":   "add",
		"tag_ids":  []string{outdoorID},
	}, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())

	// The auto-applied "nature" should be on the file too.
	resp = h.doJSON("GET", "/files/"+fileID+"/tags", nil, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var tagsResp []any
	resp.decode(t, &tagsResp)
	names := make([]string, 0, len(tagsResp))
	for _, tg := range tagsResp {
		names = append(names, tg.(map[string]any)["name"].(string))
	}
	assert.ElementsMatch(t, []string{"outdoor", "nature"}, names)
}

// TestMediaQueryTokenAuth verifies the ?access_token= fallback: it authenticates
// a GET (so media can be opened via a plain link/new tab) but is rejected for a
// non-GET, and a missing token is still 401.
func TestMediaQueryTokenAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	h := setupSuite(t)
	token := h.login("admin", "admin")
	file := h.uploadJPEG(token, "q.jpg")
	fileID := file["id"].(string)

	// GET with token in the query, no Authorization header → 200.
	resp := h.do("GET", "/files/"+fileID+"/content?access_token="+token, nil, "", "")
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())

	// No token anywhere → 401.
	resp = h.do("GET", "/files/"+fileID+"/content", nil, "", "")
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// Query token must NOT authorize a state-changing (non-GET) request → 401.
	resp = h.do("DELETE", "/files/"+fileID+"?access_token="+token, nil, "", "")
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode, resp.String())
}

// ---------------------------------------------------------------------------
// Security regression tests
// ---------------------------------------------------------------------------

// loginPair logs in and returns the full access/refresh token pair.
func (h *harness) loginPair(name, password string) (access, refresh string) {
	h.t.Helper()
	resp := h.doJSON("POST", "/auth/login", map[string]string{"name": name, "password": password}, "")
	require.Equal(h.t, http.StatusOK, resp.StatusCode, "login failed: %s", resp)
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	resp.decode(h.t, &out)
	require.NotEmpty(h.t, out.AccessToken)
	require.NotEmpty(h.t, out.RefreshToken)
	return out.AccessToken, out.RefreshToken
}

// TestRefreshTokenFlow verifies that refresh tokens work (regression for the
// stored-hash mismatch that made /refresh always 401), that a refresh token is
// rejected as a bearer access token, and that rotating a session revokes the
// pre-rotation access token.
func TestRefreshTokenFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	h := setupSuite(t)

	access, refresh := h.loginPair("admin", "admin")

	// A refresh token must not be accepted as a bearer access token.
	resp := h.doJSON("GET", "/users/me", nil, refresh)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, resp.String())

	// Refreshing yields a working new pair.
	resp = h.doJSON("POST", "/auth/refresh", map[string]string{"refresh_token": refresh}, "")
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())
	var pair struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	resp.decode(t, &pair)
	require.NotEmpty(t, pair.AccessToken)

	resp = h.doJSON("GET", "/users/me", nil, pair.AccessToken)
	assert.Equal(t, http.StatusOK, resp.StatusCode, resp.String())

	// The pre-rotation access token is now revoked (its session was rotated away).
	resp = h.doJSON("GET", "/users/me", nil, access)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, resp.String())
}

// TestNonOwnerAccessControl verifies that a non-owner, non-admin user cannot
// read or change another user's object ACL, cannot view or tag another user's
// private file, and cannot trigger a server-side import.
func TestNonOwnerAccessControl(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	h := setupSuite(t)
	adminToken := h.login("admin", "admin")

	mkUser := func(name, pass string) {
		resp := h.doJSON("POST", "/users", map[string]any{
			"name": name, "password": pass, "can_create": true,
		}, adminToken)
		require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
	}
	mkUser("alice", "alicepass")
	mkUser("bob", "bobpass")

	aliceToken := h.login("alice", "alicepass")
	bobToken := h.login("bob", "bobpass")

	// Alice uploads a private file.
	file := h.uploadJPEG(aliceToken, "secret.jpg")
	fileID := file["id"].(string)

	// Bob cannot read the file's ACL...
	resp := h.doJSON("GET", "/acl/file/"+fileID, nil, bobToken)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, resp.String())

	// ...nor grant himself access.
	resp = h.doJSON("PUT", "/acl/file/"+fileID, map[string]any{
		"permissions": []map[string]any{{"user_id": 2, "can_view": true, "can_edit": true}},
	}, bobToken)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, resp.String())

	// ...and still cannot view it.
	resp = h.doJSON("GET", "/files/"+fileID, nil, bobToken)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, resp.String())

	// Bob cannot list or modify tags on Alice's private file.
	resp = h.doJSON("GET", "/files/"+fileID+"/tags", nil, bobToken)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, resp.String())

	resp = h.doJSON("PUT", "/files/"+fileID+"/tags", map[string]any{
		"tag_ids": []string{"11111111-1111-1111-1111-111111111111"},
	}, bobToken)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, resp.String())

	// A non-admin cannot trigger a server-side import.
	resp = h.doJSON("POST", "/files/import", map[string]any{}, bobToken)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, resp.String())

	// The owner can still manage her own file's ACL.
	resp = h.doJSON("GET", "/acl/file/"+fileID, nil, aliceToken)
	assert.Equal(t, http.StatusOK, resp.StatusCode, resp.String())
}

// importEvent mirrors service.ImportEvent for decoding the streamed progress.
type importEvent struct {
	Type     string `json:"type"`
	Total    int    `json:"total"`
	Index    int    `json:"index"`
	Filename string `json:"filename"`
	Status   string `json:"status"`
	Reason   string `json:"reason"`
	Imported int    `json:"imported"`
	Skipped  int    `json:"skipped"`
	Errors   int    `json:"errors"`
}

// parseImportEvents splits an NDJSON import response into its events.
func parseImportEvents(t *testing.T, resp *testResponse) []importEvent {
	t.Helper()
	var events []importEvent
	for _, line := range bytes.Split(resp.bodyBytes, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var ev importEvent
		require.NoError(t, json.Unmarshal(line, &ev), "event line: %s", line)
		events = append(events, ev)
	}
	return events
}

// TestImportFromFolder verifies the admin server-side import: supported files
// are ingested, subdirectories are skipped, the source is removed from the
// import folder afterwards, and a file without EXIF takes the source's mtime as
// its content_datetime.
func TestImportFromFolder(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	h := setupSuite(t)
	adminToken := h.login("admin", "admin")

	// Drop a non-EXIF JPEG into the import folder with a known mtime, plus a
	// subdirectory that must be skipped.
	srcPath := filepath.Join(h.importDir, "scan.jpg")
	require.NoError(t, os.WriteFile(srcPath, minimalJPEG(), 0o644))
	mtime := time.Date(2021, 3, 4, 5, 6, 7, 0, time.UTC)
	require.NoError(t, os.Chtimes(srcPath, mtime, mtime))
	require.NoError(t, os.Mkdir(filepath.Join(h.importDir, "nested"), 0o755))

	resp := h.doJSON("POST", "/files/import", map[string]any{}, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())

	// The import streams newline-delimited JSON progress events.
	events := parseImportEvents(t, resp)
	var start, done *importEvent
	files := map[string]importEvent{}
	for i := range events {
		switch events[i].Type {
		case "start":
			start = &events[i]
		case "done":
			done = &events[i]
		case "file":
			files[events[i].Filename] = events[i]
		}
	}
	require.NotNil(t, start, resp.String())
	require.NotNil(t, done, resp.String())
	assert.Equal(t, 2, start.Total, "start total counts every entry")
	assert.Equal(t, 1, done.Imported, resp.String())
	assert.Equal(t, 1, done.Skipped, resp.String()) // the nested directory
	assert.Equal(t, 0, done.Errors, resp.String())

	// Per-file events: the JPEG imported, the subdirectory was skipped.
	assert.Equal(t, "imported", files["scan.jpg"].Status, resp.String())
	assert.Equal(t, "skipped", files["nested"].Status, resp.String())

	// Source file is gone from the import folder after a successful import.
	_, statErr := os.Stat(srcPath)
	assert.True(t, os.IsNotExist(statErr), "source should be removed after import")

	// The imported file took the mtime as content_datetime (no EXIF present).
	listResp := h.doJSON("GET", "/files?limit=10", nil, adminToken)
	require.Equal(t, http.StatusOK, listResp.StatusCode, listResp.String())
	var list struct {
		Items []struct {
			ContentDatetime string `json:"content_datetime"`
		} `json:"items"`
	}
	listResp.decode(t, &list)
	require.Len(t, list.Items, 1, listResp.String())
	ct, err := time.Parse(time.RFC3339, list.Items[0].ContentDatetime)
	require.NoError(t, err)
	assert.True(t, ct.Equal(mtime), "content_datetime %v should equal mtime %v", ct, mtime)
}

// TestContentRangeRequests verifies the original-content endpoint answers a
// byte-range request with 206 Partial Content (so the browser can seek within
// audio/video) rather than streaming the whole body.
func TestContentRangeRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	h := setupSuite(t)
	token := h.login("admin", "admin")
	file := h.uploadJPEG(token, "clip.jpg")
	id := file["id"].(string)

	req, err := http.NewRequest("GET", h.url("/files/"+id+"/content?inline=1"), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Range", "bytes=0-9")
	resp, err := h.client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusPartialContent, resp.StatusCode)
	assert.Equal(t, "bytes", resp.Header.Get("Accept-Ranges"))
	assert.Regexp(t, `^bytes 0-9/\d+$`, resp.Header.Get("Content-Range"))
	require.Len(t, body, 10)
	assert.Equal(t, minimalJPEG()[:10], body)
}

// TestBlockRevokesActiveSessions verifies that blocking a user immediately
// invalidates their outstanding access tokens.
func TestBlockRevokesActiveSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	h := setupSuite(t)
	adminToken := h.login("admin", "admin")

	resp := h.doJSON("POST", "/users", map[string]any{"name": "dave", "password": "davepass"}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
	var dave map[string]any
	resp.decode(t, &dave)
	daveID := dave["id"].(float64)

	daveToken := h.login("dave", "davepass")
	resp = h.doJSON("GET", "/users/me", nil, daveToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())

	// Block dave.
	resp = h.doJSON("PATCH", fmt.Sprintf("/users/%.0f", daveID), map[string]any{"is_blocked": true}, adminToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())

	// Dave's previously-issued access token is now rejected.
	resp = h.doJSON("GET", "/users/me", nil, daveToken)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, resp.String())
}

// fileListIDs returns the set of file IDs visible to token via GET /files.
func (h *harness) fileListIDs(token string) map[string]bool {
	h.t.Helper()
	resp := h.doJSON("GET", "/files", nil, token)
	require.Equal(h.t, http.StatusOK, resp.StatusCode, resp.String())
	var page map[string]any
	resp.decode(h.t, &page)
	ids := map[string]bool{}
	if items, ok := page["items"].([]any); ok {
		for _, it := range items {
			if m, ok := it.(map[string]any); ok {
				if id, ok := m["id"].(string); ok {
					ids[id] = true
				}
			}
		}
	}
	return ids
}

// TestPrivateByDefaultVisibility verifies that listings only return rows the
// caller may see: private files are hidden from non-owners, public files are
// visible to all, an explicit grant reveals a private file, and admins see all.
func TestPrivateByDefaultVisibility(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	h := setupSuite(t)
	adminToken := h.login("admin", "admin")

	resp := h.doJSON("POST", "/users", map[string]any{"name": "alice", "password": "alicepass"}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
	resp = h.doJSON("POST", "/users", map[string]any{"name": "bob", "password": "bobpass"}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
	var bob map[string]any
	resp.decode(t, &bob)
	bobID := bob["id"].(float64)

	aliceToken := h.login("alice", "alicepass")
	bobToken := h.login("bob", "bobpass")

	file := h.uploadJPEG(aliceToken, "alice-secret.jpg")
	fileID := file["id"].(string)

	// Owner and admin see it; the unrelated user does not.
	assert.True(t, h.fileListIDs(aliceToken)[fileID], "owner should see own file")
	assert.True(t, h.fileListIDs(adminToken)[fileID], "admin should see all files")
	assert.False(t, h.fileListIDs(bobToken)[fileID], "private file must not appear for a non-owner")

	// Making it public reveals it to everyone.
	resp = h.doJSON("PATCH", "/files/"+fileID, map[string]any{"is_public": true}, aliceToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())
	assert.True(t, h.fileListIDs(bobToken)[fileID], "public file should be visible to all")

	// Private again → hidden; an explicit view grant reveals it.
	resp = h.doJSON("PATCH", "/files/"+fileID, map[string]any{"is_public": false}, aliceToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())
	assert.False(t, h.fileListIDs(bobToken)[fileID])

	resp = h.doJSON("PUT", "/acl/file/"+fileID, map[string]any{
		"permissions": []map[string]any{{"user_id": bobID, "can_view": true, "can_edit": false}},
	}, aliceToken)
	require.Equal(t, http.StatusOK, resp.StatusCode, resp.String())
	assert.True(t, h.fileListIDs(bobToken)[fileID], "granted file should be visible in the listing")
}

// TestPoolOperationsRequireACL verifies that a non-owner cannot view or modify
// another user's private pool's contents.
func TestPoolOperationsRequireACL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	h := setupSuite(t)
	adminToken := h.login("admin", "admin")

	resp := h.doJSON("POST", "/users", map[string]any{"name": "alice", "password": "alicepass"}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
	resp = h.doJSON("POST", "/users", map[string]any{"name": "bob", "password": "bobpass"}, adminToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())

	aliceToken := h.login("alice", "alicepass")
	bobToken := h.login("bob", "bobpass")

	file := h.uploadJPEG(aliceToken, "f.jpg")
	fileID := file["id"].(string)

	resp = h.doJSON("POST", "/pools", map[string]any{"name": "alice pool", "is_public": false}, aliceToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())
	var pool map[string]any
	resp.decode(t, &pool)
	poolID := pool["id"].(string)

	resp = h.doJSON("POST", "/pools/"+poolID+"/files", map[string]any{"file_ids": []string{fileID}}, aliceToken)
	require.Equal(t, http.StatusCreated, resp.StatusCode, resp.String())

	// Bob cannot view the pool, list its files, or mutate its membership.
	for _, c := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/pools/" + poolID, nil},
		{"GET", "/pools/" + poolID + "/files", nil},
		{"POST", "/pools/" + poolID + "/files", map[string]any{"file_ids": []string{fileID}}},
		{"POST", "/pools/" + poolID + "/files/remove", map[string]any{"file_ids": []string{fileID}}},
		{"PUT", "/pools/" + poolID + "/files/reorder", map[string]any{"file_ids": []string{fileID}}},
	} {
		resp = h.doJSON(c.method, c.path, c.body, bobToken)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "%s %s: %s", c.method, c.path, resp)
	}

	// The owner can still list the pool's files.
	resp = h.doJSON("GET", "/pools/"+poolID+"/files", nil, aliceToken)
	assert.Equal(t, http.StatusOK, resp.StatusCode, resp.String())
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// minimalJPEG returns the bytes of a 1×1 white JPEG image.
// Generated offline; no external dependency needed.
func minimalJPEG() []byte {
	// This is a valid minimal JPEG: SOI + APP0 + DQT + SOF0 + DHT + SOS + EOI.
	// 1×1 white pixel, quality ~50. Verified with `file` and browsers.
	return []byte{
		0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xff, 0xdb, 0x00, 0x43,
		0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
		0x09, 0x08, 0x0a, 0x0c, 0x14, 0x0d, 0x0c, 0x0b, 0x0b, 0x0c, 0x19, 0x12,
		0x13, 0x0f, 0x14, 0x1d, 0x1a, 0x1f, 0x1e, 0x1d, 0x1a, 0x1c, 0x1c, 0x20,
		0x24, 0x2e, 0x27, 0x20, 0x22, 0x2c, 0x23, 0x1c, 0x1c, 0x28, 0x37, 0x29,
		0x2c, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1f, 0x27, 0x39, 0x3d, 0x38, 0x32,
		0x3c, 0x2e, 0x33, 0x34, 0x32, 0xff, 0xc0, 0x00, 0x0b, 0x08, 0x00, 0x01,
		0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xff, 0xc4, 0x00, 0x1f, 0x00, 0x00,
		0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0xff, 0xc4, 0x00, 0xb5, 0x10, 0x00, 0x02, 0x01, 0x03,
		0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7d,
		0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06,
		0x13, 0x51, 0x61, 0x07, 0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xa1, 0x08,
		0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0, 0x24, 0x33, 0x62, 0x72,
		0x82, 0x09, 0x0a, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28,
		0x29, 0x2a, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x43, 0x44, 0x45,
		0x46, 0x47, 0x48, 0x49, 0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
		0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x73, 0x74, 0x75,
		0x76, 0x77, 0x78, 0x79, 0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
		0x8a, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4,
		0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7,
		0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca,
		0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2, 0xe3,
		0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea, 0xf1, 0xf2, 0xf3, 0xf4, 0xf5,
		0xf6, 0xf7, 0xf8, 0xf9, 0xfa, 0xff, 0xda, 0x00, 0x08, 0x01, 0x01, 0x00,
		0x00, 0x3f, 0x00, 0xfb, 0xd3, 0xff, 0xd9,
	}
}

// replaceDSNDatabase returns a copy of dsn with the dbname parameter replaced.
// Handles both key=value libpq-style strings and postgres:// URLs.
func replaceDSNDatabase(dsn, newDB string) string {
	// key=value style: replace dbname=xxx or append if absent.
	if !strings.Contains(dsn, "://") {
		const key = "dbname="
		if idx := strings.Index(dsn, key); idx >= 0 {
			end := strings.IndexByte(dsn[idx+len(key):], ' ')
			if end < 0 {
				return dsn[:idx] + key + newDB
			}
			return dsn[:idx] + key + newDB + dsn[idx+len(key)+end:]
		}
		return dsn + " dbname=" + newDB
	}
	// URL style: not used in our defaults, but handled for completeness.
	return dsn
}

// freePort returns an available TCP port on localhost.
func freePort() int {
	l, _ := net.Listen("tcp", ":0")
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// writeFile writes content to a temp file and returns its path.
func writeFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, content, 0o644))
	return path
}

// suppress unused-import warnings for helpers kept for future use.
var (
	_ = freePort
	_ = writeFile
)
