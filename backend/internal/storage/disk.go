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
	"runtime"
	"strconv"
	"strings"
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
	maxPixels     int
	// genSem bounds concurrent thumbnail/preview generation. Each resize already
	// fans out across every core (imaging uses GOMAXPROCS), and large sources cost
	// hundreds of MB to decode, so unbounded parallelism on a burst of big images
	// pegs the CPU and can exhaust RAM. A buffered channel caps how many run at once.
	genSem chan struct{}
}

var _ port.FileStorage = (*DiskStorage)(nil)

// NewDiskStorage creates a DiskStorage and ensures both directories exist.
//
// maxPixels caps the source pixel count we will decode in-process (0 → a sane
// default). concurrency bounds simultaneous generation (≤0 → half the CPUs).
func NewDiskStorage(
	filesPath, thumbsPath string,
	thumbW, thumbH, prevW, prevH int,
	maxPixels, concurrency int,
) (*DiskStorage, error) {
	for _, p := range []string{filesPath, thumbsPath} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			return nil, fmt.Errorf("storage: create directory %q: %w", p, err)
		}
	}
	if maxPixels <= 0 {
		maxPixels = defaultMaxDecodePixels
	}
	if concurrency <= 0 {
		concurrency = max(1, runtime.GOMAXPROCS(0)/2)
	}
	return &DiskStorage{
		filesPath:     filesPath,
		thumbsPath:    thumbsPath,
		thumbWidth:    thumbW,
		thumbHeight:   thumbH,
		previewWidth:  prevW,
		previewHeight: prevH,
		maxPixels:     maxPixels,
		genSem:        make(chan struct{}, concurrency),
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

// Thumbnail returns a JPEG scaled to fit within the configured max width×height,
// preserving the original aspect ratio (never upscaled, never cropped); the grid
// cell letterboxes it as needed. Generated on first call and cached. Video files
// are thumbnailed via ffmpeg; other non-image files get a placeholder.
func (s *DiskStorage) Thumbnail(ctx context.Context, id uuid.UUID) (io.ReadCloser, error) {
	return s.serveGenerated(ctx, id, s.thumbCachePath(id), s.thumbWidth, s.thumbHeight)
}

// Preview returns a JPEG scaled to fit within the configured max width×height,
// preserving the original aspect ratio (never upscaled, never cropped) so the
// viewer shows the whole image. Generated on first call and cached. Video files
// are thumbnailed via ffmpeg; other non-image files get a placeholder.
func (s *DiskStorage) Preview(ctx context.Context, id uuid.UUID) (io.ReadCloser, error) {
	return s.serveGenerated(ctx, id, s.previewCachePath(id), s.previewWidth, s.previewHeight)
}

// VideoFrameMiddle decodes a representative frame from the middle of a video
// (duration/2). The midpoint avoids the shared intros, title cards and black
// lead-in frames that make a fixed early offset collide across unrelated clips,
// so it is the right source for the video's perceptual (duplicate-detection)
// hash. The file must already exist in storage; ffmpeg/ffprobe must be installed.
// This is not part of port.FileStorage — only the dedup CLI needs it, with a
// concrete *DiskStorage — so the interface stays lean and ffmpeg stays out of the
// upload path.
func (s *DiskStorage) VideoFrameMiddle(ctx context.Context, id uuid.UUID) (image.Image, error) {
	srcPath := s.originalPath(id)
	if _, err := os.Stat(srcPath); err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("storage: stat %q: %w", srcPath, err)
	}
	return extractVideoFrameMiddle(ctx, srcPath)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// serveGenerated is the shared implementation for Thumbnail and Preview. Both
// fit the source within maxW×maxH preserving the aspect ratio (no crop, no
// upscale); they differ only in the configured dimensions.
//
// Resolution order:
//  1. Return cached JPEG if present.
//  2. vipsthumbnail (shrink-on-load; the primary still-image path).
//  3. Pure-Go decode + imaging.Fit (fallback when vips is absent).
//  4. Extract a frame with ffmpeg (video files).
//  5. Solid-colour placeholder (archives, unrecognised formats, etc.).
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

	// Bound concurrent generation so a burst of large images can't peg every core
	// or exhaust RAM. Queue here (respecting cancellation) rather than starting
	// the heavy decode immediately.
	select {
	case s.genSem <- struct{}{}:
		defer func() { <-s.genSem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Another request may have generated this while we waited on the semaphore.
	if f, err := os.Open(cachePath); err == nil {
		return f, nil
	}

	// Primary path: vipsthumbnail. It shrinks on load (e.g. JPEG DCT scaling), so
	// even a 200+ Mpx photo is thumbnailed in a fraction of the memory and CPU of a
	// full in-process decode, writing the final JPEG straight to the cache. Falls
	// through when vips is absent or can't read the source (e.g. a video).
	if vipsThumbnailPath != "" {
		if rc, err := s.vipsThumbnail(ctx, srcPath, cachePath, maxW, maxH); err == nil {
			return rc, nil
		}
	}

	// Fallback pipeline (pure Go):
	//  1. Still-image decode (JPEG/PNG/GIF), rejecting oversized rasters.
	//  2. Video frame extraction via ffmpeg.
	//  3. Solid-colour placeholder.
	var img image.Image
	if decoded, err := decodeImageLimited(srcPath, s.maxPixels); err == nil {
		img = imaging.Fit(decoded, maxW, maxH, imaging.Lanczos)
	} else if frame, err := extractVideoFrameMiddle(ctx, srcPath); err == nil {
		img = imaging.Fit(frame, maxW, maxH, imaging.Lanczos)
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

// defaultMaxDecodePixels is the fallback cap when none is configured. It bounds
// the cost of a decompression bomb (a tiny file that expands to an enormous
// raster) and the per-image memory; ~300 Mpx covers e.g. a 13000×17000 photo.
const defaultMaxDecodePixels = 300_000_000

// decodeImageLimited decodes the image at path after first inspecting its header
// dimensions via image.DecodeConfig (which does not allocate the raster), and
// refuses images whose pixel count exceeds maxPixels.
func decodeImageLimited(path string, maxPixels int) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	if int64(cfg.Width)*int64(cfg.Height) > int64(maxPixels) {
		return nil, fmt.Errorf("image too large to decode: %dx%d", cfg.Width, cfg.Height)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return imaging.Decode(f, imaging.AutoOrientation(true))
}

// vipsThumbnailPath is the resolved path to the vipsthumbnail CLI, or "" when it
// isn't installed — in which case generation falls back to the pure-Go pipeline.
var vipsThumbnailPath, _ = exec.LookPath("vipsthumbnail")

// vipsThumbnail generates a JPEG thumbnail with the vipsthumbnail CLI, writing it
// straight to cachePath via an atomic temp→rename. vips decodes large images at a
// reduced scale (shrink-on-load), so this costs a fraction of the memory and CPU
// of a full in-process decode. The result is fit within maxW×maxH and never
// upscaled (the ">" size modifier). Returns an error for inputs vips can't read
// (e.g. videos) so the caller can fall back.
func (s *DiskStorage) vipsThumbnail(ctx context.Context, srcPath, cachePath string, maxW, maxH int) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tmp, err := os.CreateTemp(filepath.Dir(cachePath), ".vips-*.jpg")
	if err != nil {
		return nil, fmt.Errorf("storage: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	_ = tmp.Close()

	cmd := exec.CommandContext(ctx, vipsThumbnailPath,
		srcPath,
		"--size", fmt.Sprintf("%dx%d>", maxW, maxH),
		"--output", tmpName+"[Q=85]",
	)
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		os.Remove(tmpName)
		return nil, fmt.Errorf("vipsthumbnail: %w", err)
	}
	if fi, err := os.Stat(tmpName); err != nil || fi.Size() == 0 {
		os.Remove(tmpName)
		return nil, fmt.Errorf("vipsthumbnail: no output produced")
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

// extractVideoFrameMiddle extracts a single frame from the middle of the video
// (duration/2), falling back to a 1s offset when the duration can't be probed.
// The midpoint dodges shared intros, title cards and black lead-in frames, and
// matches the frame used for the perceptual hash so a video's thumbnail/preview
// shows the same representative frame dedup compared. See extractVideoFrameAt for
// the mechanics.
func extractVideoFrameMiddle(ctx context.Context, srcPath string) (image.Image, error) {
	at := 1.0
	if d, err := videoDurationSeconds(ctx, srcPath); err == nil && d > 0 {
		at = d / 2
	}
	return extractVideoFrameAt(ctx, srcPath, at)
}

// extractVideoFrameAt uses ffmpeg to extract a single frame at atSec seconds into
// the video, piped out as PNG. The fast input seek (-ss before -i) is keyframe-
// accurate and cheap; if atSec is past the end the seek is silently ignored and
// the first available frame is returned instead. Returns an error if ffmpeg is
// not installed or produces no output. The run is bounded by a timeout so a
// malformed file cannot hang the caller indefinitely.
func extractVideoFrameAt(ctx context.Context, srcPath string, atSec float64) (image.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-ss", strconv.FormatFloat(atSec, 'f', 3, 64), // fast input seek; ignored gracefully past end
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

// videoDurationSeconds returns the container duration in seconds via ffprobe.
// Used to seek to the middle of a clip for perceptual hashing and thumbnails.
func videoDurationSeconds(ctx context.Context, srcPath string) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		srcPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe duration: %w", err)
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("ffprobe duration parse %q: %w", out, err)
	}
	return d, nil
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
