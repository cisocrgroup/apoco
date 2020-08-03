package ml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"

	"gonum.org/v1/gonum/mat"
)

func TestLR(t *testing.T) {
	for _, tc := range []struct {
		x, y, want []float64
	}{
		{
			[]float64{10, 5, 8, 4, 8, 2, 10, 4, 10, 10, 3, 4},
			[]float64{1, 0, 1},
			[]float64{.5, .3, .7},
		},
	} {
		x := mat.NewDense(len(tc.y), len(tc.x)/len(tc.y), tc.x)
		y := mat.NewVecDense(3, tc.y)
		lr := LR{LearningRate: 0.05, Ntrain: 5}
		lr.Fit(x, y)
		// log.Printf("weight: %v", lr.weights.RawVector().Data)
		got := lr.Predict(x, 0.5)
		if !reflect.DeepEqual(got.RawVector().Data, tc.y) {
			t.Fatalf("expected %v; got %v", tc.y, got.RawVector().Data)
		}
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
			if !floatArrayEqual(tc.test, tc.want, 1e-5) {
				t.Fatalf("expected %v; got %v", tc.want, tc.test)
			}
		})
	}
}

func TestZScoreNormalize(t *testing.T) {
	for _, tc := range []struct {
		test, want []float64
		r, c       int
	}{
		{[]float64{1, 2, 3}, []float64{-1, 0, 1}, 3, 1},
		{[]float64{1, 1, 1}, []float64{-1, 0, 1}, 3, 1},
	} {
		t.Run(fmt.Sprintf("%v", tc.test), func(t *testing.T) {
			xs := mat.NewDense(tc.r, tc.c, tc.test)
			if err := zScoreNormalization(xs); err != nil {
				t.Fatalf("got error: %v", err)
			}
			if !floatArrayEqual(tc.test, tc.want, 1e-5) {
				t.Fatalf("expected %v; got %v", tc.want, tc.test)
			}
		})
	}
}

func floatArrayEqual(a, b []float64, tolerance float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if diff := math.Abs(a[i] - b[i]); diff > tolerance {
			return false
		}
	}
	return true
}
