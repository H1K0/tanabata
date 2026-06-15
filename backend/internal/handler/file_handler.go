package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/service"
)

// FileHandler handles all /files endpoints.
type FileHandler struct {
	fileSvc        *service.FileService
	tagSvc         *service.TagService
	authSvc        *service.AuthService
	maxUploadBytes int64
}

// NewFileHandler creates a FileHandler. maxUploadBytes caps the size of an
// uploaded or replacement file. authSvc mints content tokens for media URLs.
func NewFileHandler(fileSvc *service.FileService, tagSvc *service.TagService, authSvc *service.AuthService, maxUploadBytes int64) *FileHandler {
	return &FileHandler{fileSvc: fileSvc, tagSvc: tagSvc, authSvc: authSvc, maxUploadBytes: maxUploadBytes}
}

// formFileLimited reads the "file" multipart field while bounding how many bytes
// are read from the request body, then rejects files larger than the configured
// cap. The body limit guards against a dishonest Content-Length; the Size check
// gives a clear rejection for an honestly-declared oversized file.
func (h *FileHandler) formFileLimited(c *gin.Context) (*multipart.FileHeader, bool) {
	// Allow a little slack above the file cap for multipart framing overhead.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxUploadBytes+(1<<20))

	fh, err := c.FormFile("file")
	if err != nil {
		respondError(c, domain.ErrValidation)
		return nil, false
	}
	if fh.Size > h.maxUploadBytes {
		respondError(c, domain.ErrValidation)
		return nil, false
	}
	return fh, true
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

type tagJSON struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Notes         *string `json:"notes"`
	Color         *string `json:"color"`
	CategoryID    *string `json:"category_id"`
	CategoryName  *string `json:"category_name"`
	CategoryColor *string `json:"category_color"`
	CreatorID     int16   `json:"creator_id"`
	CreatorName   string  `json:"creator_name"`
	IsPublic      bool    `json:"is_public"`
	CreatedAt     string  `json:"created_at"`
}

type fileJSON struct {
	ID              string          `json:"id"`
	OriginalName    *string         `json:"original_name"`
	MIMEType        string          `json:"mime_type"`
	MIMEExtension   string          `json:"mime_extension"`
	ContentDatetime string          `json:"content_datetime"`
	Notes           *string         `json:"notes"`
	Metadata        json.RawMessage `json:"metadata"`
	EXIF            json.RawMessage `json:"exif"`
	PHash           *int64          `json:"phash"`
	CreatorID       int16           `json:"creator_id"`
	CreatorName     string          `json:"creator_name"`
	IsPublic        bool            `json:"is_public"`
	IsDeleted       bool            `json:"is_deleted"`
	NeedsReview     bool            `json:"needs_review"`
	CreatedAt       string          `json:"created_at"`
	Tags            []tagJSON       `json:"tags"`
}

func toTagJSON(t domain.Tag) tagJSON {
	j := tagJSON{
		ID:            t.ID.String(),
		Name:          t.Name,
		Notes:         t.Notes,
		Color:         t.Color,
		CategoryName:  t.CategoryName,
		CategoryColor: t.CategoryColor,
		CreatorID:     t.CreatorID,
		CreatorName:   t.CreatorName,
		IsPublic:      t.IsPublic,
		CreatedAt:     t.CreatedAt.Format(time.RFC3339),
	}
	if t.CategoryID != nil {
		s := t.CategoryID.String()
		j.CategoryID = &s
	}
	return j
}

