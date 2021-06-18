package apoco

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCharIO(t *testing.T) {
	for _, tc := range []struct {
		test string
		want Char
	}{
		{"a:.3", Char{.3, 'a'}},
		{"\u017f:.2", Char{.2, '\u017f'}},
		{"b:.4 xxx", Char{.4, 'v'}},
	} {
		t.Run(tc.test, func(t *testing.T) {
			var got Char
			if _, err := fmt.Sscanf(tc.test, "%v", &got); err != nil {
				t.Errorf("error: %v", err)
			}
			if got != tc.want {
				t.Errorf("expected %s; got %s", tc.want, got)
			}
		})
	}
}

func TestCharsIO(t *testing.T) {
	for _, tc := range []struct {
		test string
		want Chars
	}{
		{"a:.3,b:.2", Chars{Char{.3, 'a'}, Char{.2, 'b'}}},
		{"a:.3,c:.3,d:.4", Chars{Char{.3, 'a'}, Char{.3, 'c'}, Char{.4, 'd'}}},
	} {
		t.Run(tc.test, func(t *testing.T) {
			var got Chars
			if _, err := fmt.Sscanf(tc.test, "%v", &got); err != nil {
				t.Errorf("error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %s; got %s", tc.want, got)
			}
		})
	}
}
