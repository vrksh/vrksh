package tokcount

import (
	"container/heap"
	"math"
)

// maxPieceBytes bounds how much of one piece is merged at once. The merge
// needs ~60 bytes of state per input byte, so one 20 MB whitespace run used
// to cost 1.4 GB. Real text never splits into pieces this long.
const maxPieceBytes = 4096

// encodePiece is bytePairEncode over chunks of at most maxPieceBytes.
// ponytail: a token can't straddle a chunk boundary, so a pathological piece
// over 4 KB may count a few tokens differently from tiktoken; the ids still
// decode to the exact input. Uniform runs (spaces, newlines, one letter)
// match exactly because 4096 is a multiple of their token lengths.
func encodePiece(piece []byte, ranks map[string]int) []int {
	if len(piece) <= maxPieceBytes {
		return bytePairEncode(piece, ranks)
	}
	var out []int
	for len(piece) > 0 {
		n := min(len(piece), maxPieceBytes)
		out = append(out, bytePairEncode(piece[:n], ranks)...)
		piece = piece[n:]
	}
	return out
}

// bytePairEncode splits a piece that is not itself a token into tokens by
// repeatedly merging the adjacent pair with the lowest rank, leftmost on a
// tie. That is the merge order of tiktoken and tiktoken-go, so the output is
// identical (TestMatchesTiktokenGo checks it).
//
// tiktoken-go rescans every pair after each merge, which is O(n^2) per piece
// and took ~30 s on 200 KB of one repeated character. Here the candidate pairs
// sit in a min-heap ordered by (rank, start offset); entries made stale by a
// merge are skipped when popped. O(n log n).
func bytePairEncode(piece []byte, ranks map[string]int) []int {
	if len(piece) == 1 {
		return []int{ranks[string(piece)]}
	}

	n := len(piece)
	// Parts form a linked list over byte offsets: part i covers
	// piece[i:next[i]]. Removed parts are skipped via next/prev.
	next := make([]int, n)
	prev := make([]int, n)
	rank := make([]int, n) // rank of merging part i with the part after it
	for i := range n {
		next[i], prev[i] = i+1, i-1
	}
	pairRank := func(i int) int {
		if i < 0 || next[i] >= n {
			return math.MaxInt
		}
		if r, ok := ranks[string(piece[i:next[next[i]]])]; ok {
			return r
		}
		return math.MaxInt
	}
	// next[n-1] == n, so next[next[i]] needs next[n]; extend by one.
	next = append(next, n)

	h := make(pairHeap, 0, n)
	for i := range n {
		rank[i] = pairRank(i)
		if rank[i] != math.MaxInt {
			h = append(h, pair{rank[i], i})
		}
	}
	heap.Init(&h)

	for h.Len() > 0 {
		p := heap.Pop(&h).(pair)
		i := p.start
		if rank[i] != p.rank {
			continue // stale: part i was merged away or its neighbour changed
		}
		// Merge part i with the part after it.
		j := next[i]
		rank[j] = math.MaxInt // j no longer starts a part
		next[i] = next[j]
		if next[i] < n {
			prev[next[i]] = i
		}
		rank[i] = pairRank(i)
		if rank[i] != math.MaxInt {
			heap.Push(&h, pair{rank[i], i})
		}
		if k := prev[i]; k >= 0 {
			rank[k] = pairRank(k)
			if rank[k] != math.MaxInt {
				heap.Push(&h, pair{rank[k], k})
			}
		}
	}

	var out []int
	for i := 0; i < n; i = next[i] {
		out = append(out, ranks[string(piece[i:next[i]])])
	}
	return out
}

type pair struct{ rank, start int }

type pairHeap []pair

func (h pairHeap) Len() int { return len(h) }
func (h pairHeap) Less(a, b int) bool {
	if h[a].rank != h[b].rank {
		return h[a].rank < h[b].rank
	}
	return h[a].start < h[b].start
}
func (h pairHeap) Swap(a, b int) { h[a], h[b] = h[b], h[a] }
func (h *pairHeap) Push(x any)   { *h = append(*h, x.(pair)) }
func (h *pairHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}
