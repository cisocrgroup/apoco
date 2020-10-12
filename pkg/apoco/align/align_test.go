package align

import (
	"reflect"
	"testing"
)

func Test(t *testing.T) {
	for _, tc := range []struct {
		master, other string
		want          []string
	}{
		{"", "", []string{"", ""}},
		{"", "T", []string{"", "T"}},
		{"", "A B", []string{"", "A B"}},
		{"T", "", []string{"T", ""}},
		{"ab cd", "ab cd", []string{"ab", "ab", "cd", "cd"}},
		{"ab cd", "abcd", []string{"ab", "abcd", "cd", "abcd"}},
		{"abcd", "ab cd", []string{"abcd", "ab cd"}},
		{"n uch ter in", "nuchter in",
			[]string{"n", "nuchter", "uch", "nuchter", "ter", "nuchter", "in", "in"}},
	} {
		t.Run(tc.master, func(t *testing.T) {
			pos := Do([]rune(tc.master), []rune(tc.other))
			var got []string
			for i := range pos {
				for j := range pos[i] {
					if j == 0 {
						got = append(got, string(pos[i][j].Slice()))
					} else {
						got = append(got, string(pos[i][j].Slice()))
					}
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expected %v; got %v", tc.want, got)
			}
		})
	}
}
