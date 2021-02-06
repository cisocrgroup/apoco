// +build ignore

package ml

import (
	"fmt"
	"math"
	"math/rand"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

// NNConfig represents a configuration for a neural net.
type NNConfig struct {
	Input  int
	Output int
	Hidden int
	Epochs int
	Alpha  float64
}

// NN represents a neural net with one input, hidden and output layer.
type NN struct {
	config  NNConfig
	whidden *mat.Dense
	bhidden *mat.Dense
	wout    *mat.Dense
	bout    *mat.Dense
}

// NewNN creates and initializes a neural net instance.
func NewNN(config NNConfig) *NN {
	nn := &NN{config: config}
	nn.init()
	return nn
}

func (nn *NN) init() {
	whidden := make([]float64, nn.config.Input*nn.config.Hidden)
	bhidden := make([]float64, nn.config.Hidden)
	wout := make([]float64, nn.config.Hidden*nn.config.Output)
	bout := make([]float64, nn.config.Output)
	for _, arr := range [][]float64{whidden, bhidden, wout, bout} {
		for i := range arr {
			arr[i] = rand.Float64()
		}
	}
	nn.whidden = mat.NewDense(nn.config.Input, nn.config.Hidden, whidden)
	nn.bhidden = mat.NewDense(1, nn.config.Hidden, bhidden)
	nn.wout = mat.NewDense(nn.config.Hidden, nn.config.Output, wout)
	nn.bout = mat.NewDense(1, nn.config.Output, bout)
}

// Train trains the neural network on the given data.
func (nn *NN) Train(xs [][]float64, ys []float64) error {
	if len(xs) == 0 || len(xs[0]) == 0 || len(ys) == 0 {
		return fmt.Errorf("train: zero dimensions")
	}
	XS := makeDenseMat(xs)
	YS := makeDenseVec(ys)
	// r, c := YS.Dims()
	// output := mat.NewDense(r, c, nil)
	for i := 0; i < nn.config.Epochs; i++ {
		nn.forwardBackward(XS, YS)
	}
	return nil
}

// Predict returns the predictions for the given feature vectors.
func (nn *NN) Predict(xs [][]float64) ([]float64, error) {
	if len(xs) == 0 || len(xs[0]) == 0 {
		return nil, fmt.Errorf("predict: zero dimensions")
	}
	XS := makeDenseMat(xs)
	r, _ := XS.Dims()
	_, c := nn.whidden.Dims()
	hiddenLayerInput := mat.NewDense(r, c, nil)
	hiddenLayerInput.Mul(XS, nn.whidden)
	addBHidden := func(_, col int, v float64) float64 { return v + nn.bhidden.At(0, col) }
	hiddenLayerInput.Apply(addBHidden, hiddenLayerInput)

	r, c = hiddenLayerInput.Dims()
	hiddenLayerActivations := mat.NewDense(r, c, nil)
	applySigmoid := func(_, _ int, v float64) float64 { return sigmoid(v) }
	hiddenLayerActivations.Apply(applySigmoid, hiddenLayerInput)

	r, _ = hiddenLayerActivations.Dims()
	_, c = nn.wout.Dims()
	outputLayerInput := mat.NewDense(r, c, nil)
	outputLayerInput.Mul(hiddenLayerActivations, nn.wout)
	addBOut := func(_, col int, v float64) float64 { return v + nn.bout.At(0, col) }
	outputLayerInput.Apply(addBOut, outputLayerInput)
	// r, c := outputLayerInput.Dims()
	// nn.output = mat.NewDense(r, c, nil)
	outputLayerInput.Apply(applySigmoid, outputLayerInput)
	return outputLayerInput.RawRowView(0), nil
}

func (nn *NN) forwardBackward(xs, ys *mat.Dense) {
	// forward
	r, _ := xs.Dims()
	_, c := nn.whidden.Dims()
	hiddenLayerInput := mat.NewDense(r, c, nil)
	hiddenLayerInput.Mul(xs, nn.whidden)
	addBHidden := func(_, col int, v float64) float64 { return v + nn.bhidden.At(0, col) }
	hiddenLayerInput.Apply(addBHidden, hiddenLayerInput)

	r, c = hiddenLayerInput.Dims()
	hiddenLayerActivations := mat.NewDense(r, c, nil)
	applySigmoid := func(_, _ int, v float64) float64 { return sigmoid(v) }
	hiddenLayerActivations.Apply(applySigmoid, hiddenLayerInput)

	r, _ = hiddenLayerActivations.Dims()
	_, c = nn.wout.Dims()
	outputLayerInput := mat.NewDense(r, c, nil)
	// rr, cc := nn.wout.Dims()
	// apoco.L("mul: %dx%d X %dx%d", r, c, rr, cc)
	outputLayerInput.Mul(hiddenLayerActivations, nn.wout)
	addBOut := func(_, col int, v float64) float64 { return v + nn.bout.At(0, col) }
	outputLayerInput.Apply(addBOut, outputLayerInput)
	outputLayerInput.Apply(applySigmoid, outputLayerInput)

	// backward
	r, c = outputLayerInput.Dims()
	networkError := mat.NewDense(r, c, nil)
	networkError.Sub(ys, outputLayerInput)

	r, c = outputLayerInput.Dims()
	slopeOutputLayer := mat.NewDense(r, c, nil)
	applySigmoidPrime := func(_, _ int, v float64) float64 { return sigmoidp(v) }
	slopeOutputLayer.Apply(applySigmoidPrime, outputLayerInput)
	r, c = hiddenLayerActivations.Dims()
	slopeHiddenLayer := mat.NewDense(r, c, nil)
	slopeHiddenLayer.Apply(applySigmoidPrime, hiddenLayerActivations)

	r, c = networkError.Dims()
	dOutput := mat.NewDense(r, c, nil)
	dOutput.MulElem(networkError, slopeOutputLayer)
	r, _ = dOutput.Dims()
	_, c = nn.wout.T().Dims()
	errorAtHiddenLayer := mat.NewDense(r, c, nil)
	errorAtHiddenLayer.Mul(dOutput, nn.wout.T())

	r, c = errorAtHiddenLayer.Dims()
	dHiddenLayer := mat.NewDense(r, c, nil)
	dHiddenLayer.MulElem(errorAtHiddenLayer, slopeHiddenLayer)

	// adjust parameters
	r, _ = hiddenLayerActivations.T().Dims()
	_, c = dOutput.Dims()
	wOutAdj := mat.NewDense(r, c, nil)
	wOutAdj.Mul(hiddenLayerActivations.T(), dOutput)
	wOutAdj.Scale(nn.config.Alpha, wOutAdj)
	nn.wout.Add(nn.wout, wOutAdj)

	bOutAdj := sumAlongAxis(axisCol, dOutput)
	bOutAdj.Scale(nn.config.Alpha, bOutAdj)
	bOutAdj.Add(nn.bout, bOutAdj)

	r, _ = xs.T().Dims()
	_, c = dHiddenLayer.Dims()
	wHiddenAdj := mat.NewDense(r, c, nil)
	wHiddenAdj.Mul(xs.T(), dHiddenLayer)

	bHiddenAdj := sumAlongAxis(axisCol, dHiddenLayer)
	bHiddenAdj.Scale(nn.config.Alpha, bHiddenAdj)
	nn.bhidden.Add(nn.bhidden, bHiddenAdj)
}

func makeDenseMat(xs [][]float64) *mat.Dense {
	XS := mat.NewDense(len(xs), len(xs[0]), nil)
	for i := range xs {
		for j := range xs[i] {
			XS.Set(i, j, xs[i][j])
		}
	}
	return XS
}

func makeDenseVec(xs []float64) *mat.Dense {
	return mat.NewDense(len(xs), 1, xs)
}

type axis int

const (
	axisCol axis = 0
	axisRow axis = 1
)

func sumAlongAxis(a axis, m *mat.Dense) *mat.Dense {
	r, c := m.Dims()
	switch a {
	case axisCol:
		data := make([]float64, c)
		for i := 0; i < c; i++ {
			col := mat.Col(nil, i, m)
			data[i] = floats.Sum(col)
		}
		return mat.NewDense(1, c, data)
	case axisRow:
		data := make([]float64, r)
		for i := 0; i < r; i++ {
			row := mat.Col(nil, i, m)
			data[i] = floats.Sum(row)
		}
		return mat.NewDense(r, 1, data)
	default:
		panic(fmt.Sprintf("bad axis: %d", a))
	}
}

func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

func sigmoidp(x float64) float64 {
	return x * (1.0 - x)
}
