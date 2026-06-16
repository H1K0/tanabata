package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
)

// id builds a deterministic UUID whose byte order matches n, so tests can reason
// about the canonical (FileA < FileB) ordering buildPairs produces.
func id(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012d", n))
}

func entry(n int, hash uint64) domain.PHashEntry {
	return domain.PHashEntry{ID: id(n), PHash: int64(hash)}
}

// pairKey canonicalises a pair for set comparison regardless of emission order.
func pairKey(p domain.DuplicatePair) string {
	a, b := p.FileA, p.FileB
	if bytes.Compare(a[:], b[:]) > 0 {
		a, b = b, a
	}
	return fmt.Sprintf("%s|%s|%d", a, b, p.Distance)
}

func TestBuildPairs_ThresholdAndCanonicalOrder(t *testing.T) {
	entries := []domain.PHashEntry{
		entry(1, 0x0000000000000000),
		entry(2, 0x0000000000000001), // distance 1 from #1
		entry(3, 0x00000000000000FF), // distance 8 from #1, 7 from #2
		entry(4, 0xFFFFFFFFFFFFFFFF), // distance 64 from #1
	}

	// Tight threshold: only the distance-1 pair qualifies.
	got := buildPairs(entries, 2, nil)
	if len(got) != 1 {
		t.Fatalf("threshold 2: got %d pairs, want 1: %+v", len(got), got)
	}
	if got[0].FileA != id(1) || got[0].FileB != id(2) || got[0].Distance != 1 {
		t.Errorf("threshold 2: unexpected pair %+v", got[0])
	}
	// Canonical order always FileA < FileB.
	if bytes.Compare(got[0].FileA[:], got[0].FileB[:]) >= 0 {
		t.Error("pair not in canonical FileA < FileB order")
	}

	// Looser threshold pulls in #3's pairs but never #4.
	got8 := buildPairs(entries, 8, nil)
	want := map[string]bool{
		pairKey(domain.DuplicatePair{FileA: id(1), FileB: id(2), Distance: 1}): true,
		pairKey(domain.DuplicatePair{FileA: id(1), FileB: id(3), Distance: 8}): true,
		pairKey(domain.DuplicatePair{FileA: id(2), FileB: id(3), Distance: 7}): true,
	}
	if len(got8) != len(want) {
		t.Fatalf("threshold 8: got %d pairs, want %d: %+v", len(got8), len(want), got8)
	}
	for _, p := range got8 {
		if !want[pairKey(p)] {
			t.Errorf("threshold 8: unexpected pair %+v", p)
		}
	}
}

func TestBuildPairs_IdenticalHashesPairAtDistanceZero(t *testing.T) {
	entries := []domain.PHashEntry{
		entry(1, 0xABCDABCDABCDABCD),
		entry(2, 0xABCDABCDABCDABCD),
	}
	got := buildPairs(entries, 0, nil)
	if len(got) != 1 || got[0].Distance != 0 || got[0].FileA != id(1) || got[0].FileB != id(2) {
		t.Fatalf("identical hashes: got %+v, want one distance-0 pair (1,2)", got)
	}
}

func TestClusterPairs_ConnectedComponents(t *testing.T) {
	pairs := []domain.DuplicatePair{
		{FileA: id(1), FileB: id(2)},
		{FileA: id(2), FileB: id(3)}, // transitively joins 1-2-3
		{FileA: id(5), FileB: id(6)},
	}
	clusters := clusterPairs(pairs)
	if len(clusters) != 2 {
		t.Fatalf("got %d clusters, want 2: %+v", len(clusters), clusters)
	}
	// Sorted by smallest id: {1,2,3} then {5,6}.
	if len(clusters[0]) != 3 || clusters[0][0] != id(1) || clusters[0][2] != id(3) {
		t.Errorf("cluster 0 = %v, want [1 2 3]", clusters[0])
	}
	if len(clusters[1]) != 2 || clusters[1][0] != id(5) {
		t.Errorf("cluster 1 = %v, want [5 6]", clusters[1])
	}
	// Each cluster's ids are sorted.
	for _, c := range clusters {
		if !sort.SliceIsSorted(c, func(i, j int) bool { return bytes.Compare(c[i][:], c[j][:]) < 0 }) {
			t.Errorf("cluster not sorted: %v", c)
		}
	}
}

func TestPickMetadata_Merge(t *testing.T) {
	keep := json.RawMessage(`{"a":1,"b":2}`)
	discard := json.RawMessage(`{"b":9,"c":3}`)

	out := pickMetadata(mergeMerge, keep, discard)
	var m map[string]int
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("merge result not valid JSON: %v (%s)", err, out)
	}
	want := map[string]int{"a": 1, "b": 2, "c": 3} // survivor wins on "b"
	if fmt.Sprint(m) != fmt.Sprint(want) {
		t.Errorf("merge = %v, want %v", m, want)
	}

	if string(pickMetadata(mergeKeep, keep, discard)) != string(keep) {
		t.Error("keep choice should return survivor metadata unchanged")
	}
	if string(pickMetadata(mergeDiscard, keep, discard)) != string(discard) {
		t.Error("discard choice should return the other file's metadata")
	}
}

func TestMergeSpec_Normalize(t *testing.T) {
	// Empty fields default to "keep".
	spec := MergeSpec{Keep: id(1), Discard: id(2)}
	if err := spec.normalize(); err != nil {
		t.Fatalf("normalize empty: %v", err)
	}
	if spec.Fields.OriginalName != mergeKeep || spec.Fields.Tags != mergeKeep || spec.Fields.Metadata != mergeKeep {
		t.Errorf("empty fields not defaulted to keep: %+v", spec.Fields)
	}

	// "both" is invalid for a scalar field.
	bad := MergeSpec{Keep: id(1), Discard: id(2), Fields: MergeFields{Notes: mergeBoth}}
	if err := bad.normalize(); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("scalar=both: got %v, want ErrValidation", err)
	}

	// "discard" is invalid for a relation field.
	badRel := MergeSpec{Keep: id(1), Discard: id(2), Fields: MergeFields{Tags: mergeDiscard}}
	if err := badRel.normalize(); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("relation=discard: got %v, want ErrValidation", err)
	}
}
