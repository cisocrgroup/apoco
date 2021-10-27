package ml

import (
	"gonum.org/v1/gonum/mat"
)

type Predictor interface {
	Predict(x *mat.Dense) *mat.VecDense
}

type Fitter interface {
	Fit(x *mat.Dense, y *mat.VecDense) float64
}

// Predefined values for true and false.
const (
	False = float64(0)
	//False = float64(-1)
	True = float64(1)
)

// Bool converts a bool to a value representing false or true.
func Bool(t bool) float64 {
	if t {
		return True
	}
	return False
}