func toFileJSON(f domain.File) fileJSON {
	tags := make([]tagJSON, len(f.Tags))
	for i, t := range f.Tags {
		tags[i] = toTagJSON(t)
	}
	exif := f.EXIF
	if exif == nil {
		exif = json.RawMessage("{}")
	}
	return fileJSON{
		ID:              f.ID.String(),
		OriginalName:    f.OriginalName,
		MIMEType:        f.MIMEType,
		MIMEExtension:   f.MIMEExtension,
		ContentDatetime: f.ContentDatetime.Format(time.RFC3339),
		Notes:           f.Notes,
		Metadata:        f.Metadata,
		EXIF:            exif,
		PHash:           f.PHash,
		CreatorID:       f.CreatorID,
		CreatorName:     f.CreatorName,
		IsPublic:        f.IsPublic,
		IsDeleted:       f.IsDeleted,
		NeedsReview:     f.NeedsReview,
		CreatedAt:       f.CreatedAt.Format(time.RFC3339),
		Tags:            tags,
	}
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func parseFileID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, domain.ErrValidation)
		return uuid.UUID{}, false
	}
	return id, true
}

// ---------------------------------------------------------------------------
// GET /files
// ---------------------------------------------------------------------------

func (h *FileHandler) List(c *gin.Context) {
	params := domain.FileListParams{
		Cursor:    c.Query("cursor"),
		Direction: c.DefaultQuery("direction", "forward"),
		Sort:      c.DefaultQuery("sort", "created"),
		Order:     c.DefaultQuery("order", "desc"),
		Filter:    c.Query("filter"),
		Search:    c.Query("search"),
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		n, err := strconv.Atoi(limitStr)
		if err != nil || n < 1 || n > 200 {
			respondError(c, domain.ErrValidation)
			return
		}
		params.Limit = n
	} else {
		params.Limit = 50
	}

	if anchorStr := c.Query("anchor"); anchorStr != "" {
		id, err := uuid.Parse(anchorStr)
		if err != nil {
			respondError(c, domain.ErrValidation)
			return
		}
		params.Anchor = &id
	}

	if trashStr := c.Query("trash"); trashStr == "true" || trashStr == "1" {
		params.Trash = true
	}

	page, err := h.fileSvc.List(c.Request.Context(), params)
	if err != nil {
		respondError(c, err)
		return
	}

	items := make([]fileJSON, len(page.Items))
	for i, f := range page.Items {
		items[i] = toFileJSON(f)
	}

	respondJSON(c, http.StatusOK, gin.H{
		"items":       items,
		"next_cursor": page.NextCursor,
		"prev_cursor": page.PrevCursor,
	})
}

// ---------------------------------------------------------------------------
// POST /files  (multipart upload)
// ---------------------------------------------------------------------------

func (h *FileHandler) Upload(c *gin.Context) {
	fh, ok := h.formFileLimited(c)
	if !ok {
		return
	}

	src, err := fh.Open()
	if err != nil {
		respondError(c, err)
		return
	}
	defer src.Close()

	// Detect MIME from actual bytes (ignore client-supplied Content-Type).
	mt, err := mimetype.DetectReader(src)
	if err != nil {
		respondError(c, err)
		return
	}
	// Rewind by reopening — FormFile gives a multipart.File which supports Seek.
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		respondError(c, err)
		return
	}
	mimeStr := strings.SplitN(mt.String(), ";", 2)[0]

	params := service.UploadParams{
		Reader:   src,
		MIMEType: mimeStr,
		IsPublic: c.PostForm("is_public") == "true",
	}

	if name := fh.Filename; name != "" {
		params.OriginalName = &name
	}
	if notes := c.PostForm("notes"); notes != "" {
		params.Notes = &notes
	}
	if metaStr := c.PostForm("metadata"); metaStr != "" {
		params.Metadata = json.RawMessage(metaStr)
	}
	if dtStr := c.PostForm("content_datetime"); dtStr != "" {
		t, err := time.Parse(time.RFC3339, dtStr)
		if err != nil {
			respondError(c, domain.ErrValidation)
			return
		}
		params.ContentDatetime = &t
	}
	if tagIDsStr := c.PostForm("tag_ids"); tagIDsStr != "" {
		for _, raw := range strings.Split(tagIDsStr, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			id, err := uuid.Parse(raw)
			if err != nil {
				respondError(c, domain.ErrValidation)
				return
			}
			params.TagIDs = append(params.TagIDs, id)
		}
	}

	f, err := h.fileSvc.Upload(c.Request.Context(), params)
	if err != nil {
		respondError(c, err)
		return
	}

	respondJSON(c, http.StatusCreated, toFileJSON(*f))
}

