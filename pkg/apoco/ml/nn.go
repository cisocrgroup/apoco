package ml

import (
	"fmt"
	"log"
	"math"
	"strings"

	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat/distuv"
)

func logmat(pref string, m mat.Matrix) {
	r, c := m.Dims()
	for i := 0; i < r; i++ {
		var b strings.Builder
		for j := 0; j < c; j++ {
			fmt.Fprintf(&b, " %.4f", m.At(i, j))
		}
		log.Printf("%s: %s", pref, b.String())
	}
}

type NN struct {
	wh, wo  mat.Dense
	alloc   allocator
	lr      float64
	inputs  int
	hiddens int
	outputs int
	epochs  int
}

func CreateNetwork(input, hidden int, lr float64) *NN {
	nn := NN{
		inputs:  input,
		hiddens: hidden,
		outputs: 2, // Fixed to 2 classes True/False
		lr:      lr,
	}
	nn.wh.ReuseAs(nn.hiddens, nn.inputs)
	randomInit(&nn.wh, float64(nn.inputs))
	nn.wo.ReuseAs(nn.outputs, nn.hiddens)
	randomInit(&nn.wo, float64(nn.hiddens))
	return &nn
}

func (nn *NN) classes2mat(classes []bool) []float64 {
	if nn.outputs != 2 {
		panic("nn: classes2mat: output dimension must be 2")
	}
	ys := make([]float64, 0, len(classes)*nn.outputs)
	for i := range classes {
		if classes[i] {
			ys = append(ys, .01)
			ys = append(ys, .99)
		} else {
			ys = append(ys, .99)
			ys = append(ys, .01)
		}
	}
	return ys
}

func (nn *NN) vec2mat(vec *mat.VecDense) *mat.Dense {
	if nn.outputs != 2 {
		panic("nn: classes2mat: output dimension must be 2")
	}
	ys := make([]float64, 0, vec.Len()*nn.outputs)
	for i := 0; i < vec.Len(); i++ {
		switch vec.AtVec(i) {
		case True:
			ys = append(ys, .01)
			ys = append(ys, .99)
		case False:
			ys = append(ys, .99)
			ys = append(ys, .01)
		default:
			panic("bad class value")
		}
	}
	return mat.NewDense(vec.Len(), nn.outputs, ys)
}

func (nn *NN) Predict(x *mat.Dense) *mat.VecDense {
	r, _ := x.Dims()
	ys := mat.NewVecDense(r, nil)
	for i := 0; i < r; i++ {
		// forward propagation
		inputs := x.RowView(i) //mat.NewDense(c, 1, xs[i:i+c])
		hiddenIn := dot(&nn.wh, inputs)
		// hiddenInputs := dot(&nn.wh, inputs)
		hiddenOut := apply(sigmoid, hiddenIn)
		// hiddenOutputs := apply(sigmoid, hiddenInputs)
		finalIn := dot(&nn.wo, hiddenOut)
		// finalInputs := dot(&nn.wo, hiddenOutputs)
		finalOut := apply(sigmoid, finalIn)
		// finalOutputs := apply(sigmoid, finalInputs)
		if finalOut.At(0, 0) > finalOut.At(1, 0) {
			ys.SetVec(i, -math.Abs(finalOut.At(0, 0)))
		} else {
			ys.SetVec(i, math.Abs(finalOut.At(1, 0)))
		}
	}
	return ys
}

// Fit trains the neural network on the given data.
func (nn *NN) Fit(x *mat.Dense, y *mat.VecDense) float64 {
	r, _ := x.Dims()
	ys := nn.vec2mat(y)
	for i := 0; i < nn.epochs; i++ {
		for i := 0; i < r; i++ {
			nn.train(x.RowView(i), ys.RowView(i)) //.T())
		}
	}
	return 0.0
}

