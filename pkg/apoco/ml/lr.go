package ml

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"math"

	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

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

// LR implements LinearRegression
type LR struct {
	weights      *mat.VecDense
	LearningRate float64
	Ntrain       int
}

func (lr *LR) gradient(x *mat.Dense, y, p *mat.VecDense) (*mat.VecDense, float64) {
	r, _ := x.Dims()
	var gradient, dif mat.VecDense
	dif.SubVec(p, y)
	err := averageError(&dif)
	gradient.MulVec(x.T(), &dif)
	gradient.ScaleVec(1.0/float64(r), &gradient)
	return &gradient, err
}

func averageError(dif *mat.VecDense) float64 {
	sum := 0.0
	for i := 0; i < dif.Len(); i++ {
		sum += dif.AtVec(i) * dif.AtVec(i)
	}
	return math.Sqrt(sum) / float64(dif.Len())
}

func (lr *LR) sigmoid(x *mat.VecDense) *mat.VecDense {
	for i := 0; i < x.Len(); i++ {
		x.SetVec(i, 1.0/(1.0+math.Exp(-x.AtVec(i))))
	}
	return x
}

// Weights returns the weights of the logic regression model.
func (lr *LR) Weights() []float64 {
	return lr.weights.RawVector().Data

}

// PredictProb calculates the probablility predictions for the given values.
func (lr *LR) PredictProb(x *mat.Dense) *mat.VecDense {
	var tmp mat.VecDense
	tmp.MulVec(x, lr.weights)
	return lr.sigmoid(&tmp)
}

// Predict calculates the predictions for the given values.
func (lr *LR) Predict(x *mat.Dense, t float64) *mat.VecDense {
	tmp := lr.PredictProb(x)
	for i := 0; i < tmp.Len(); i++ {
		if tmp.AtVec(i) > t {
			tmp.SetVec(i, True)
		} else {
			tmp.SetVec(i, False)
		}
	}
	return tmp
}

// Fit fits the linear regression model and returns its final error.
func (lr *LR) Fit(x *mat.Dense, y *mat.VecDense) float64 {
	_, c := x.Dims()
	lr.weights = mat.NewVecDense(c, nil)
	errb := math.MaxFloat64
	for i := 0; i < lr.Ntrain; i++ {
		pred := lr.PredictProb(x)
		gradient, err := lr.gradient(x, y, pred)
		if errb < err {
			// log.Printf("[%d] break %e/%e", i, errb, err)
			return errb
		}
		gradient.ScaleVec(lr.LearningRate, gradient)
		lr.weights.SubVec(lr.weights, gradient)
		// if i%100 == 0 {
		// 	log.Printf("[%d] %e/%e", i, errb, err)
		// }
		errb = err
	}
	return errb
}

type lrdata struct {
	Weights      []float64
	LearningRate float64
	Ntrain       int
}

// MarshalJSON implements the json.Marshal interface.
func (lr *LR) MarshalJSON() ([]byte, error) {
	data := lrdata{
		LearningRate: lr.LearningRate,
		Ntrain:       lr.Ntrain,
		Weights:      lr.Weights(),
	}
	return json.Marshal(data)
}

// GobEncode implements the GobEncoder interface.
func (lr *LR) GobEncode() ([]byte, error) {
	data := lrdata{
		LearningRate: lr.LearningRate,
		Ntrain:       lr.Ntrain,
		Weights:      lr.Weights(),
	}
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(data)
	return buf.Bytes(), err
}

// UnmarshalJSON implements the json.Unmarshal interface.
func (lr *LR) UnmarshalJSON(data []byte) error {
	var tmp lrdata
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	*lr = LR{
		Ntrain:       tmp.Ntrain,
		LearningRate: tmp.LearningRate,
		weights:      mat.NewVecDense(len(tmp.Weights), tmp.Weights),
	}
	return nil
}

// GobDecode implements the GobDecoder interface.
func (lr *LR) GobDecode(data []byte) error {
	var tmp lrdata
	r := bytes.NewReader(data)
	if err := gob.NewDecoder(r).Decode(&tmp); err != nil {
		return err
	}
	*lr = LR{
		Ntrain:       tmp.Ntrain,
		LearningRate: tmp.LearningRate,
		weights:      mat.NewVecDense(len(tmp.Weights), tmp.Weights),
	}
	return nil
}

// Normalize normalizes the the given feature vectors.
func Normalize(xs *mat.Dense) error {
	return meanNormalization(xs)
}

func meanNormalization(xs *mat.Dense) error {
	r, c := xs.Dims()
	if r == 0 || c == 0 {
		return fmt.Errorf("normalize: zero length")
	}
	means := make([]float64, c)
	diff := make([]float64, c)
	for j := 0; j < c; j++ {
		// for j := 0; j < c; j++ {
		max := -math.MaxFloat64
		min := math.MaxFloat64
		var sum float64
		for i := 0; i < r; i++ {
			val := xs.At(i, j)
			if max < val {
				max = val
			}
			if min > val {
				min = val
			}
			sum += val
		}
		// Specifically handle values that are clearly between
		// [0,1] and have a diff of 0.
		if max-min == 0 && max >= 0 && max <= 1 && min >= 0 && min <= 1 {
			min = 0
			max = 1
		} else if max-min == 0 {
			return fmt.Errorf("normalize[%d]: max - min = %f - %f cannot be 0", j, max, min)
		}
		means[j] = sum / float64(r)
		diff[j] = max - min
		for i := 0; i < r; i++ {
			val := (xs.At(i, j) - means[j]) / diff[j]
			xs.Set(i, j, val)
		}
	}
	return nil
}

// TODO: Remove this
func zScoreNormalization(xs *mat.Dense) error {
	r, c := xs.Dims()
	tmp := make([]float64, r)
	for j := 0; j < c; j++ {
		cols := xs.ColView(j)
		for i := range tmp {
			tmp[i] = cols.AtVec(i)
		}
		xbar, sigma := stat.MeanStdDev(tmp, nil)
		for i := range tmp {
			x := tmp[i]
			xp := (x - xbar) / sigma
			xs.Set(i, j, xp)
		}
	}
	return nil
}
