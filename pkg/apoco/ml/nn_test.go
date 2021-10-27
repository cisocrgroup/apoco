package ml

import (
	"testing"

	"gonum.org/v1/gonum/mat"
)

func TestNNXOR(t *testing.T) {
	xs := mat.NewDense(4, 2, []float64{
		.01, .01, // false
		.01, .99, // true
		.99, .01, // true
		.99, .99, // false
	})
	want := []float64{False, True, True, False}
	ys := mat.NewVecDense(4, want)
	nn := CreateNetwork(2, 4, .5) //1e-4)
	nn.epochs = 10000
	nn.Fit(xs, ys)

	got := nn.Predict(xs)
	for i := range want {
		if !check(want[i], got.AtVec(i)) {
			t.Errorf("expected %g; got %g", want[i], got.AtVec(i))
		}
	}
}

func check(want, got float64) bool {
	if want == True {
		return got > 0
	}
	return got < 0
}
