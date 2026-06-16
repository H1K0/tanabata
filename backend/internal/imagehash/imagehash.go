// Package imagehash computes a 64-bit perceptual hash (dHash) of an image and
// compares two hashes by Hamming distance. It is used for near-duplicate
// detection: visually similar images (re-encoded, resized, recompressed) produce
// hashes a small distance apart, while unrelated images are far apart.
//
// dHash is chosen for its robustness and simplicity: the image is reduced to a
// 9×8 grayscale and each pixel is compared to its right-hand neighbour, yielding
// 64 gradient-direction bits. It tolerates scaling and brightness/contrast
// changes well, which is exactly what re-encoded duplicates exhibit.
package imagehash

import (
	"bytes"
	"image"
	_ "image/gif"  // register GIF decoder
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
	"math/bits"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp" // register WebP decoder
)

// hashWidth/hashHeight define the reduced grayscale used for dHash. The extra
// column (width = height+1) provides the right-hand neighbour for the 64
// horizontal comparisons that make up the hash.
const (
	hashHeight = 8
	hashWidth  = hashHeight + 1
)

// FromImage reduces img to a 9×8 grayscale and returns its 64-bit dHash. The
// uint64 of gradient bits is returned as int64 (a plain bit reinterpretation) so
// it fits PostgreSQL's bigint; equality and Distance are bitwise, so the signed
// interpretation never matters.
func FromImage(img image.Image) int64 {
	small := imaging.Grayscale(imaging.Resize(img, hashWidth, hashHeight, imaging.Lanczos))

	var hash uint64
	bit := 0
	for y := 0; y < hashHeight; y++ {
		for x := 0; x < hashHeight; x++ {
			// After Grayscale, R == G == B, so the red channel is the luminance.
			left := small.Pix[small.PixOffset(x, y)]
			right := small.Pix[small.PixOffset(x+1, y)]
			if left < right {
				hash |= 1 << uint(63-bit)
			}
			bit++
		}
	}
	return int64(hash)
}

// FromBytes decodes data (JPEG/PNG/GIF/WebP) and returns its dHash. ok is false
// when the bytes are not a decodable image, so callers can simply skip hashing
// (e.g. leave phash NULL) rather than fail.
func FromBytes(data []byte) (hash int64, ok bool) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, false
	}
	return FromImage(img), true
}

// Distance returns the Hamming distance (0–64) between two hashes: the number of
// differing bits. 0 means identical; small values mean near-duplicate.
func Distance(a, b int64) int {
	return bits.OnesCount64(uint64(a) ^ uint64(b))
}
