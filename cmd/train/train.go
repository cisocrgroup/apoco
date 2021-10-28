package train

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/spf13/cobra"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

// CMD defines the apoco train command.
var CMD = &cobra.Command{
	Use:   "train [CSV...]",
	Short: "Train post-correction models",
	Args:  cobra.MinimumNArgs(1),
	Run:   train,
}

var flags = struct {
	parameter, model, typ string
	nocr, batch           int
}{}

func init() {
	// Train flags
	CMD.PersistentFlags().StringVarP(&flags.parameter, "parameter", "p", "config.toml",
		"set the path to the configuration file")
	CMD.PersistentFlags().StringVarP(&flags.typ, "type", "t", "",
		"set the type of the model (rr, dm, ...)")
	CMD.PersistentFlags().StringVarP(&flags.model, "model", "M", "",
		"set the model path (overwrites the setting in the configuration file)")
	CMD.PersistentFlags().IntVarP(&flags.nocr, "nocr", "n", 0,
		"set the number of parallel OCRs (overwrites the setting in the configuration file)")
	CMD.PersistentFlags().IntVarP(&flags.batch, "batch", "b", 1e8,
		"set the number of parallel OCRs (overwrites the setting in the configuration file)")
}

func train(_ *cobra.Command, args []string) {
	c, err := internal.ReadConfig(flags.parameter)
	chk(err)
	internal.UpdateInConfig(&c.Model, flags.model)
	internal.UpdateInConfig(&c.Nocr, flags.nocr)

	learn, ntrain, fn, err := getTrainingParams(c)
	chk(err)
	lr := &ml.LR{LearningRate: learn, Ntrain: ntrain}
	for _, name := range args {
		fitFile(c, fn, lr, name)
	}

	m, err := internal.ReadModel(c.Model, c.LM, false)
	chk(err)
	m.Put(flags.typ, c.Nocr, lr, fn)
	chk(m.Write(c.Model))
}

func fitFile(c *internal.Config, fn []string, f ml.Fitter, name string) {
	r, err := os.Open(name)
	chk(err)
	defer r.Close()
	fit(c, fn, f, r)
}

func fit(c *internal.Config, fn []string, f ml.Fitter, r io.Reader) {
	s := bufio.NewScanner(r)
	var err float64
	var xs, ys []float64
	for s.Scan() {
		xs, ys = readFeatures(xs, ys, s.Text())
		if len(ys) >= flags.batch {
			apoco.Log("fit %s/%d: xs=%d,ys=%d,lr=%g,ntrain=%d",
				flags.typ, c.Nocr, len(xs), len(ys), f.(*ml.LR).LearningRate, f.(*ml.LR).Ntrain)
			x := mat.NewDense(len(ys), len(xs)/len(ys), xs)
			y := mat.NewVecDense(len(ys), ys)
			chk(logCorrelationMat(c, fn, x))
			err = f.Fit(x, y)
			xs = xs[0:0]
			ys = ys[0:0]
		}
	}
	chk(s.Err())
	if len(ys) > 0 {
		apoco.Log("fit %s/%d: xs=%d,ys=%d,lr=%g,ntrain=%d",
			flags.typ, c.Nocr, len(xs), len(ys), f.(*ml.LR).LearningRate, f.(*ml.LR).Ntrain)
		x := mat.NewDense(len(ys), len(xs)/len(ys), xs)
		y := mat.NewVecDense(len(ys), ys)
		chk(logCorrelationMat(c, fn, x))
		err = f.Fit(x, y)
	}
	log.Printf("fit %s/%d: remaining error: %g", flags.typ, c.Nocr, err)
}

func readFeatures(xs, ys []float64, line string) ([]float64, []float64) {
	vals := strings.Split(line, ",")
	for i := range vals {
		val, err := strconv.ParseFloat(vals[i], 64)
		chk(err)
		if i == len(vals)-1 {
			ys = append(ys, val)
		} else {
			xs = append(xs, val)
		}
	}
	return xs, ys
}

func getTrainingParams(c *internal.Config) (float64, int, []string, error) {
	switch flags.typ {
	case "rr":
		return c.DM.LearningRate, c.RR.Ntrain, c.RR.Features, nil
	case "dm":
		return c.DM.LearningRate, c.DM.Ntrain, c.DM.Features, nil
	case "ms":
		return c.DM.LearningRate, c.MS.Ntrain, c.MS.Features, nil
	}
	return 0, 0, nil, fmt.Errorf("bad type: %s", flags.typ)
}

func logCorrelationMat(c *internal.Config, fn []string, x *mat.Dense) error {
	if !apoco.LogEnabled() {
		return nil
	}
	fs, err := apoco.NewFeatureSet(fn...)
	if err != nil {
		return err
	}
	var names []string
	switch flags.typ {
	case "dm":
		names = fs.Names(c.DM.Features, flags.typ, c.Nocr)
	case "rr":
		names = fs.Names(c.RR.Features, flags.typ, c.Nocr)
	case "ms":
		names = fs.Names(c.MS.Features, flags.typ, c.Nocr)
	default:
		panic("bad type: " + flags.typ)
	}
	cor := correlationMat(x)
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 1, 1, 1, ' ', tabwriter.AlignRight)
	_, cols := cor.Dims()
	fmt.Fprintf(w, "\t")
	for i := 0; i < cols; i++ {
		fmt.Fprintf(w, "\t[%d]", i+1)
	}
	fmt.Fprintln(w, "\t")
	for i, name := range names {
		fmt.Fprintf(w, "[%d]\t%s", i+1, name)
		for j := 0; j < cols; j++ {
			fmt.Fprintf(w, "\t%.2g", cor.At(i, j))
		}
		fmt.Fprintln(w, "\t")
	}
	w.Flush()
	// Log lines
	s := bufio.NewScanner(&buf)
	for s.Scan() {
		apoco.Log(s.Text())
	}
	return s.Err()
}

func correlationMat(m *mat.Dense) *mat.Dense {
	_, c := m.Dims()
	ret := mat.NewDense(c, c, nil)
	var xs, ys []float64
	for i := 0; i < c; i++ {
		for j := 0; j < c; j++ {
			ret.Set(i, j, pearson(m, i, j, &xs, &ys))
		}
	}
	return ret
}

func pearson(m *mat.Dense, x, y int, xs, ys *[]float64) float64 {
	col(m, x, xs)
	col(m, y, ys)
	cov := stat.Covariance(*xs, *ys, nil)
	σx := stat.StdDev(*xs, nil)
	σy := stat.StdDev(*ys, nil)
	return cov / (σx * σy)
}

func col(m *mat.Dense, c int, out *[]float64) {
	r, _ := m.Dims()
	if len(*out) != r {
		*out = make([]float64, r)
	}
	for i := 0; i < r; i++ {
		(*out)[i] = m.At(i, c)
	}
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
