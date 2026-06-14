package levenshtein

import (
	"math/rand"
	"testing"
)

func TestDistanceKnown(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"abc", "abc", 0},
		{"kitten", "sitting", 3},
		{"sitting", "kitten", 3},
		{"flaw", "lawn", 2},
		{"gumbo", "gambol", 2},
		{"book", "back", 2},
		{"a", "b", 1},
		{"ab", "ba", 2},
		{"distance", "difference", 5},
		{"levenshtein", "frankenstein", 6},
		{"resume and cafe", "resumes and cafes", 2},
	}
	for _, c := range cases {
		if got := DistanceString(c.a, c.b); got != c.want {
			t.Errorf("DistanceString(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
		if got := Distance([]byte(c.a), []byte(c.b)); got != c.want {
			t.Errorf("Distance(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
		// Symmetry.
		if got := DistanceString(c.b, c.a); got != c.want {
			t.Errorf("DistanceString(%q, %q) = %d, want %d (symmetry)", c.b, c.a, got, c.want)
		}
	}
}

// randBytes builds a slice of length n over an alphabet of `alpha` distinct
// byte values.
func randBytes(r *rand.Rand, n, alpha int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + r.Intn(alpha))
	}
	return b
}

// TestDistanceDifferential exhaustively compares the bit-parallel result with
// the scalar DP reference over a wide range of lengths (exercising the
// single-word path, multi-word blocks, and partial final blocks) and
// alphabets.
func TestDistanceDifferential(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	// Lengths chosen to straddle the 64-bit block boundaries.
	lengths := []int{0, 1, 2, 3, 7, 15, 31, 32, 33, 63, 64, 65, 100, 127, 128, 129, 200, 255, 256, 257, 300}
	alphabets := []int{1, 2, 4, 26}
	for _, alpha := range alphabets {
		for _, la := range lengths {
			for _, lb := range lengths {
				for trial := 0; trial < 3; trial++ {
					a := randBytes(r, la, alpha)
					b := randBytes(r, lb, alpha)
					want := referenceDistance(a, b)
					got := Distance(a, b)
					if got != want {
						t.Fatalf("Distance(len=%d,len=%d,alpha=%d) = %d, want %d\na=%q\nb=%q",
							la, lb, alpha, got, want, a, b)
					}
				}
			}
		}
	}
}

// TestDistanceLongIdentical and near-identical long strings drive the blocked
// path with mostly-matching content (carries that don't change the score).
func TestDistanceLongEdits(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	base := randBytes(r, 500, 4)
	for _, edits := range []int{0, 1, 5, 50, 200} {
		mod := append([]byte(nil), base...)
		for i := 0; i < edits; i++ {
			pos := r.Intn(len(mod))
			mod[pos] = byte('a' + r.Intn(4))
		}
		want := referenceDistance(base, mod)
		got := Distance(base, mod)
		if got != want {
			t.Fatalf("edits=%d: Distance = %d, want %d", edits, got, want)
		}
	}
}

func FuzzDistance(f *testing.F) {
	seeds := []struct{ a, b string }{
		{"", ""},
		{"a", ""},
		{"kitten", "sitting"},
		{"the quick brown fox", "the lazy dog"},
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "b"},
	}
	for _, s := range seeds {
		f.Add([]byte(s.a), []byte(s.b))
	}
	f.Fuzz(func(t *testing.T, a, b []byte) {
		want := referenceDistance(a, b)
		got := Distance(a, b)
		if got != want {
			t.Fatalf("Distance(%q, %q) = %d, want %d", a, b, got, want)
		}
		// Symmetry must always hold.
		if rev := Distance(b, a); rev != got {
			t.Fatalf("asymmetric: Distance(a,b)=%d Distance(b,a)=%d", got, rev)
		}
	})
}
