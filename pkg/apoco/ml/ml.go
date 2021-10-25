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
