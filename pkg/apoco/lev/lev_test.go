package lev

import (
	"fmt"
	"testing"
)

func TestDistance(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"abc", "abc", 0},
		{"abc", "ABC", 3},
		{"abc", "aBc", 1},
		{"aBc", "abc", 1},
		{"ab", "abc", 1},
		{"abc", "ab", 1},
	} {
		t.Run(fmt.Sprintf("%s-%s", tc.a, tc.b), func(t *testing.T) {
			if got := Distance(tc.a, tc.b); got != tc.want {
				t.Fatalf("expected %d; got %d", tc.want, got)
			}
		})
	}
}

func benchmarkDistance(s1, s2 string, b *testing.B) {
	for i := 0; i < b.N; i++ {
		Distance(s1, s2) //"first long string", "second longer string")
	}
}

func BenchmarkDistance1(b *testing.B) {
	benchmarkDistance("short", "longer", b)
}

func BenchmarkDistance2(b *testing.B) {
	benchmarkDistance("first long string", "second long string", b)
}
