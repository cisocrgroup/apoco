package ml

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"gonum.org/v1/gonum/mat"
)

func eqf64(a, b, epsilon float64) bool {
	return (a-b) < epsilon && (b-a) < epsilon
}

func eqf64s(a, b []float64, epsilon float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !eqf64(a[i], b[i], epsilon) {
			return false
		}
	}
	return true
}

func TestWeights(t *testing.T) {
	for _, tc := range []struct {
		name       string
		x, y, want []float64
	}{
		{
			"first",
			[]float64{10, 5, 8, 4, 8, 2, 10, 4, 10, 10, 3, 4},
			[]float64{1, 0, 1},
			[]float64{.108, .225, -.153, 0.008},
		},
		{
			"second",
			[]float64{230.1, 37.8, 69.2, 44.5, 39.3, 45.1, 17.2, 45.9, 69.3, 151.5, 41.3, 58.5, 180.8, 10.8, 58.4, 8.7, 48.9, 75, 57.5, 32.8, 23.5, 120.2, 19.6, 11.6, 8.6, 2.1, 1},
			[]float64{22.1, 10.4, 9.3, 18.5, 12.9, 7.2, 11.8, 13.2, 4.8},
			[]float64{346.525, 92.544, 141.201},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := mat.NewDense(len(tc.y), len(tc.x)/len(tc.y), tc.x)
			y := mat.NewVecDense(len(tc.y), tc.y)
			lr := LR{LearningRate: 0.05, Ntrain: 5}
			lr.Fit(x, y)
			if !eqf64s(lr.weights.RawVector().Data, tc.want, 1e-3) {
				t.Errorf("expected %v; got %v", tc.want, lr.weights.RawVector().Data)
			}
		})
	}
}
func TestPredict(t *testing.T) {
	for _, tc := range []struct {
		name string
		x, y []float64
	}{
		{
			"first",
			[]float64{10, 5, 8, 4, 8, 2, 10, 4, 10, 10, 3, 4},
			[]float64{1, 0, 1},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := mat.NewDense(len(tc.y), len(tc.x)/len(tc.y), tc.x)
			y := mat.NewVecDense(len(tc.y), tc.y)
			lr := LR{LearningRate: 0.05, Ntrain: 5}
			lr.Fit(x, y)
			if got := lr.Predict(x, 0.5); !eqf64s(got.RawVector().Data, tc.y, 1e-5) {
				t.Errorf("expected %v; got %v", tc.y, got.RawVector().Data)
			}
		})
	}
}

func TestJSON(t *testing.T) {
	lr := LR{
		LearningRate: 0.1,
		Ntrain:       1000,
		weights:      mat.NewVecDense(3, []float64{.1, .2, .3}),
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&lr); err != nil {
		t.Fatalf("got error: %v", err)
	}
	var lr2 LR
	if err := json.NewDecoder(&buf).Decode(&lr2); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if !reflect.DeepEqual(lr, lr2) {
		t.Fatalf("expected %v; got %v", lr, lr2)
	}
}

func TestGOB(t *testing.T) {
	lr := LR{
		LearningRate: 0.1,
		Ntrain:       1000,
		weights:      mat.NewVecDense(3, []float64{.1, .2, .3}),
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&lr); err != nil {
		t.Fatalf("got error: %v", err)
	}
	var lr2 LR
	if err := gob.NewDecoder(&buf).Decode(&lr2); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if !reflect.DeepEqual(lr, lr2) {
		t.Fatalf("expected %v; got %v", lr, lr2)
	}
}

func TestNormalize(t *testing.T) {
	for _, tc := range []struct {
		test, want []float64
		r, c       int
	}{
		{[]float64{1, 2, 3}, []float64{-.5, 0, .5}, 3, 1},
		{[]float64{1, 10, 2, 20, 3, 30}, []float64{-.5, -.5, 0, 0, .5, .5}, 3, 2},
		// {[]float64{.1, .2, .3}, []float64{(.1 - .2) / 1, (.2 - .2) / 1, (.3 - .2) / 1}, 3, 1},
	} {
		t.Run(fmt.Sprintf("%v", tc.test), func(t *testing.T) {
			xs := mat.NewDense(tc.r, tc.c, tc.test)
			if err := Normalize(xs); err != nil {
				t.Fatalf("got error: %v", err)
			}
			if !eqf64s(tc.test, tc.want, 1e-5) {
				t.Fatalf("expected %v; got %v", tc.want, tc.test)
			}
		})
	}
}
