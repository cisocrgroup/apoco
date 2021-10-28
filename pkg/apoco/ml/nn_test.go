package ml

import (
	"testing"

	"gonum.org/v1/gonum/mat"
)

var (
	xorxs = mat.NewDense(4, 2, []float64{
		.01, .01, // false
		.01, .99, // true
		.99, .01, // true
		.99, .99, // false
	})
	xorys = mat.NewVecDense(4, []float64{False, True, True, False})
)

func TestXorNN(t *testing.T) {
	nn := xorfit()
	got := xorpredict(nn)
	if got.Len() != xorys.Len() {
		t.Fatalf("different lengths: expected %d; got %d", xorys.Len(), got.Len())
	}
	for i := 0; i < xorys.Len(); i++ {
		if !xorcheck(xorys.AtVec(i), got.AtVec(i)) {
			t.Errorf("expected %g; got %g", xorys.AtVec(i), got.AtVec(i))
		}
	}
}

func BenchmarkXorNN(b *testing.B) {
	for i := 0; i < b.N; i++ {
		nn := xorfit()
		xorpredict(nn)
	}
}

func xorfit() *NN {
	nn := CreateNetwork(2, 4, .5)
	nn.epochs = 10000
	nn.Fit(xorxs, xorys)
	return nn
}

func xorpredict(nn *NN) *mat.VecDense {
	return nn.Predict(xorxs)
}

func xorcheck(want, got float64) bool {
	if want == True {
		return got > 0
	}
	return got < 0
}
