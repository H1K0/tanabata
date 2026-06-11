package service

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// exiftoolPath is resolved once at startup. When exiftool isn't installed we
// skip the subprocess and fall back to the pure-Go EXIF reader, so the server
// still runs (with thinner metadata) on hosts without it.
var exiftoolPath, _ = exec.LookPath("exiftool")

// metadataTimeout bounds a single exiftool invocation so a pathological file
// can't wedge an upload.
const metadataTimeout = 30 * time.Second

// metaTempFileKeys are exiftool fields that describe the temporary file we feed
// it rather than the content. Dropping them avoids leaking internal paths and
// recording the temp file's permissions/inode timestamps.
var metaTempFileKeys = []string{
	"SourceFile",
	"Directory",
	"FileAccessDate",
	"FileInodeChangeDate",
	"FilePermissions",
}

// metaDateKeys are the metadata fields, in priority order, holding the moment
// the content was actually captured/created — photos first, then video atoms.
var metaDateKeys = []string{
	"DateTimeOriginal",
	"CreateDate",
	"MediaCreateDate",
	"TrackCreateDate",
	"ModifyDate",
}

// extractMetadata returns rich metadata as JSON plus the best content datetime
// it can find. It prefers exiftool, which understands video, audio and every
// image format and emits machine-readable numeric values (the basis for later
// analytics); when exiftool is unavailable it falls back to the pure-Go EXIF
// reader, which only handles JPEG/TIFF.
//
// originalName supplies the extension exiftool uses for format detection and the
// FileName reported back. mtime, when set (e.g. a server-side import), is stamped
// onto the temp file so FileModifyDate reflects the real source.
func extractMetadata(data []byte, originalName string, mtime *time.Time) (json.RawMessage, *time.Time) {
	if exiftoolPath != "" {
		if raw, dt, ok := exiftoolExtract(data, originalName, mtime); ok {
			return raw, dt
		}
	}
	return extractEXIFWithDatetime(data)
}

// exiftoolExtract stages the bytes in a temp file and shells out to exiftool.
// It returns ok=false on any failure so the caller can fall back.
func exiftoolExtract(data []byte, originalName string, mtime *time.Time) (json.RawMessage, *time.Time, bool) {
	// exiftool reads a real file far more reliably than a pipe (it seeks freely,
	// e.g. to a trailing MP4 moov atom), so stage the bytes in a temp file whose
	// extension matches the original for accurate format detection.
	tmp, err := os.CreateTemp("", "tfm-meta-*"+filepath.Ext(originalName))
	if err != nil {
		return nil, nil, false
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, nil, false
	}
	if err := tmp.Close(); err != nil {
		return nil, nil, false
	}
	if mtime != nil {
		_ = os.Chtimes(tmpName, *mtime, *mtime)
	}

	ctx, cancel := context.WithTimeout(context.Background(), metadataTimeout)
	defer cancel()
	// -n forces raw numeric/machine values for every tag (no "3.53 Mbps" strings)
	// so the metadata is analytics-ready. -all extracts every tag. largefilesupport
	// handles multi-GB videos. Output is a one-element JSON array.
	out, err := exec.CommandContext(ctx, exiftoolPath,
		"-n", "-all", "-json", "-api", "largefilesupport=1", tmpName,
	).Output()
	if err != nil {
		return nil, nil, false
	}

	var arr []map[string]json.RawMessage
	if err := json.Unmarshal(out, &arr); err != nil || len(arr) == 0 {
		return nil, nil, false
	}
	m := arr[0]

	dt := pickMetaDatetime(m)

	// Strip temp-file artifacts and substitute the real name.
	for _, k := range metaTempFileKeys {
		delete(m, k)
	}
	if mtime == nil {
		// Without a real source mtime this is just the temp file's write time.
		delete(m, "FileModifyDate")
	}
	if originalName != "" {
		if nb, err := json.Marshal(originalName); err == nil {
			m["FileName"] = nb
		}
	} else {
		delete(m, "FileName")
	}

	raw, err := json.Marshal(m)
	if err != nil {
		return nil, nil, false
	}
	return raw, dt, true
}

// pickMetaDatetime returns the first parseable content date among metaDateKeys.
func pickMetaDatetime(m map[string]json.RawMessage) *time.Time {
	for _, key := range metaDateKeys {
		raw, ok := m[key]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			continue
		}
		if t, ok := parseExifDate(s); ok {
			return &t
		}
	}
	return nil
}

// parseExifDate parses exiftool's "YYYY:MM:DD HH:MM:SS" timestamps, with or
// without a trailing timezone offset. Zeroed placeholders ("0000:00:00 ...")
// fail to parse and are skipped by the caller.
func parseExifDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{
		"2006:01:02 15:04:05-07:00",
		"2006:01:02 15:04:05Z07:00",
		"2006:01:02 15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// extractEXIFWithDatetime is the pure-Go fallback used when exiftool is absent.
// It parses EXIF from raw bytes (JPEG/TIFF only), returning both the JSON
// representation and the DateTimeOriginal (if present). Both may be nil.
func extractEXIFWithDatetime(data []byte) (json.RawMessage, *time.Time) {
	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, nil
	}
	b, err := x.MarshalJSON()
	if err != nil {
		return nil, nil
	}
	var dt *time.Time
	if t, err := x.DateTime(); err == nil {
		dt = &t
	}
	return json.RawMessage(b), dt
}
