package ml

import (
	"gonum.org/v1/gonum/mat"
)

// Predictor is used to predict values based on a ml-model.
type Predictor interface {
	Predict(x *mat.Dense) *mat.VecDense
}

// Fitter is used to train a ml-model on input values.
type Fitter interface {
	Fit(x *mat.Dense, y *mat.VecDense) float64
}

// Predefined values for true and false.
const (
	False = float64(0)
	True  = float64(1)
)

// Bool converts a bool to a value representing false or true.
func Bool(t bool) float64 {
	if t {
		return True
	}
	return False
}
