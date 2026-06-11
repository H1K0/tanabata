// Package storage provides a local-filesystem implementation of port.FileStorage.
package storage

import (
	"bytes"
	"context"
	"fmt"
	_ "golang.org/x/image/webp" // register WebP decoder
	"image"
	"image/color"
	_ "image/gif" // register GIF decoder
	"image/jpeg"
	_ "image/png" // register PNG decoder
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

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

// InvalidateCache removes the cached thumbnail and preview for id, if present,
// so they are regenerated from the current file content on the next request.
func (s *DiskStorage) InvalidateCache(_ context.Context, id uuid.UUID) error {
	for _, p := range []string{s.thumbCachePath(id), s.previewCachePath(id)} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("storage.InvalidateCache remove %q: %w", p, err)
		}
	}
	return nil
}

// Thumbnail returns a JPEG that fits within the configured max width×height
// (never upscaled, never cropped). Generated on first call and cached.
// Video files are thumbnailed via ffmpeg; other non-image files get a placeholder.
func (s *DiskStorage) Thumbnail(ctx context.Context, id uuid.UUID) (io.ReadCloser, error) {
	return s.serveGenerated(ctx, id, s.thumbCachePath(id), s.thumbWidth, s.thumbHeight)
}

// Preview returns a JPEG that fits within the configured max width×height
// (never upscaled, never cropped). Generated on first call and cached.
// Video files are thumbnailed via ffmpeg; other non-image files get a placeholder.
func (s *DiskStorage) Preview(ctx context.Context, id uuid.UUID) (io.ReadCloser, error) {
	return s.serveGenerated(ctx, id, s.previewCachePath(id), s.previewWidth, s.previewHeight)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// serveGenerated is the shared implementation for Thumbnail and Preview.
// imaging.Thumbnail fits the source within maxW×maxH without upscaling or cropping.
//
// Resolution order:
//  1. Return cached JPEG if present.
//  2. Decode as still image (JPEG/PNG/GIF via imaging).
//  3. Extract a frame with ffmpeg (video files).
//  4. Solid-colour placeholder (archives, unrecognised formats, etc.).
func (s *DiskStorage) serveGenerated(ctx context.Context, id uuid.UUID, cachePath string, maxW, maxH int) (io.ReadCloser, error) {
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

	// 1. Try still-image decode (JPEG/PNG/GIF), rejecting decompression bombs.
	// 2. Try video frame extraction via ffmpeg.
	// 3. Fall back to placeholder.
	var img image.Image
	if decoded, err := decodeImageLimited(srcPath); err == nil {
		img = imaging.Thumbnail(decoded, maxW, maxH, imaging.Lanczos)
	} else if frame, err := extractVideoFrame(ctx, srcPath); err == nil {
		img = imaging.Thumbnail(frame, maxW, maxH, imaging.Lanczos)
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

// maxDecodePixels caps the pixel count of an image we are willing to decode
// into memory, bounding the cost of a decompression bomb (a tiny file that
// expands to an enormous raster). 64 Mpx is ~ an 8192×8192 image.
const maxDecodePixels = 64 << 20

// decodeImageLimited decodes the image at path after first inspecting its header
// dimensions via image.DecodeConfig (which does not allocate the raster), and
// refuses images whose pixel count exceeds maxDecodePixels.
func decodeImageLimited(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxDecodePixels {
		return nil, fmt.Errorf("image too large to decode: %dx%d", cfg.Width, cfg.Height)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return imaging.Decode(f, imaging.AutoOrientation(true))
}

// extractVideoFrame uses ffmpeg to extract a single frame from a video file.
// It seeks 1 second in (keyframe-accurate fast seek) and pipes the frame out
// as PNG. If the video is shorter than 1 s the seek is silently ignored by
// ffmpeg and the first available frame is returned instead.
// Returns an error if ffmpeg is not installed or produces no output. The run is
// bounded by a timeout so a malformed file cannot hang the request indefinitely.
func extractVideoFrame(ctx context.Context, srcPath string) (image.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-ss", "1", // fast input seek; ignored gracefully on short files
		"-i", srcPath,
		"-vframes", "1",
		"-f", "image2",
		"-vcodec", "png",
		"pipe:1",
	)
	cmd.Stdout = &out
	cmd.Stderr = io.Discard // suppress ffmpeg progress output

	if err := cmd.Run(); err != nil || out.Len() == 0 {
		return nil, fmt.Errorf("ffmpeg frame extract: %w", err)
	}
	return imaging.Decode(&out)
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
