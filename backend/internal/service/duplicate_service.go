package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

// Merge field source values.
const (
	mergeKeep    = "keep"
	mergeDiscard = "discard"
	mergeBoth    = "both"
	mergeMerge   = "merge"
)

// MergeFields chooses, per field, which file supplies the survivor's value when
// resolving a duplicate. Scalars accept "keep"/"discard"; metadata also accepts
// "merge" (shallow object merge, survivor wins on key conflicts); relations
// (tags, pools) accept "keep"/"both" (union) — there is deliberately no option
// to drop the survivor's own tags/pools. An empty value defaults to "keep".
type MergeFields struct {
	OriginalName    string `json:"original_name"`
	Notes           string `json:"notes"`
	ContentDatetime string `json:"content_datetime"`
	IsPublic        string `json:"is_public"`
	Metadata        string `json:"metadata"`
	Tags            string `json:"tags"`
	Pools           string `json:"pools"`
}

// MergeSpec is the input to a duplicate resolution: keep one file, fold chosen
// fields in from the other, and (usually) trash the other.
type MergeSpec struct {
	Keep            uuid.UUID
	Discard         uuid.UUID
	Fields          MergeFields
	DeleteDiscarded bool
}

// normalize fills empty choices with "keep" and rejects unknown values.
func (m *MergeSpec) normalize() error {
	scalar := func(v *string) error {
		if *v == "" {
			*v = mergeKeep
		}
		if *v != mergeKeep && *v != mergeDiscard {
			return domain.ErrValidation
		}
		return nil
	}
	relation := func(v *string) error {
		if *v == "" {
			*v = mergeKeep
		}
		if *v != mergeKeep && *v != mergeBoth {
			return domain.ErrValidation
		}
		return nil
	}
	f := &m.Fields
	if err := scalar(&f.OriginalName); err != nil {
		return err
	}
	if err := scalar(&f.Notes); err != nil {
		return err
	}
	if err := scalar(&f.ContentDatetime); err != nil {
		return err
	}
	if err := scalar(&f.IsPublic); err != nil {
		return err
	}
	if f.Metadata == "" {
		f.Metadata = mergeKeep
	}
	if f.Metadata != mergeKeep && f.Metadata != mergeDiscard && f.Metadata != mergeMerge {
		return domain.ErrValidation
	}
	if err := relation(&f.Tags); err != nil {
		return err
	}
	if err := relation(&f.Pools); err != nil {
		return err
	}
	return nil
}

// DuplicateService finds near-duplicate clusters and resolves them.
type DuplicateService struct {
	files      port.FileRepo
	pairs      port.DuplicatePairRepo
	dismissals port.DismissalRepo
	acl        *ACLService
	audit      *AuditService
	tx         port.Transactor
	threshold  int
}

// NewDuplicateService creates a DuplicateService. threshold is the maximum
// Hamming distance for two files to be treated as duplicate candidates.
func NewDuplicateService(
	files port.FileRepo,
	pairs port.DuplicatePairRepo,
	dismissals port.DismissalRepo,
	acl *ACLService,
	audit *AuditService,
	tx port.Transactor,
	threshold int,
) *DuplicateService {
	return &DuplicateService{
		files:      files,
		pairs:      pairs,
		dismissals: dismissals,
		acl:        acl,
		audit:      audit,
		tx:         tx,
		threshold:  threshold,
	}
}

// Cluster is a group of near-duplicate files together with the pairwise Hamming
// distances known between them. Distances are read from the stored pairs, so two
// files linked into the cluster only transitively (through an intermediate) may
// have no direct distance — that pair is simply omitted.
type Cluster struct {
	Files     []domain.File
	Distances []PairDistance
}

// PairDistance is the stored Hamming distance between two files of a cluster.
type PairDistance struct {
	A        uuid.UUID
	B        uuid.UUID
	Distance int
}

