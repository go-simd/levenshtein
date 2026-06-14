// Package levenshtein computes the Levenshtein (edit) distance between two
// byte slices or strings using Myers' bit-parallel algorithm.
//
// The Levenshtein distance is the minimum number of single-character
// insertions, deletions, and substitutions required to transform one string
// into the other.
//
// # Algorithm
//
// Instead of the classic O(m*n) dynamic-programming (DP) recurrence, this
// package uses Myers' bit-parallel algorithm (G. Myers, "A fast bit-vector
// algorithm for approximate string matching based on dynamic programming",
// J. ACM 46(3), 1999), in the clean reformulation of Hyyrö. It packs an
// entire column of the DP matrix into machine words and advances it with a
// handful of bitwise operations, giving O(ceil(m/w)*n) time where w is the
// word size (64). For patterns up to 64 symbols this is a single-word routine
// that runs in O(n) word operations; longer patterns are processed in blocks
// of 64 rows ("bit-vector blocking").
//
// All operations work on bytes. To compare Unicode text by rune rather than by
// byte, decode to []rune (or use a rune-aware wrapper) before calling.
package levenshtein

// Distance returns the Levenshtein edit distance between a and b, comparing
// element by element (byte by byte). It is symmetric: Distance(a, b) ==
// Distance(b, a).
func Distance(a, b []byte) int {
	// Fast trivial cases.
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Myers packs the *shorter* string into the bit-vectors (the "pattern")
	// and streams the longer one (the "text"). This minimises the number of
	// 64-bit blocks and the work per text symbol.
	p, t := a, b
	if len(p) > len(t) {
		p, t = t, p
	}

	if len(p) <= 64 {
		return myers64(p, t)
	}
	return myersBlocked(p, t)
}

// DistanceString is the string-typed convenience wrapper around Distance. It
// compares the inputs byte by byte (UTF-8 code units), which matches Distance.
func DistanceString(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	return Distance([]byte(a), []byte(b))
}

// peqMap holds the per-symbol match (equality) bit-masks for the pattern.
// peqMap[c] has bit i set iff pattern[i] == c. ASCII (and any byte value) is
// covered directly by a 256-entry table; non-zero entries beyond ASCII are
// handled the same way since the key is a full byte.
type peqMap [256]uint64

// myers64 computes the edit distance for a pattern of length 1..64 using the
// single-word Myers/Hyyrö recurrence.
//
// Invariant maintained across the text: Pv/Mv are the bit-vectors of the
// vertical positive/negative deltas of the current DP column; score holds the
// distance value in the bottom (highest pattern index) cell.
func myers64(pattern, text []byte) int {
	m := len(pattern)

	var peq peqMap
	for i := 0; i < m; i++ {
		peq[pattern[i]] |= uint64(1) << uint(i)
	}

	// last is the bit selecting the bottom row of the column.
	last := uint64(1) << uint(m-1)

	// Initial column: distance from empty text prefix, i.e. all vertical
	// deltas are +1 (Pv = all ones over the m rows), score = m.
	pv := ^uint64(0)
	mv := uint64(0)
	score := m

	for _, c := range text {
		eq := peq[c]

		xv := eq | mv
		xh := (((eq & pv) + pv) ^ pv) | eq

		ph := mv | ^(xh | pv)
		mh := pv & xh

		// Update running score by the horizontal delta in the bottom cell.
		if ph&last != 0 {
			score++
		} else if mh&last != 0 {
			score--
		}

		// Shift horizontal deltas into the vertical deltas for next column.
		ph = (ph << 1) | 1
		mh <<= 1

		pv = mh | ^(xv | ph)
		mv = ph & xv
	}

	return score
}

// myersBlocked computes the edit distance for patterns longer than 64 symbols
// by tiling the DP column into blocks of 64 rows and carrying the horizontal
// deltas between adjacent blocks. This is the standard multi-word extension of
// Myers' algorithm (Hyyrö 2003, "A note on bit-parallel alignment computation").
//
// blocks is the number of 64-bit words covering the pattern. Each text symbol
// updates every block top-to-bottom, threading the carry (hp/hm) of the bottom
// horizontal delta of one block into the top of the next.
func myersBlocked(pattern, text []byte) int {
	m := len(pattern)
	blocks := (m + 63) / 64

	// Per-symbol equality masks, one word per block.
	// peq[c] is a slice of `blocks` words for byte value c, built lazily so we
	// only allocate rows for byte values that actually occur in the pattern.
	var peq [256][]uint64
	for i := 0; i < m; i++ {
		c := pattern[i]
		if peq[c] == nil {
			peq[c] = make([]uint64, blocks)
		}
		peq[c][i>>6] |= uint64(1) << uint(i&63)
	}
	// zero is used as the equality vector for byte values absent from the
	// pattern, avoiding allocating 256 full rows.
	zero := make([]uint64, blocks)

	// Per-block vertical delta vectors.
	pv := make([]uint64, blocks)
	mv := make([]uint64, blocks)
	for i := range pv {
		pv[i] = ^uint64(0)
	}

	// Bit selecting the bottom row of the final (possibly partial) block.
	lastBit := uint(m-1) & 63
	last := uint64(1) << lastBit

	score := m

	for _, c := range text {
		peqc := peq[c]
		if peqc == nil {
			peqc = zero
		}

		// Horizontal carry into the top of the first block: implicit +1
		// column boundary, identical to the single-word `(ph<<1)|1` seed.
		var hp, hm uint64 = 1, 0

		for b := 0; b < blocks; b++ {
			eq := peqc[b]
			pvb := pv[b]
			mvb := mv[b]

			// Fold the incoming horizontal carry into the equality vector
			// (a set carry-minus bit forces a match in row 0 of the block).
			eqIn := eq | hm

			xv := eqIn | mvb
			xh := (((eqIn & pvb) + pvb) ^ pvb) | eqIn

			ph := mvb | ^(xh | pvb)
			mh := pvb & xh

			// Track score using the bottom cell of the last block.
			if b == blocks-1 {
				if ph&last != 0 {
					score++
				} else if mh&last != 0 {
					score--
				}
			}

			// Extract this block's bottom horizontal delta as the carry for
			// the next block, then shift in the incoming carry at the top.
			outHp := (ph >> 63) & 1
			outHm := (mh >> 63) & 1

			ph = (ph << 1) | hp
			mh = (mh << 1) | hm

			pv[b] = mh | ^(xv | ph)
			mv[b] = ph & xv

			hp = outHp
			hm = outHm
		}
	}

	return score
}
