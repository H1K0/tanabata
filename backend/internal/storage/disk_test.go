package storage

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
)

// writeTestImage writes a w×h PNG filled with a distinct (non-placeholder)
// colour so a generated thumbnail can be told apart from the grey placeholder.
func writeTestImage(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{R: 0xC0, G: 0x10, B: 0x20, A: 0xFF})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeImageLimited(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "img.png")
	writeTestImage(t, p, 100, 80) // 8000 px

	if _, err := decodeImageLimited(p, 4000); err == nil {
		t.Fatal("expected rejection for an image over the pixel cap")
	}

	img, err := decodeImageLimited(p, 100000)
	if err != nil {
		t.Fatalf("expected decode within the cap, got %v", err)
	}
	if b := img.Bounds(); b.Dx() != 100 || b.Dy() != 80 {
		t.Fatalf("unexpected decoded size %v", b.Size())
	}
}

// TestThumbnailGeneratesAndCaches exercises the full generation path (semaphore
// acquire → decode → fit → encode → cache) and the cache fast path on re-request.
func TestThumbnailGeneratesAndCaches(t *testing.T) {
	files := t.TempDir()
	thumbs := t.TempDir()
	id := uuid.New()
	writeTestImage(t, filepath.Join(files, id.String()), 100, 80)

	s, err := NewDiskStorage(files, thumbs, 160, 160, 1920, 1080, 0, 1)
	if err != nil {
		t.Fatal(err)
	}

	rc, err := s.Thumbnail(context.Background(), id)
	if err != nil {
		t.Fatalf("Thumbnail: %v", err)
	}
	data, _ := io.ReadAll(rc)
	rc.Close()

	out, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode thumbnail: %v", err)
	}
	// The source fits within 160×160, so it is not upscaled.
	if b := out.Bounds(); b.Dx() != 100 || b.Dy() != 80 {
		t.Fatalf("unexpected thumbnail size %v", b.Size())
	}
	// Centre pixel should be the source's red, not the grey placeholder.
	r, g, b, _ := out.At(50, 40).RGBA()
	if !(r>>8 > g>>8+40 && r>>8 > b>>8+40) {
		t.Fatalf("thumbnail is not the source image (got r=%d g=%d b=%d) — fell back to placeholder?", r>>8, g>>8, b>>8)
	}

	// The cache file must now exist, and a second request must serve it.
	if _, err := os.Stat(s.thumbCachePath(id)); err != nil {
		t.Fatalf("cache file not written: %v", err)
	}
	rc2, err := s.Thumbnail(context.Background(), id)
	if err != nil {
		t.Fatalf("Thumbnail (cached): %v", err)
	}
	rc2.Close()
}

// TestThumbnailFallbackWithoutVips forces the pure-Go pipeline (as if vips were
// not installed) and verifies generation still produces the source image.
func TestThumbnailFallbackWithoutVips(t *testing.T) {
	orig := vipsThumbnailPath
	vipsThumbnailPath = ""
	t.Cleanup(func() { vipsThumbnailPath = orig })

	files := t.TempDir()
	thumbs := t.TempDir()
	id := uuid.New()
	writeTestImage(t, filepath.Join(files, id.String()), 100, 80)

	s, err := NewDiskStorage(files, thumbs, 160, 160, 1920, 1080, 0, 1)
	if err != nil {
		t.Fatal(err)
	}

	rc, err := s.Thumbnail(context.Background(), id)
	if err != nil {
		t.Fatalf("Thumbnail: %v", err)
	}
	data, _ := io.ReadAll(rc)
	rc.Close()

	out, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode thumbnail: %v", err)
	}
	if b := out.Bounds(); b.Dx() != 100 || b.Dy() != 80 {
		t.Fatalf("unexpected thumbnail size %v", b.Size())
	}
	r, g, b, _ := out.At(50, 40).RGBA()
	if !(r>>8 > g>>8+40 && r>>8 > b>>8+40) {
		t.Fatalf("fallback produced a placeholder, not the source (r=%d g=%d b=%d)", r>>8, g>>8, b>>8)
	}
}

// TestPreviewGeneratesAndCaches verifies Preview runs through the same pipeline
// with the preview dimensions and its own cache file (not the thumbnail's).
func TestPreviewGeneratesAndCaches(t *testing.T) {
	files := t.TempDir()
	thumbs := t.TempDir()
	id := uuid.New()
	// Larger than the thumbnail box but within the preview box, so the preview
	// keeps full resolution where a thumbnail would shrink it.
	writeTestImage(t, filepath.Join(files, id.String()), 400, 300)

	s, err := NewDiskStorage(files, thumbs, 160, 160, 1920, 1080, 0, 1)
	if err != nil {
		t.Fatal(err)
	}

	rc, err := s.Preview(context.Background(), id)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	data, _ := io.ReadAll(rc)
	rc.Close()

	out, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	// 400×300 fits within 1920×1080, so the preview is not downscaled.
	if b := out.Bounds(); b.Dx() != 400 || b.Dy() != 300 {
		t.Fatalf("unexpected preview size %v", b.Size())
	}
	r, g, b, _ := out.At(200, 150).RGBA()
	if !(r>>8 > g>>8+40 && r>>8 > b>>8+40) {
		t.Fatalf("preview is not the source image (r=%d g=%d b=%d)", r>>8, g>>8, b>>8)
	}

	// The preview cache must be written, and the thumbnail cache must not — they
	// are separate files served by the same code with different dimensions.
	if _, err := os.Stat(s.previewCachePath(id)); err != nil {
		t.Fatalf("preview cache not written: %v", err)
	}
	if _, err := os.Stat(s.thumbCachePath(id)); err == nil {
		t.Fatal("thumbnail cache should not exist after a preview-only request")
	}
}
