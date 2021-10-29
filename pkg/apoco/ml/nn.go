package ml

import (
	"bytes"
	"encoding/gob"
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

type NNConfig struct {
	LearningRate float64
	Epochs       int
	Input        int
	Hidden       int
}

type NN struct {
	wh, wo  *mat.Dense
	alloc   allocator
	lr      float64
	inputs  int
	hiddens int
	outputs int
	epochs  int
}

func NewNN(c NNConfig) *NN {
	nn := NN{
		inputs:  c.Input,
		hiddens: c.Hidden,
		outputs: 2, // Fixed to 2 classes True/False
		epochs:  c.Epochs,
		lr:      c.LearningRate,
	}
	nn.wh = mat.NewDense(nn.hiddens, nn.inputs, nil)
	randomInit(nn.wh, float64(nn.inputs))
	nn.wo = mat.NewDense(nn.outputs, nn.hiddens, nil)
	randomInit(nn.wo, float64(nn.hiddens))
	return &nn
}

func (nn *NN) vec2mat(vec *mat.VecDense) *mat.Dense {
	if nn.outputs != 2 {
		panic("nn: vec2mat: output dimension must be 2")
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
		nn.alloc.reset()
		// forward propagation
		inputs := x.RowView(i) //mat.NewDense(c, 1, xs[i:i+c])
		hiddenIn := nn.dot(nn.wh, inputs)
		hiddenOut := nn.apply(sigmoid, hiddenIn)
		finalIn := nn.dot(nn.wo, hiddenOut)
		finalOut := nn.apply(sigmoid, finalIn)
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
	var lerr mat.Matrix
	for i := 0; i < nn.epochs; i++ {
		for i := 0; i < r; i++ {
			nn.alloc.reset()
			lerr = nn.train(x.RowView(i), ys.RowView(i)) //.T())
		}
	}
	return avgerr(lerr)
}

func (nn *NN) train(inputs, targets mat.Matrix) mat.Matrix {
	// Forward propagation.
	hiddenIn := nn.dot(nn.wh, inputs)
	hiddenOut := nn.apply(sigmoid, hiddenIn)
	finalIn := nn.dot(nn.wo, hiddenOut)
	finalOut := nn.apply(sigmoid, finalIn)
	// Calculate errors.
	outErr := nn.sub(targets, finalOut)
	hiddenErr := nn.dot(nn.wo.T(), outErr)
	// Backward propagation.
	nn.wo.Add(nn.wo, nn.scale(nn.lr,
		nn.dot(nn.multiply(outErr, nn.sigmoidp(finalOut)), hiddenOut.T())))
	nn.wh.Add(nn.wh, nn.scale(nn.lr,
		nn.dot(nn.multiply(hiddenErr, nn.sigmoidp(hiddenOut)), inputs.T())))
	return outErr
}

func avgerr(m mat.Matrix) float64 {
	r, c := m.Dims()
	sum := 0.0
	for i := 0; i < r; i++ {
		for j := 0; j < c; j++ {
			sum += m.At(i, j)
		}
	}
	return sum / float64(r*c)
}

func (nn *NN) dot(m, n mat.Matrix) mat.Matrix {
	r, _ := m.Dims()
	_, c := n.Dims()
	o := nn.alloc.newMat(r, c)
	o.Product(m, n)
	return o
}

func (nn *NN) apply(fn func(i, j int, v float64) float64, m mat.Matrix) mat.Matrix {
	r, c := m.Dims()
	o := nn.alloc.newMat(r, c)
	o.Apply(fn, m)
	return o
}

func (nn *NN) scale(s float64, m mat.Matrix) mat.Matrix {
	r, c := m.Dims()
	o := nn.alloc.newMat(r, c)
	o.Scale(s, m)
	return o
}

func (nn *NN) multiply(m, n mat.Matrix) mat.Matrix {
	r, c := m.Dims()
	o := nn.alloc.newMat(r, c)
	o.MulElem(m, n)
	return o
}

func (nn *NN) sub(m, n mat.Matrix) mat.Matrix {
	r, c := m.Dims()
	o := nn.alloc.newMat(r, c)
	o.Sub(m, n)
	return o
}

func (nn *NN) addScalar(i float64, m mat.Matrix) mat.Matrix {
	r, c := m.Dims()
	n := nn.alloc.newMat(r, c)
	n.Apply(func(r, c int, _ float64) float64 {
		return m.At(r, c) + i
	}, n)
	return n
}

func (nn *NN) sigmoidp(m mat.Matrix) mat.Matrix {
	rows, _ := m.Dims()
	o := nn.alloc.alloc(rows)
	for i := range o {
		o[i] = 1
	}
	ones := mat.NewDense(rows, 1, o)
	return nn.multiply(m, nn.sub(ones, m))
}

func sigmoid(_, _ int, z float64) float64 {
	return 1.0 / (1 + math.Exp(-1*z))
}

type nndata struct {
	WH, WO       []float64
	LearningRate float64
	Ntrain       int
	Input        int
	Hidden       int
	Output       int
}

// return lr.weights.RawVector().Data

// GobEncode implements the GobEncoder interface.
func (nn *NN) GobEncode() ([]byte, error) {
	data := nndata{
		LearningRate: nn.lr,
		Ntrain:       nn.epochs,
		Input:        nn.inputs,
		Hidden:       nn.hiddens,
		Output:       nn.outputs,
		WH:           nn.wh.RawMatrix().Data,
		WO:           nn.wo.RawMatrix().Data,
	}
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(data)
	return buf.Bytes(), err
}

// GobDecode implements the GobDecoder interface.
func (nn *NN) GobDecode(data []byte) error {
	var tmp nndata
	r := bytes.NewReader(data)
	if err := gob.NewDecoder(r).Decode(&tmp); err != nil {
		return err
	}
	*nn = NN{
		lr:      tmp.LearningRate,
		epochs:  tmp.Ntrain,
		inputs:  tmp.Input,
		hiddens: tmp.Hidden,
		outputs: tmp.Output,
		wh:      mat.NewDense(tmp.Hidden, tmp.Input, tmp.WH),
		wo:      mat.NewDense(tmp.Output, tmp.Hidden, tmp.WO),
	}
	return nil
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
	blocks []*block
}

func (a *allocator) reset() {
	for i := range a.blocks {
		a.blocks[i].reset()
	}
}

func (a *allocator) block(n int) *block {
	for i := range a.blocks {
		if a.blocks[i].n == n {
			return a.blocks[i]
		}
	}
	node := &block{n: n}
	a.blocks = append(a.blocks, node)
	return node
}

func (a *allocator) alloc(n int) []float64 {
	return a.block(n).alloc()
}

func (a *allocator) newMat(r, c int) *mat.Dense {
	return mat.NewDense(r, c, a.alloc(r*c))
}

type block struct {
	data [][]float64
	i, n int
}

func (b *block) reset() {
	b.i = 0
}

func (b *block) alloc() []float64 {
	if b.i < len(b.data) {
		ret := b.data[b.i]
		b.i++
		return ret
	}
	ret := make([]float64, b.n)
	b.data = append(b.data, ret)
	b.i = len(b.data)
	return ret
}

var _ Predictor = &NN{}
var _ Fitter = &NN{}