// Clusters returns a page of duplicate clusters visible to the caller. Pairs are
// read from the precomputed table (no all-pairs scan here) and grouped into
// connected components; pagination is over whole clusters. Each cluster carries
// the stored pairwise distances so callers can show how close the files are.
func (s *DuplicateService) Clusters(ctx context.Context, limit, offset int) (clusters []Cluster, total int, err error) {
	userID, isAdmin, _ := domain.UserFromContext(ctx)

	pairs, err := s.pairs.ListVisible(ctx, userID, isAdmin)
	if err != nil {
		return nil, 0, err
	}
	groups := clusterPairs(pairs)
	total = len(groups)

	if offset < 0 {
		offset = 0
	}
	if offset >= len(groups) {
		return []Cluster{}, total, nil
	}
	end := offset + limit
	if end > len(groups) || limit <= 0 {
		end = len(groups)
	}

	// Index the stored distances once; each page cluster looks up its own pairs.
	distByPair := make(map[[2]uuid.UUID]int, len(pairs))
	for _, p := range pairs {
		distByPair[orderedPair(p.FileA, p.FileB)] = p.Distance
	}

	out := make([]Cluster, 0, end-offset)
	for _, ids := range groups[offset:end] {
		files := make([]domain.File, 0, len(ids))
		for _, id := range ids {
			f, err := s.files.GetByID(ctx, id)
			if err != nil {
				// A file deleted between the pair read and now just drops out.
				if errors.Is(err, domain.ErrNotFound) {
					continue
				}
				return nil, 0, err
			}
			files = append(files, *f)
		}
		if len(files) >= 2 {
			out = append(out, Cluster{Files: files, Distances: clusterDistances(files, distByPair)})
		}
	}
	return out, total, nil
}

// Rescan recomputes the entire duplicate_pairs table from the current set of
// perceptual hashes. It is the only thing that populates the table, so the
// duplicates view reflects state as of the last rescan. Called by the dedup CLI.
func (s *DuplicateService) Rescan(ctx context.Context, onProgress func(done, total int)) error {
	entries, err := s.files.ListAllPHashes(ctx)
	if err != nil {
		return err
	}
	pairs := buildPairs(entries, s.threshold, onProgress)
	return s.pairs.ReplaceAll(ctx, pairs)
}

// Dismiss records two files as "not a duplicate" so the pair stops surfacing.
// The caller must be able to view both files.
func (s *DuplicateService) Dismiss(ctx context.Context, a, b uuid.UUID) error {
	if a == b {
		return domain.ErrValidation
	}
	userID, isAdmin, _ := domain.UserFromContext(ctx)
	for _, id := range []uuid.UUID{a, b} {
		f, err := s.files.GetByID(ctx, id)
		if err != nil {
			return err
		}
		ok, err := s.acl.CanView(ctx, userID, isAdmin, f.CreatorID, f.IsPublic, fileObjectTypeID, id)
		if err != nil {
			return err
		}
		if !ok {
			return domain.ErrForbidden
		}
	}
	if err := s.dismissals.Add(ctx, a, b, userID); err != nil {
		return err
	}
	objType := fileObjectType
	_ = s.audit.Log(ctx, "duplicate_dismiss", &objType, &a, map[string]any{"other": b.String()})
	return nil
}

