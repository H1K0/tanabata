// Package storage provides a local-filesystem implementation of port.FileStorage.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/gif" // register GIF decoder
	_ "image/png" // register PNG decoder
	"io"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

// DiskStorage implements port.FileStorage using the local filesystem.
//
// Directory layout:
//
//	{filesPath}/{id}                — original file (UUID basename, no extension)
//	{thumbsPath}/{id}_thumb.jpg     — thumbnail cache
//	{thumbsPath}/{id}_preview.jpg   — preview cache
type DiskStorage struct {
	filesPath     string
	thumbsPath    string
	thumbWidth    int
	thumbHeight   int
	previewWidth  int
	previewHeight int
}

var _ port.FileStorage = (*DiskStorage)(nil)

// NewDiskStorage creates a DiskStorage and ensures both directories exist.
func NewDiskStorage(
	filesPath, thumbsPath string,
	thumbW, thumbH, prevW, prevH int,
) (*DiskStorage, error) {
	for _, p := range []string{filesPath, thumbsPath} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			return nil, fmt.Errorf("storage: create directory %q: %w", p, err)
		}
	}
	return &DiskStorage{
		filesPath:     filesPath,
		thumbsPath:    thumbsPath,
		thumbWidth:    thumbW,
		thumbHeight:   thumbH,
		previewWidth:  prevW,
		previewHeight: prevH,
	}, nil
}

// ---------------------------------------------------------------------------
// port.FileStorage implementation
// ---------------------------------------------------------------------------

// Save writes r to {filesPath}/{id} and returns the number of bytes written.
func (s *DiskStorage) Save(_ context.Context, id uuid.UUID, r io.Reader) (int64, error) {
	dst := s.originalPath(id)
	f, err := os.Create(dst)
	if err != nil {
		return 0, fmt.Errorf("storage.Save create %q: %w", dst, err)
	}
	n, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		os.Remove(dst)
		return 0, fmt.Errorf("storage.Save write: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(dst)
		return 0, fmt.Errorf("storage.Save close: %w", closeErr)
	}
	return n, nil
}

// Read opens the original file for reading. The caller must close the result.
func (s *DiskStorage) Read(_ context.Context, id uuid.UUID) (io.ReadCloser, error) {
	f, err := os.Open(s.originalPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("storage.Read: %w", err)
	}
	return f, nil
}

// Delete removes the original file. Returns ErrNotFound if it does not exist.
func (s *DiskStorage) Delete(_ context.Context, id uuid.UUID) error {
	if err := os.Remove(s.originalPath(id)); err != nil {
		if os.IsNotExist(err) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("storage.Delete: %w", err)
	}
	return nil
}

// Thumbnail returns a JPEG that fits within the configured max width×height
// (never upscaled, never cropped). Generated on first call and cached.
// Non-image source files receive a solid-colour placeholder.
func (s *DiskStorage) Thumbnail(_ context.Context, id uuid.UUID) (io.ReadCloser, error) {
	return s.serveGenerated(id, s.thumbCachePath(id), s.thumbWidth, s.thumbHeight)
}

// Preview returns a JPEG that fits within the configured max width×height
// (never upscaled, never cropped). Generated on first call and cached.
// Non-image source files receive a solid-colour placeholder.
func (s *DiskStorage) Preview(_ context.Context, id uuid.UUID) (io.ReadCloser, error) {
	return s.serveGenerated(id, s.previewCachePath(id), s.previewWidth, s.previewHeight)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// serveGenerated is the shared implementation for Thumbnail and Preview.
// imaging.Thumbnail fits the source within maxW×maxH without upscaling or cropping.
func (s *DiskStorage) serveGenerated(id uuid.UUID, cachePath string, maxW, maxH int) (io.ReadCloser, error) {
	// Fast path: cache hit.
	if f, err := os.Open(cachePath); err == nil {
		return f, nil
	}

	// Verify the original file exists before doing any work.
	srcPath := s.originalPath(id)
	if _, err := os.Stat(srcPath); err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("storage: stat %q: %w", srcPath, err)
	}

	// Attempt to decode as an image. imaging.Open handles JPEG, PNG, and GIF
	// (decoders registered via blank imports). Any non-decodable file (video,
	// archive, …) silently produces a placeholder.
	var img image.Image
	if decoded, err := imaging.Open(srcPath, imaging.AutoOrientation(true)); err == nil {
		img = imaging.Thumbnail(decoded, maxW, maxH, imaging.Lanczos)
	} else {
		img = placeholder(maxW, maxH)
	}

	// Write to cache atomically (temp→rename) and return an open reader.
	if rc, err := writeCache(cachePath, img); err == nil {
		return rc, nil
	}

	// Cache write failed (read-only fs, disk full, …). Serve from an
	// in-memory buffer so the request still succeeds.
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("storage: encode in-memory JPEG: %w", err)
	}
	return io.NopCloser(&buf), nil
}

// writeCache encodes img as JPEG to cachePath via an atomic temp→rename write,
// then opens and returns the cache file.
func writeCache(cachePath string, img image.Image) (io.ReadCloser, error) {
	dir := filepath.Dir(cachePath)
	tmp, err := os.CreateTemp(dir, ".cache-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("storage: create temp file: %w", err)
	}
	tmpName := tmp.Name()

	encErr := jpeg.Encode(tmp, img, &jpeg.Options{Quality: 85})
	closeErr := tmp.Close()
	if encErr != nil {
		os.Remove(tmpName)
		return nil, fmt.Errorf("storage: encode cache JPEG: %w", encErr)
	}
	if closeErr != nil {
		os.Remove(tmpName)
		return nil, fmt.Errorf("storage: close temp file: %w", closeErr)
	}
	if err := os.Rename(tmpName, cachePath); err != nil {
		os.Remove(tmpName)
		return nil, fmt.Errorf("storage: rename cache file: %w", err)
	}
	f, err := os.Open(cachePath)
	if err != nil {
		return nil, fmt.Errorf("storage: open cache file: %w", err)
	}
	return f, nil
}

// ---------------------------------------------------------------------------
// Path helpers
// ---------------------------------------------------------------------------

func (s *DiskStorage) originalPath(id uuid.UUID) string {
	return filepath.Join(s.filesPath, id.String())
}

func (s *DiskStorage) thumbCachePath(id uuid.UUID) string {
	return filepath.Join(s.thumbsPath, id.String()+"_thumb.jpg")
}

func (s *DiskStorage) previewCachePath(id uuid.UUID) string {
	return filepath.Join(s.thumbsPath, id.String()+"_preview.jpg")
}

// ---------------------------------------------------------------------------
// Image helpers
// ---------------------------------------------------------------------------

// placeholder returns a solid-colour image of size w×h for files that cannot
// be decoded as images. Uses #444455 from the design palette.
func placeholder(w, h int) *image.NRGBA {
	return imaging.New(w, h, color.NRGBA{R: 0x44, G: 0x44, B: 0x55, A: 0xFF})
}
