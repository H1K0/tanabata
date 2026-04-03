package migrations

import "embed"

// FS holds all goose migration SQL files, embedded at build time.
//
//go:embed *.sql
var FS embed.FS
