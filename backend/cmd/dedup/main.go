// Command dedup is the offline maintenance tool for duplicate detection. It runs
// in two phases:
//
//	hashes — compute the perceptual hash of every live image/video that has none
//	         yet (images from their bytes, videos from a middle frame via ffmpeg).
//	pairs  — rebuild data.duplicate_pairs from all current hashes.
//
// Both phases run by default; pass -hashes or -pairs to run only one. It reuses
// the server's configuration (DATABASE_URL, FILES_PATH, THUMBS_CACHE_PATH, …) and
// is safe to re-run: hashing only touches files whose phash is NULL, and the
// pairs rebuild is a full replace.
//
//	go run ./cmd/dedup            # hashes, then pairs
//	go run ./cmd/dedup -pairs      # only rebuild pairs
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/uuid"

	"tanabata/backend/internal/config"
	"tanabata/backend/internal/db/postgres"
	"tanabata/backend/internal/imagehash"
	"tanabata/backend/internal/service"
	"tanabata/backend/internal/storage"
)

func main() {
	hashesOnly := flag.Bool("hashes", false, "only (re)compute missing perceptual hashes")
	pairsOnly := flag.Bool("pairs", false, "only rebuild the duplicate pairs table")
	flag.Parse()

	// No flag, or both, means run everything.
	doHashes := *hashesOnly || !*pairsOnly
	doPairs := *pairsOnly || !*hashesOnly

	cfg, err := config.Load()
	if err != nil {
		fatal("load config", err)
	}

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("connect to database", err)
	}
	defer pool.Close()

	diskStorage, err := storage.NewDiskStorage(
		cfg.FilesPath, cfg.ThumbsCachePath,
		cfg.ThumbWidth, cfg.ThumbHeight,
		cfg.PreviewWidth, cfg.PreviewHeight,
		cfg.ThumbMaxPixels, cfg.ThumbConcurrency,
	)
	if err != nil {
		fatal("init storage", err)
	}

	fileRepo := postgres.NewFileRepo(pool)
	pairRepo := postgres.NewDuplicatePairRepo(pool)
	dismissalRepo := postgres.NewDismissalRepo(pool)
	aclRepo := postgres.NewACLRepo(pool)
	auditRepo := postgres.NewAuditRepo(pool)
	tagRepo := postgres.NewTagRepo(pool)
	categoryRepo := postgres.NewCategoryRepo(pool)
	poolRepo := postgres.NewPoolRepo(pool)
	transactor := postgres.NewTransactor(pool)

	aclSvc := service.NewACLService(aclRepo, fileRepo, tagRepo, categoryRepo, poolRepo, transactor)
	auditSvc := service.NewAuditService(auditRepo)
	dupSvc := service.NewDuplicateService(
		fileRepo, pairRepo, dismissalRepo, aclSvc, auditSvc, transactor, cfg.DuplicateHashThreshold,
	)

	if doHashes {
		if err := backfillHashes(ctx, fileRepo, diskStorage); err != nil {
			fatal("backfill hashes", err)
		}
	}
	if doPairs {
		fmt.Printf("rebuilding duplicate pairs (threshold %d)...\n", cfg.DuplicateHashThreshold)
		// total is only known once Rescan has loaded the hashes, so create the bar
		// lazily on the first progress callback.
		var prog *progress
		if err := dupSvc.Rescan(ctx, func(done, total int) {
			if prog == nil {
				prog = newProgress("matching", total)
			}
			prog.set(done)
		}); err != nil {
			fatal("rescan pairs", err)
		}
		if prog != nil {
			prog.finish()
		}
		fmt.Println("  done")
	}
}

// backfillHashes computes and stores a perceptual hash for every live image/video
// that lacks one. Failures on individual files are counted and reported, not
// fatal, so one unreadable file doesn't abort the whole run.
func backfillHashes(ctx context.Context, files *postgres.FileRepo, store *storage.DiskStorage) error {
	pending, err := files.ListMissingPHash(ctx)
	if err != nil {
		return err
	}
	total := len(pending)
	fmt.Printf("hashing %d files without a perceptual hash...\n", total)

	var hashed, skipped, failed int
	prog := newProgress("hashing", total)
	for i, f := range pending {
		ph, err := hashOne(ctx, store, f.ID, f.MIMEType)
		switch {
		case err != nil:
			failed++
			fmt.Fprintf(os.Stderr, "\n  %s (%s): %v\n", f.ID, f.MIMEType, err)
		case ph == nil:
			skipped++ // not decodable; leave phash NULL
		default:
			if err := files.SetPHash(ctx, f.ID, ph); err != nil {
				return fmt.Errorf("set phash for %s: %w", f.ID, err)
			}
			hashed++
		}
		prog.set(i + 1)
	}
	prog.finish()
	fmt.Printf("  hashed %d, skipped %d, failed %d\n", hashed, skipped, failed)
	return nil
}

// hashOne returns the perceptual hash for one file, or nil when it isn't hashable
// (e.g. an image that won't decode). Images are hashed from their bytes; videos
// from a middle frame.
func hashOne(ctx context.Context, store *storage.DiskStorage, id uuid.UUID, mime string) (*int64, error) {
	switch {
	case strings.HasPrefix(mime, "image/"):
		rc, err := store.Read(ctx, id)
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, err
		}
		if h, ok := imagehash.FromBytes(data); ok {
			return &h, nil
		}
		return nil, nil
	case strings.HasPrefix(mime, "video/"):
		img, err := store.VideoFrameMiddle(ctx, id)
		if err != nil {
			return nil, err
		}
		h := imagehash.FromImage(img)
		return &h, nil
	default:
		return nil, nil
	}
}

// progress renders a dependency-free progress indicator. On a TTY it draws an
// in-place bar; otherwise (pipe, cron, CI) it prints a line every 10% so logs
// stay readable instead of filling with carriage returns.
type progress struct {
	label   string
	tty     bool
	total   int
	lastDec int // last 10%-decile printed in non-TTY mode
}

func newProgress(label string, total int) *progress {
	fi, _ := os.Stdout.Stat()
	tty := fi != nil && fi.Mode()&os.ModeCharDevice != 0
	return &progress{label: label, tty: tty, total: total, lastDec: -1}
}

func (p *progress) set(done int) {
	if p.total <= 0 {
		return
	}
	pct := done * 100 / p.total
	if p.tty {
		const w = 30
		filled := done * w / p.total
		fmt.Printf("\r  %s [%s%s] %3d%% (%d/%d)",
			p.label,
			strings.Repeat("#", filled), strings.Repeat("-", w-filled),
			pct, done, p.total)
		return
	}
	if dec := pct / 10; dec != p.lastDec {
		p.lastDec = dec
		fmt.Printf("  %s %d%% (%d/%d)\n", p.label, pct, done, p.total)
	}
}

// finish ends the in-place bar with a newline (TTY only).
func (p *progress) finish() {
	if p.tty && p.total > 0 {
		fmt.Println()
	}
}

func fatal(what string, err error) {
	fmt.Fprintf(os.Stderr, "dedup: %s: %v\n", what, err)
	os.Exit(1)
}
