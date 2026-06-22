package service

import (
	"bytes"
	"math/bits"
	"sort"

	"github.com/google/uuid"

	"tanabata/backend/internal/domain"
)

// hamming returns the number of differing bits between two perceptual hashes.
func hamming(a, b uint64) int { return bits.OnesCount64(a ^ b) }

// bkNode is a node in a BK-tree over Hamming distance. Files that share the exact
// same hash are collected in ids (a distance-0 collision), so identical images
// don't degenerate the tree into a chain.
type bkNode struct {
	hash     uint64
	ids      []uuid.UUID
	children map[int]*bkNode
}

// bkTree indexes perceptual hashes for sublinear radius queries. Building one and
// querying every element with a small radius is far cheaper than the O(N²) all-
// pairs comparison at 100k+ files.
type bkTree struct{ root *bkNode }

func (t *bkTree) insert(hash uint64, id uuid.UUID) {
	if t.root == nil {
		t.root = &bkNode{hash: hash, ids: []uuid.UUID{id}, children: map[int]*bkNode{}}
		return
	}
	node := t.root
	for {
		d := hamming(hash, node.hash)
		if d == 0 {
			node.ids = append(node.ids, id)
			return
		}
		child, ok := node.children[d]
		if !ok {
			node.children[d] = &bkNode{hash: hash, ids: []uuid.UUID{id}, children: map[int]*bkNode{}}
			return
		}
		node = child
	}
}

// query visits every node whose hash is within radius of target. The triangle
// inequality bounds which children can hold a match to [d-radius, d+radius].
func (t *bkTree) query(target uint64, radius int, visit func(node *bkNode, dist int)) {
	if t.root == nil {
		return
	}
	stack := []*bkNode{t.root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		d := hamming(target, node.hash)
		if d <= radius {
			visit(node, d)
		}
		lo, hi := d-radius, d+radius
		for cd, child := range node.children {
			if cd >= lo && cd <= hi {
				stack = append(stack, child)
			}
		}
	}
}

// buildPairs returns every unordered pair of files whose hashes are within
// threshold, each emitted exactly once with FileA < FileB (UUID byte order).
// onProgress, if set, is called periodically with (processed, total).
func buildPairs(entries []domain.PHashEntry, threshold int, onProgress func(done, total int)) []domain.DuplicatePair {
	tree := &bkTree{}
	for _, e := range entries {
		tree.insert(uint64(e.PHash), e.ID)
	}

	var pairs []domain.DuplicatePair
	total := len(entries)
	for i := range entries {
		e := entries[i]
		tree.query(uint64(e.PHash), threshold, func(node *bkNode, dist int) {
			for _, other := range node.ids {
				// Emit each pair once, from the smaller id, which also skips self.
				if bytes.Compare(e.ID[:], other[:]) < 0 {
					pairs = append(pairs, domain.DuplicatePair{FileA: e.ID, FileB: other, Distance: dist})
				}
			}
		})
		if onProgress != nil && (i+1)%1000 == 0 {
			onProgress(i+1, total)
		}
	}
	if onProgress != nil {
		onProgress(total, total)
	}
	return pairs
}

// orderedPair returns the two ids in canonical (a < b by UUID byte order) order,
// matching how the pairs table keys a distance so a lookup hits regardless of the
// argument order.
func orderedPair(a, b uuid.UUID) [2]uuid.UUID {
	if bytes.Compare(a[:], b[:]) > 0 {
		return [2]uuid.UUID{b, a}
	}
	return [2]uuid.UUID{a, b}
}

// clusterDistances returns the stored Hamming distance for every pair of files in
// the cluster that has one. Pairs present only transitively have no stored
// distance and are left out.
func clusterDistances(files []domain.File, distByPair map[[2]uuid.UUID]int) []PairDistance {
	var out []PairDistance
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if d, ok := distByPair[orderedPair(files[i].ID, files[j].ID)]; ok {
				out = append(out, PairDistance{A: files[i].ID, B: files[j].ID, Distance: d})
			}
		}
	}
	return out
}

// clusterPairs groups pairs into connected components (transitive closure) via
// union-find. Every returned cluster has at least two files; clusters and the ids
// within them are sorted by UUID for stable pagination.
func clusterPairs(pairs []domain.DuplicatePair) [][]uuid.UUID {
	parent := map[uuid.UUID]uuid.UUID{}
	var find func(uuid.UUID) uuid.UUID
	find = func(x uuid.UUID) uuid.UUID {
		p, ok := parent[x]
		if !ok {
			parent[x] = x
			return x
		}
		if p != x {
			parent[x] = find(p)
		}
		return parent[x]
	}
	union := func(a, b uuid.UUID) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	for _, p := range pairs {
		union(p.FileA, p.FileB)
	}

	groups := map[uuid.UUID][]uuid.UUID{}
	for node := range parent {
		root := find(node)
		groups[root] = append(groups[root], node)
	}

	clusters := make([][]uuid.UUID, 0, len(groups))
	for _, ids := range groups {
		sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
		clusters = append(clusters, ids)
	}
	sort.Slice(clusters, func(i, j int) bool {
		return bytes.Compare(clusters[i][0][:], clusters[j][0][:]) < 0
	})
	return clusters
}
