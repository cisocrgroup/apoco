// +build ignore

package ml

import (
	"reflect"
	"strings"
	"testing"
)

func TestReadCSV(t *testing.T) {
	for _, tc := range []struct {
		test   string
		withGT bool
		ys     [][]float64
		gt     []float64
	}{
		{"1,2,3", true, [][]float64{{1, 2}}, []float64{3}},
		{".1,.2,.3", false, [][]float64{{.1, .2, .3}}, nil},
		{"1,2\n3,4", true, [][]float64{{1}, {3}}, []float64{2, 4}},
	} {
		t.Run(tc.test, func(t *testing.T) {
			buf := strings.NewReader(tc.test)
			ys, gt, err := readCSV(buf, tc.withGT)
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if !reflect.DeepEqual(tc.ys, ys) {
				t.Fatalf("expected %#v; got %#v", tc.ys, ys)
			}
			if !reflect.DeepEqual(tc.gt, gt) {
				t.Fatalf("expected %#v; got %#v", tc.gt, gt)
			}
		})
	}
}
