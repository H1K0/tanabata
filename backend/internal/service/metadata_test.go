package service

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"
)

func TestParseExifDate(t *testing.T) {
	cases := []struct {
		in   string
		ok   bool
		want time.Time
	}{
		{"2026:03:24 16:57:58", true, time.Date(2026, 3, 24, 16, 57, 58, 0, time.UTC)},
		{"2026:05:08 23:07:55+03:00", true, time.Date(2026, 5, 8, 23, 7, 55, 0, time.FixedZone("", 3*3600))},
		{"  2026:01:02 03:04:05  ", true, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)},
		{"0000:00:00 00:00:00", false, time.Time{}},
		{"not a date", false, time.Time{}},
		{"", false, time.Time{}},
	}
	for _, c := range cases {
		got, ok := parseExifDate(c.in)
		if ok != c.ok {
			t.Errorf("parseExifDate(%q) ok=%v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && !got.Equal(c.want) {
			t.Errorf("parseExifDate(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// tinyPNG returns a valid 2x3 PNG with no embedded EXIF/date.
func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.Set(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestExtractMetadataExiftool(t *testing.T) {
	if exiftoolPath == "" {
		t.Skip("exiftool not installed; metadata extraction falls back to goexif")
	}

	raw, dt := extractMetadata(tinyPNG(t), "snapshot.png", nil)
	if raw == nil {
		t.Fatal("expected non-nil metadata JSON")
	}
	if dt != nil {
		t.Errorf("a PNG without a capture date should yield no content datetime, got %v", dt)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("metadata is not valid JSON: %v", err)
	}

	// exiftool understood the format (goexif never would for PNG).
	if v := jsonString(t, m, "FileType"); v != "PNG" {
		t.Errorf("FileType = %q, want PNG", v)
	}

	// Dimensions are numeric, not human-readable strings.
	for _, key := range []string{"ImageWidth", "ImageHeight"} {
		raw, ok := m[key]
		if !ok {
			t.Errorf("missing %s", key)
			continue
		}
		var n float64
		if err := json.Unmarshal(raw, &n); err != nil {
			t.Errorf("%s is not numeric: %s", key, raw)
		}
	}

	// FileName is the original, not the temp file; temp-file artifacts are gone.
	if v := jsonString(t, m, "FileName"); v != "snapshot.png" {
		t.Errorf("FileName = %q, want snapshot.png", v)
	}
	for _, leaked := range []string{"SourceFile", "Directory", "FilePermissions", "FileModifyDate"} {
		if _, ok := m[leaked]; ok {
			t.Errorf("temp-file field %q should have been stripped", leaked)
		}
	}
}

func jsonString(t *testing.T, m map[string]json.RawMessage, key string) string {
	t.Helper()
	raw, ok := m[key]
	if !ok {
		t.Errorf("missing key %q", key)
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Errorf("key %q is not a string: %s", key, raw)
		return ""
	}
	return s
}