// Resolve merges a duplicate pair: the survivor (keep) takes the chosen fields
// from the other (discard), and the other is trashed when DeleteDiscarded is set.
// The caller must be able to edit both files. Returns the updated survivor.
func (s *DuplicateService) Resolve(ctx context.Context, spec MergeSpec) (*domain.File, error) {
	if spec.Keep == spec.Discard {
		return nil, domain.ErrValidation
	}
	if err := spec.normalize(); err != nil {
		return nil, err
	}

	keep, err := s.files.GetByID(ctx, spec.Keep)
	if err != nil {
		return nil, err
	}
	discard, err := s.files.GetByID(ctx, spec.Discard)
	if err != nil {
		return nil, err
	}

	userID, isAdmin, _ := domain.UserFromContext(ctx)
	for _, f := range []*domain.File{keep, discard} {
		ok, err := s.acl.CanEdit(ctx, userID, isAdmin, f.CreatorID, fileObjectTypeID, f.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, domain.ErrForbidden
		}
	}

	// FileRepo.Update rewrites all editable scalar columns, so build the complete
	// resolved set (each field from keep or discard) rather than a sparse patch.
	patch := &domain.File{
		OriginalName:    pickPtr(spec.Fields.OriginalName, keep.OriginalName, discard.OriginalName),
		Notes:           pickPtr(spec.Fields.Notes, keep.Notes, discard.Notes),
		ContentDatetime: pickTime(spec.Fields.ContentDatetime, keep.ContentDatetime, discard.ContentDatetime),
		IsPublic:        pickBool(spec.Fields.IsPublic, keep.IsPublic, discard.IsPublic),
		Metadata:        pickMetadata(spec.Fields.Metadata, keep.Metadata, discard.Metadata),
	}

	var result *domain.File
	txErr := s.tx.WithTx(ctx, func(ctx context.Context) error {
		updated, err := s.files.Update(ctx, keep.ID, patch)
		if err != nil {
			return err
		}

		if spec.Fields.Tags == mergeBoth {
			if err := s.files.SetTags(ctx, keep.ID, unionTagIDs(keep.Tags, discard.Tags)); err != nil {
				return err
			}
			tags, err := s.files.ListTags(ctx, keep.ID)
			if err != nil {
				return err
			}
			updated.Tags = tags
		}
		if spec.Fields.Pools == mergeBoth {
			if err := s.files.CopyPoolMemberships(ctx, keep.ID, discard.ID); err != nil {
				return err
			}
		}
		if spec.DeleteDiscarded {
			if err := s.files.SoftDelete(ctx, discard.ID); err != nil {
				return err
			}
		}
		result = updated
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	objType := fileObjectType
	_ = s.audit.Log(ctx, "file_merge", &objType, &keep.ID, map[string]any{
		"discard":           spec.Discard.String(),
		"fields":            spec.Fields,
		"deleted_discarded": spec.DeleteDiscarded,
	})
	return result, nil
}

// --- field pickers ---------------------------------------------------------

func pickPtr(choice string, keep, discard *string) *string {
	if choice == mergeDiscard {
		return discard
	}
	return keep
}

func pickBool(choice string, keep, discard bool) bool {
	if choice == mergeDiscard {
		return discard
	}
	return keep
}

func pickTime(choice string, keep, discard time.Time) time.Time {
	if choice == mergeDiscard {
		return discard
	}
	return keep
}

func unionTagIDs(a, b []domain.Tag) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(a)+len(b))
	ids := make([]uuid.UUID, 0, len(a)+len(b))
	for _, t := range append(append([]domain.Tag{}, a...), b...) {
		if !seen[t.ID] {
			seen[t.ID] = true
			ids = append(ids, t.ID)
		}
	}
	return ids
}

// pickMetadata returns keep's metadata, discard's, or a shallow merge in which
// the survivor's keys win on conflict.
func pickMetadata(choice string, keep, discard json.RawMessage) json.RawMessage {
	switch choice {
	case mergeDiscard:
		return discard
	case mergeMerge:
		km := map[string]json.RawMessage{}
		dm := map[string]json.RawMessage{}
		_ = json.Unmarshal(keep, &km)
		_ = json.Unmarshal(discard, &dm)
		out := make(map[string]json.RawMessage, len(km)+len(dm))
		for k, v := range dm {
			out[k] = v
		}
		for k, v := range km { // survivor wins
			out[k] = v
		}
		if len(out) == 0 {
			return keep
		}
		b, err := json.Marshal(out)
		if err != nil {
			return keep
		}
		return b
	default:
		return keep
	}
}