func (nn *NN) train(inputs, targets mat.Matrix) {
	// Forward propagation.
	hiddenIn := dot(&nn.wh, inputs)
	hiddenOut := apply(sigmoid, hiddenIn)
	finalIn := dot(&nn.wo, hiddenOut)
	finalOut := apply(sigmoid, finalIn)
	// nn.hiddenIn.Product(&nn.wh, inputs)
	// nn.hiddenOut.Apply(sigmoid, &nn.hiddenIn)
	// nn.finalIn.Product(&nn.wo, &nn.hiddenOut)
	// nn.finalOut.Apply(sigmoid, &nn.finalIn)

	// Calculate errors.
	outErr := sub(targets, finalOut)
	hiddenErr := dot(nn.wo.T(), outErr)
	// nn.outErr.Sub(targets, &nn.finalOut)
	// nn.hiddenErr.Product(nn.wo.T(), &nn.outErr)

	// Backward propagation.
	// sigmoidp(&nn.finalOut, &nn.tmp)
	// nn.tmp.Mul(&nn.outErr, &nn.finalOut)
	// nn.tmp.Product(&nn.tmp, nn.hiddenOut.T())
	// nn.tmp.Scale(nn.lr, &nn.tmp)
	// nn.wo.Add(&nn.wo, &nn.tmp)
	nn.wo.Add(&nn.wo, scale(nn.lr,
		dot(multiply(outErr, sigmoidp(finalOut)), hiddenOut.T())))

	// sigmoidp(&nn.hiddenOut, &nn.tmp)
	// nn.tmp.Mul(&nn.hiddenErr, &nn.hiddenOut)
	// nn.tmp.Product(&nn.tmp, inputs.T())
	// nn.tmp.Scale(nn.lr, &nn.tmp)
	// nn.wh.Add(&nn.wh, &nn.tmp)
	nn.wh.Add(&nn.wh, scale(nn.lr,
		dot(multiply(hiddenErr, sigmoidp(hiddenOut)), inputs.T())))
}

func dot(m, n mat.Matrix) mat.Matrix {
	r, _ := m.Dims()
	_, c := n.Dims()

	//o := nn.alloc.newMat(r, c)
	o := mat.NewDense(r, c, nil)

	o.Product(m, n)
	return o
}

func apply(fn func(i, j int, v float64) float64, m mat.Matrix) mat.Matrix {
	r, c := m.Dims()
	o := mat.NewDense(r, c, nil)
	o.Apply(fn, m)
	return o
}

func scale(s float64, m mat.Matrix) mat.Matrix {
	r, c := m.Dims()
	o := mat.NewDense(r, c, nil)
	o.Scale(s, m)
	return o
}

func multiply(m, n mat.Matrix) mat.Matrix {
	r, c := m.Dims()
	o := mat.NewDense(r, c, nil)
	o.MulElem(m, n)
	return o
}

func add(out *mat.Dense, m, n mat.Matrix) mat.Matrix {
	out.Add(m, n)
	return out
}

func sub(m, n mat.Matrix) mat.Matrix {
	r, c := m.Dims()
	o := mat.NewDense(r, c, nil)
	o.Sub(m, n)
	return o
}

func addScalar(i float64, m mat.Matrix) mat.Matrix {
	r, c := m.Dims()
	a := make([]float64, r*c)
	for x := 0; x < r*c; x++ {
		a[x] = i
	}
	n := mat.NewDense(r, c, a)
	return add(n, m, n)
}

func sigmoid(r, c int, z float64) float64 {
	return 1.0 / (1 + math.Exp(-1*z))
}

func sigmoidp(m mat.Matrix) mat.Matrix {
	rows, _ := m.Dims()
	o := make([]float64, rows)
	for i := range o {
		o[i] = 1
	}
	ones := mat.NewDense(rows, 1, o)
	return multiply(m, sub(ones, m))
}

func randomInit(m *mat.Dense, v float64) {
	dist := distuv.Uniform{
		Min: -1 / math.Sqrt(v),
		Max: 1 / math.Sqrt(v),
	}
	r, c := m.Dims()
	for i := 0; i < r; i++ {
		for j := 0; j < c; j++ {
			m.Set(i, j, dist.Rand())
		}
	}
}

type allocator struct {
	data []*node
}

func (a *allocator) get(n int) *node {
	for i := range a.data {
		if a.data[i].n == n {
			return a.data[i]
		}
	}
	node := &node{n: n}
	a.data = append(a.data, node)
	return node
}

func (a *allocator) alloc(n int) []float64 {
	return a.get(n).alloc()
}

func (a *allocator) newMat(r, c int) *mat.Dense {
	return mat.NewDense(r, c, a.alloc(r*c))
}

type node struct {
	data [][]float64
	i, n int
}

func (nd *node) alloc() []float64 {
	if nd.i < len(nd.data) {
		ret := nd.data[nd.i]
		nd.i++
		return ret
	}
	ret := make([]float64, nd.n)
	nd.data = append(nd.data, ret)
	nd.i = len(nd.data)
	return ret
}

var _ Predictor = &NN{}
var _ Fitter = &NN{}
