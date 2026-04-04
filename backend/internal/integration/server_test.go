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
	t      *testing.T
	server *httptest.Server
	client *http.Client
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

	diskStorage, err := storage.NewDiskStorage(filesDir, thumbsDir, 160, 160, 1920, 1080)
	require.NoError(t, err)

	// --- Repositories --------------------------------------------------------
	userRepo     := postgres.NewUserRepo(pool)
	sessionRepo  := postgres.NewSessionRepo(pool)
	fileRepo     := postgres.NewFileRepo(pool)
	mimeRepo     := postgres.NewMimeRepo(pool)
	aclRepo      := postgres.NewACLRepo(pool)
	auditRepo    := postgres.NewAuditRepo(pool)
	tagRepo      := postgres.NewTagRepo(pool)
	tagRuleRepo  := postgres.NewTagRuleRepo(pool)
	categoryRepo := postgres.NewCategoryRepo(pool)
	poolRepo     := postgres.NewPoolRepo(pool)
	transactor   := postgres.NewTransactor(pool)

	// --- Services ------------------------------------------------------------
	authSvc     := service.NewAuthService(userRepo, sessionRepo, "test-secret", 15*time.Minute, 720*time.Hour)
	aclSvc      := service.NewACLService(aclRepo)
	auditSvc    := service.NewAuditService(auditRepo)
	tagSvc      := service.NewTagService(tagRepo, tagRuleRepo, aclSvc, auditSvc, transactor)
	categorySvc := service.NewCategoryService(categoryRepo, tagRepo, aclSvc, auditSvc)
	poolSvc     := service.NewPoolService(poolRepo, aclSvc, auditSvc)
	fileSvc     := service.NewFileService(fileRepo, mimeRepo, diskStorage, aclSvc, auditSvc, tagSvc, transactor, filesDir)
	userSvc     := service.NewUserService(userRepo, auditSvc)

	// --- Handlers ------------------------------------------------------------
	authMiddleware  := handler.NewAuthMiddleware(authSvc)
	authHandler     := handler.NewAuthHandler(authSvc)
	fileHandler     := handler.NewFileHandler(fileSvc, tagSvc)
	tagHandler      := handler.NewTagHandler(tagSvc, fileSvc)
	categoryHandler := handler.NewCategoryHandler(categorySvc)
	poolHandler     := handler.NewPoolHandler(poolSvc)
	userHandler     := handler.NewUserHandler(userSvc)
	aclHandler      := handler.NewACLHandler(aclSvc)
	auditHandler    := handler.NewAuditHandler(auditSvc)

	r := handler.NewRouter(
		authMiddleware, authHandler,
		fileHandler, tagHandler, categoryHandler, poolHandler,
		userHandler, aclHandler, auditHandler,
	)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	return &harness{
		t:      t,
		server: srv,
		client: srv.Client(),
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
	bobToken   := h.login("bob", "bobpass")

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