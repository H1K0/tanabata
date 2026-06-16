package imagehash

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"testing"
)

// radial renders a smooth grayscale image whose brightness falls off with
// distance from (cx, cy). Smooth gradients are the realistic case for perceptual
// hashing and survive JPEG re-encoding well, so they make stable test fixtures.
func radial(w, h int, cx, cy float64) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	maxD := math.Hypot(float64(w), float64(h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			d := math.Hypot(float64(x)-cx, float64(y)-cy)
			v := uint8(255 * (1 - d/maxD))
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

func encodeJPEG(t *testing.T, img image.Image, quality int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatalf("jpeg encode: %v", err)
	}
	return buf.Bytes()
}

// The same image re-encoded as PNG (lossless) and JPEG (lossy) must hash to a
// small Hamming distance — that is the whole point of a perceptual hash.
func TestFromBytes_SameImageAcrossEncodings(t *testing.T) {
	img := radial(64, 64, 32, 32)

	pngHash, ok := FromBytes(encodePNG(t, img))
	if !ok {
		t.Fatal("FromBytes(PNG): ok=false")
	}
	jpgHash, ok := FromBytes(encodeJPEG(t, img, 90))
	if !ok {
		t.Fatal("FromBytes(JPEG): ok=false")
	}

	if d := Distance(pngHash, jpgHash); d > 8 {
		t.Errorf("same image, different encodings: distance = %d, want <= 8", d)
	}
}

// Visually different images must be far apart, and clearly farther than the same
// image across encodings.
func TestDistance_DifferentImagesAreFarApart(t *testing.T) {
	a := FromImage(radial(64, 64, 32, 32)) // centred
	b := FromImage(radial(64, 64, 0, 0))   // corner

	same, _ := FromBytes(encodeJPEG(t, radial(64, 64, 32, 32), 90))

	d := Distance(a, b)
	if d < 12 {
		t.Errorf("different images: distance = %d, want >= 12", d)
	}
	if d <= Distance(a, same) {
		t.Errorf("different images (%d) not farther than re-encoded same image (%d)", d, Distance(a, same))
	}
}

func TestDistance_SymmetricAndZeroForEqual(t *testing.T) {
	a := FromImage(radial(64, 64, 20, 40))
	b := FromImage(radial(64, 64, 40, 20))

	if Distance(a, a) != 0 {
		t.Errorf("Distance(a, a) = %d, want 0", Distance(a, a))
	}
	if Distance(a, b) != Distance(b, a) {
		t.Errorf("Distance not symmetric: %d vs %d", Distance(a, b), Distance(b, a))
	}
}

func TestFromBytes_RejectsNonImage(t *testing.T) {
	if _, ok := FromBytes([]byte("definitely not an image")); ok {
		t.Error("FromBytes on garbage: ok=true, want false")
	}
}
