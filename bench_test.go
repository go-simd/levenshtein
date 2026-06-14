package levenshtein

import (
	"math/rand"
	"testing"

	agnivade "github.com/agnivade/levenshtein"
)

func benchInputs(n, alpha int) (string, string) {
	r := rand.New(rand.NewSource(int64(n*1000 + alpha)))
	a := string(randBytes(r, n, alpha))
	bb := []byte(a)
	// Introduce ~10% edits so the strings differ realistically.
	for i := 0; i < n/10; i++ {
		bb[r.Intn(n)] = byte('a' + r.Intn(alpha))
	}
	return a, string(bb)
}

var sizes = []int{16, 64, 256, 1024}

func BenchmarkDistance(b *testing.B) {
	for _, n := range sizes {
		x, y := benchInputs(n, 26)
		bx, by := []byte(x), []byte(y)
		b.Run(sizeName(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = Distance(bx, by)
			}
		})
	}
}

func BenchmarkReferenceDP(b *testing.B) {
	for _, n := range sizes {
		x, y := benchInputs(n, 26)
		bx, by := []byte(x), []byte(y)
		b.Run(sizeName(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = referenceDistance(bx, by)
			}
		})
	}
}

func BenchmarkAgnivade(b *testing.B) {
	for _, n := range sizes {
		x, y := benchInputs(n, 26)
		b.Run(sizeName(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = agnivade.ComputeDistance(x, y)
			}
		})
	}
}

func sizeName(n int) string {
	switch n {
	case 16:
		return "16"
	case 64:
		return "64"
	case 256:
		return "256"
	case 1024:
		return "1024"
	}
	return "?"
}
