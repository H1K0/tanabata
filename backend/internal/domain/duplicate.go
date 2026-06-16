package domain

import "github.com/google/uuid"

// PHashEntry is a file's perceptual hash, the input to duplicate clustering.
type PHashEntry struct {
	ID    uuid.UUID
	PHash int64
}

// DuplicatePair is an unordered pair of files whose perceptual hashes are within
// the configured Hamming threshold. FileA < FileB by UUID byte order (canonical),
// so a pair is represented exactly once.
type DuplicatePair struct {
	FileA    uuid.UUID
	FileB    uuid.UUID
	Distance int
}
