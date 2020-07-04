// +build ignore

package ml

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"math/rand"
	"strconv"
	"strings"

	"gonum.org/v1/gonum/floats"
)

func readCSV(in io.Reader, withGT bool) ([][]float64, []float64, error) {
	var ret [][]float64
	var gt []float64
	s := bufio.NewScanner(in)
	for s.Scan() {
		ys, y, err := parseCSVLine(s.Text(), withGT)
		if err != nil {
			return nil, nil, fmt.Errorf("readCSV: %v", err)
		}
		ret = append(ret, ys)
		if withGT {
			gt = append(gt, y)
		}
		if len(ret) > 1 {
			if len(ret[len(ret)-1]) != len(ret[len(ret)-2]) {
				return nil, nil, fmt.Errorf("readCSV: bad ys length")
			}
		}
	}
	if err := s.Err(); err != nil {
		return nil, nil, fmt.Errorf("readCSV: %v", err)
	}
	if withGT && len(gt) != len(ret) {
		return nil, nil, fmt.Errorf("readCSV: bad ground truth length")
	}
	return ret, gt, nil
}

func parseCSVLine(line string, withGT bool) ([]float64, float64, error) {
	fields := strings.Split(line, ",")
	if len(fields) == 0 || len(fields) < 2 && withGT {
		return nil, 0, fmt.Errorf("parseCSVLine: bad line: %q", line)
	}
	var ret []float64
	var y float64
	if withGT {
		ret = make([]float64, len(fields)-1)
	} else {
		ret = make([]float64, len(fields))
	}
	for i, field := range fields {
		f, err := strconv.ParseFloat(field, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("parseCSVLine: cannot parse float: %q", field)
		}
		if i < len(fields)-1 || !withGT {
			ret[i] = f
		} else {
			y = f
		}
	}
	return ret, y, nil
}

// Weights represents the weights for the logistic regression.
type Weights []float64

// Lreg trains weights using logistic regression.
func Lreg(xs [][]float64, ys []float64, alpha float64, ntrain int) (Weights, error) {
	if len(xs) == 0 || len(xs[0]) == 0 || len(ys) == 0 {
		return nil, fmt.Errorf("lreg: zero dimensions")
	}
	ws := Weights(make([]float64, len(xs[0])))
	for i := range ws {
		ws[i] = rand.Float64()
	}
	// w := make([]float64, len(ws))
	dx := make([]float64, len(ws))
	for n := 0; n < ntrain; n++ {
		for i, x := range xs {
			// t := mat.NewVecDense(x.Len(), nil)
			// t.CopyVec(x)
			// copy(t, x)
			pred := ws.softmax(x)
			// perr := y.AtVec(i) - pred
			perr := math.Abs(ys[i] - pred)
			scale := alpha * perr * pred * (1 - pred)

			// dx := mat.NewVecDense(x.Len(), nil)
			// dx.CopyVec(x)
			// dx.ScaleVec(scale, x)
			copy(dx, x)
			floats.Scale(scale, dx)
			// w.AddVec(w, dx)
			// log.Printf("###")
			// log.Printf("scale = %f, perr = %f", scale, perr)
			// log.Printf("old weights     = %v", ws)
			floats.Add(ws, dx)
			// log.Printf("updated weights = %v", ws)
		}
	}
	return ws, nil
}

func (ws Weights) softmax(x []float64) float64 {
	// v := mat.Dot(w, x)
	// return 1.0 / (1.0 + math.Exp(-v))
	v := floats.Dot(ws, x)
	return 1.0 / (1.0 + math.Exp(-v))
}

// Predict predicts the values for the given features.
func (ws Weights) Predict(xs [][]float64) []float64 {
	ys := make([]float64, len(xs))
	for i := range xs {
		ys[i] = ws.softmax(xs[i])
	}
	return ys
}
