package align

import (
	"reflect"
	"testing"

	"git.sr.ht/~flobar/lev"
)

func TestDo2(t *testing.T) {
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
		{" ab  cd  ", "ab cd", []string{"ab", "ab", "cd", "cd"}},
		{"n uch ter in", "nuchter in",
			[]string{"n", "nuchter", "uch", "nuchter", "ter", "nuchter", "in", "in"}},
		{"a bc  d", "a b d", []string{"a", "a", "bc", "b", "d", "d"}},
	} {
		t.Run(tc.master, func(t *testing.T) {
			pos := Do([]rune(tc.master), []rune(tc.other))
			var got []string
			for i := range pos {
				for j := range pos[i] {
					got = append(got, string(pos[i][j].Slice()))
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expected %#v; got %#v", tc.want, got)
			}
		})
	}
}

func TestDo3(t *testing.T) {
	for _, tc := range []struct {
		master, other1, other2 string
		want                   []string
	}{
		{
			" H ergen ser g i eß u n g en",
			"  er;en oer g ieß u n gen",
			"Herzengießungen",
			[]string{
				"H", "er;en", "Herzengießungen",
				"ergen", "oer", "Herzengießungen",
				"ser", "g", "Herzengießungen",
				"g", "ieß", "Herzengießungen",
				"i", "u", "Herzengießungen",
				"eß", "n", "Herzengießungen",
				"u", "n", "Herzengießungen",
				"n", "gen", "Herzengießungen",
				"g", "gen", "Herzengießungen",
				"en", "gen", "Herzengießungen",
			},
		},
	} {
		t.Run(tc.master, func(t *testing.T) {
			pos := Do([]rune(tc.master), []rune(tc.other1), []rune(tc.other2))
			var got []string
			for i := range pos {
				for j := range pos[i] {
					got = append(got, string(pos[i][j].Slice()))
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expected %#v; got %#v", tc.want, got)
			}
		})
	}
}

func TestLev(t *testing.T) {
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
		{" ab  cd  ", "ab cd", []string{"ab", "ab", "cd", "cd"}},
		{"n uch ter in", "nuchter in",
			[]string{"n", "nuchter", "uch", "nuchter", "ter", "nuchter", "in", "in"}},
		{"a bc  d", "a b d", []string{"a", "a", "bc", "b", "d", "d"}},
	} {
		t.Run(tc.master, func(t *testing.T) {
			var m lev.Mat
			pos := Lev(&m, []rune(tc.master), []rune(tc.other))
			var got []string
			for i := range pos {
				for j := range pos[i] {
					got = append(got, string(pos[i][j].Slice()))
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expected %#v; got %#v", tc.want, got)
			}
		})
	}
}