// ---------------------------------------------------------------------------
// GET /files/:id
// ---------------------------------------------------------------------------

func (h *FileHandler) GetMeta(c *gin.Context) {
	id, ok := parseFileID(c)
	if !ok {
		return
	}

	f, err := h.fileSvc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}

	respondJSON(c, http.StatusOK, toFileJSON(*f))
}

// ---------------------------------------------------------------------------
// POST /files/:id/views
// ---------------------------------------------------------------------------

// RecordView logs that the current user viewed the file (activity.file_views).
func (h *FileHandler) RecordView(c *gin.Context) {
	id, ok := parseFileID(c)
	if !ok {
		return
	}

	if err := h.fileSvc.RecordView(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// PATCH /files/:id
// ---------------------------------------------------------------------------

func (h *FileHandler) UpdateMeta(c *gin.Context) {
	id, ok := parseFileID(c)
	if !ok {
		return
	}

	var body struct {
		OriginalName    *string         `json:"original_name"`
		ContentDatetime *string         `json:"content_datetime"`
		Notes           *string         `json:"notes"`
		Metadata        json.RawMessage `json:"metadata"`
		IsPublic        *bool           `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	params := service.UpdateParams{
		OriginalName: body.OriginalName,
		Notes:        body.Notes,
		Metadata:     body.Metadata,
		IsPublic:     body.IsPublic,
	}
	if body.ContentDatetime != nil {
		t, err := time.Parse(time.RFC3339, *body.ContentDatetime)
		if err != nil {
			respondError(c, domain.ErrValidation)
			return
		}
		params.ContentDatetime = &t
	}

	f, err := h.fileSvc.Update(c.Request.Context(), id, params)
	if err != nil {
		respondError(c, err)
		return
	}

	respondJSON(c, http.StatusOK, toFileJSON(*f))
}

// ---------------------------------------------------------------------------
// DELETE /files/:id  (soft-delete)
// ---------------------------------------------------------------------------

func (h *FileHandler) SoftDelete(c *gin.Context) {
	id, ok := parseFileID(c)
	if !ok {
		return
	}

	if err := h.fileSvc.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// POST /files/:id/content-token
// ---------------------------------------------------------------------------

// CreateContentToken mints a short-lived, single-file capability token the
// client can put in a content URL's access_token query parameter to open or
// stream the original by link (e.g. a long video in a new tab) without the URL
// dying when the 15-minute access token expires. It first enforces view
// permission via fileSvc.Get, so a token is only issued for a file the caller
// may actually read.
func (h *FileHandler) CreateContentToken(c *gin.Context) {
	id, ok := parseFileID(c)
	if !ok {
		return
	}

	// Authorize (and confirm existence) the same way content serving does.
	if _, err := h.fileSvc.Get(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}

	userID, isAdmin, _ := domain.UserFromContext(c.Request.Context())
	token, expiresIn, err := h.authSvc.GenerateContentToken(id.String(), userID, isAdmin)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token, "expires_in": expiresIn})
}

// ---------------------------------------------------------------------------
// GET /files/:id/content
// ---------------------------------------------------------------------------

func (h *FileHandler) GetContent(c *gin.Context) {
	id, ok := parseFileID(c)
	if !ok {
		return
	}

	res, err := h.fileSvc.GetContent(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	defer res.Body.Close()

	c.Header("Content-Type", res.MIMEType)
	c.Header("Cache-Control", "private, max-age=3600")
	// Default to attachment (download); ?inline=1 serves it for in-tab viewing.
	disposition := "attachment"
	if c.Query("inline") == "1" {
		disposition = "inline"
	}
	name := ""
	if res.OriginalName != nil {
		name = *res.OriginalName
		c.Header("Content-Disposition",
			fmt.Sprintf("%s; filename=%q", disposition, name))
	}

	// Serve with byte-range support when the body is seekable (it is for the
	// disk store): http.ServeContent advertises Accept-Ranges and answers Range
	// requests with 206 Partial Content, which is what lets the browser scrub and
	// seek within audio/video. Fall back to a plain stream otherwise.
	if seeker, ok := res.Body.(io.ReadSeeker); ok {
		http.ServeContent(c.Writer, c.Request, name, time.Time{}, seeker)
		return
	}
	c.Status(http.StatusOK)
	io.Copy(c.Writer, res.Body) //nolint:errcheck
}

// ---------------------------------------------------------------------------
// PUT /files/:id/content  (replace)
// ---------------------------------------------------------------------------

func (h *FileHandler) ReplaceContent(c *gin.Context) {
	id, ok := parseFileID(c)
	if !ok {
		return
	}

	fh, ok := h.formFileLimited(c)
	if !ok {
		return
	}

	src, err := fh.Open()
	if err != nil {
		respondError(c, err)
		return
	}
	defer src.Close()

	mt, err := mimetype.DetectReader(src)
	if err != nil {
		respondError(c, err)
		return
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		respondError(c, err)
		return
	}
	mimeStr := strings.SplitN(mt.String(), ";", 2)[0]

	name := fh.Filename
	params := service.UploadParams{
		Reader:       src,
		MIMEType:     mimeStr,
		OriginalName: &name,
	}

	f, err := h.fileSvc.Replace(c.Request.Context(), id, params)
	if err != nil {
		respondError(c, err)
		return
	}

	respondJSON(c, http.StatusOK, toFileJSON(*f))
}

// ---------------------------------------------------------------------------
// GET /files/:id/thumbnail
// ---------------------------------------------------------------------------

func (h *FileHandler) GetThumbnail(c *gin.Context) {
	id, ok := parseFileID(c)
	if !ok {
		return
	}

	rc, err := h.fileSvc.GetThumbnail(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	defer rc.Close()

	c.Header("Content-Type", "image/jpeg")
	c.Header("Cache-Control", "private, max-age=3600")
	c.Status(http.StatusOK)
	io.Copy(c.Writer, rc) //nolint:errcheck
}

// ---------------------------------------------------------------------------
// GET /files/:id/preview
// ---------------------------------------------------------------------------

func (h *FileHandler) GetPreview(c *gin.Context) {
	id, ok := parseFileID(c)
	if !ok {
		return
	}

	rc, err := h.fileSvc.GetPreview(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	defer rc.Close()

	c.Header("Content-Type", "image/jpeg")
	c.Header("Cache-Control", "private, max-age=3600")
	c.Status(http.StatusOK)
	io.Copy(c.Writer, rc) //nolint:errcheck
}

// ---------------------------------------------------------------------------
// POST /files/:id/restore
// ---------------------------------------------------------------------------

func (h *FileHandler) Restore(c *gin.Context) {
	id, ok := parseFileID(c)
	if !ok {
		return
	}

	f, err := h.fileSvc.Restore(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}

	respondJSON(c, http.StatusOK, toFileJSON(*f))
}

// ---------------------------------------------------------------------------
// DELETE /files/:id/permanent
// ---------------------------------------------------------------------------

func (h *FileHandler) PermanentDelete(c *gin.Context) {
	id, ok := parseFileID(c)
	if !ok {
		return
	}

	if err := h.fileSvc.PermanentDelete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// POST /files/bulk/tags
// ---------------------------------------------------------------------------

func (h *FileHandler) BulkSetTags(c *gin.Context) {
	var body struct {
		FileIDs []string `json:"file_ids" binding:"required"`
		Action  string   `json:"action"   binding:"required"`
		TagIDs  []string `json:"tag_ids"  binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}
	if body.Action != "add" && body.Action != "remove" {
		respondError(c, domain.ErrValidation)
		return
	}

	fileIDs, err := parseUUIDs(body.FileIDs)
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}
	tagIDs, err := parseUUIDs(body.TagIDs)
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	applied, err := h.tagSvc.BulkSetTags(c.Request.Context(), fileIDs, body.Action, tagIDs)
	if err != nil {
		respondError(c, err)
		return
	}

	strs := make([]string, len(applied))
	for i, id := range applied {
		strs[i] = id.String()
	}
	respondJSON(c, http.StatusOK, gin.H{"applied_tag_ids": strs})
}

// ---------------------------------------------------------------------------
// POST /files/bulk/delete
// ---------------------------------------------------------------------------

func (h *FileHandler) BulkDelete(c *gin.Context) {
	var body struct {
		FileIDs []string `json:"file_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	fileIDs, err := parseUUIDs(body.FileIDs)
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	if err := h.fileSvc.BulkDelete(c.Request.Context(), fileIDs); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// BulkReview sets the review status on one or more files. A single-file toggle
// is just a one-element file_ids array. Files the caller cannot edit are
// silently skipped (handled in the service).
func (h *FileHandler) BulkReview(c *gin.Context) {
	var body struct {
		FileIDs     []string `json:"file_ids"     binding:"required"`
		NeedsReview *bool    `json:"needs_review" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.NeedsReview == nil {
		respondError(c, domain.ErrValidation)
		return
	}

	fileIDs, err := parseUUIDs(body.FileIDs)
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	if err := h.fileSvc.SetNeedsReview(c.Request.Context(), fileIDs, *body.NeedsReview); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// POST /files/bulk/common-tags
// ---------------------------------------------------------------------------

func (h *FileHandler) CommonTags(c *gin.Context) {
	var body struct {
		FileIDs []string `json:"file_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	fileIDs, err := parseUUIDs(body.FileIDs)
	if err != nil {
		respondError(c, domain.ErrValidation)
		return
	}

	common, partial, err := h.tagSvc.CommonTags(c.Request.Context(), fileIDs)
	if err != nil {
		respondError(c, err)
		return
	}

	toStrs := func(tags []domain.Tag) []string {
		s := make([]string, len(tags))
		for i, t := range tags {
			s[i] = t.ID.String()
		}
		return s
	}

	respondJSON(c, http.StatusOK, gin.H{
		"common_tag_ids":  toStrs(common),
		"partial_tag_ids": toStrs(partial),
	})
}

// ---------------------------------------------------------------------------
// POST /files/import
// ---------------------------------------------------------------------------

func (h *FileHandler) Import(c *gin.Context) {
	// Server-side directory import reads arbitrary paths on the host; restrict
	// it to administrators.
	if !requireAdmin(c) {
		return
	}

	var body struct {
		Path string `json:"path"`
	}
	// Body is optional; ignore bind errors.
	_ = c.ShouldBindJSON(&body)

	// Stream progress as newline-delimited JSON so the client can render a live
	// progress bar and per-file status. Headers are deferred until the first
	// event, so a validation error (bad path, import disabled) raised before any
	// file is touched can still be returned as a normal JSON error response.
	flusher, canFlush := c.Writer.(http.Flusher)
	started := false
	enc := json.NewEncoder(c.Writer)

	emit := func(ev service.ImportEvent) {
		if !started {
			c.Header("Content-Type", "application/x-ndjson")
			c.Header("Cache-Control", "no-cache")
			c.Header("X-Accel-Buffering", "no") // don't let a proxy buffer the stream
			c.Writer.WriteHeader(http.StatusOK)
			started = true
		}
		_ = enc.Encode(ev) // appends a newline
		if canFlush {
			flusher.Flush()
		}
	}

	if _, err := h.fileSvc.Import(c.Request.Context(), body.Path, emit); err != nil {
		if !started {
			respondError(c, err)
			return
		}
		// Headers already sent; surface the failure as a terminal stream event.
		emit(service.ImportEvent{Type: "error", Reason: err.Error()})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseUUIDs(strs []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(strs))
	for _, s := range strs {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}
