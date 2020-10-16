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
