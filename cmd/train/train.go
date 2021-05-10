package train

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"text/tabwriter"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/spf13/cobra"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

// CMD defines the apoco train command.
var CMD = &cobra.Command{
	Use:   "train",
	Short: "Train post-correction models ",
}

var flags = struct {
	extensions       []string
	parameter, model string
	nocr             int
	cache, update    bool
}{}

func init() {
	// Train flags
	CMD.PersistentFlags().StringVarP(&flags.parameter, "parameter", "p", "config.toml",
		"set the path to the configuration file")
	CMD.PersistentFlags().StringSliceVarP(&flags.extensions, "extensions", "e", []string{".xml"},
		"set the input file extensions")
	CMD.PersistentFlags().StringVarP(&flags.model, "model", "M", "",
		"set the model path (overwrites the setting in the configuration file)")
	CMD.PersistentFlags().IntVarP(&flags.nocr, "nocr", "n", 0,
		"set the number of parallel OCRs (overwrites the setting in the configuration file)")
	CMD.PersistentFlags().BoolVarP(&flags.cache, "cache", "c", false,
		"enable caching of profiles (overwrites the setting in the configuration file)")
	CMD.PersistentFlags().BoolVarP(&flags.update, "update", "u", false,
		"update the model if it already exists")
	// Subcommands
	CMD.AddCommand(rrCMD, dmCMD, msCMD)
}

func logCorrelationMat(c *internal.Config, fs apoco.FeatureSet, x *mat.Dense, typ string) error {
	if !apoco.LogEnabled() {
		return nil
	}
	var names []string
	switch typ {
	case "dm":
		names = fs.Names(c.DMFeatures, typ, c.Nocr)
	case "rr":
		names = fs.Names(c.RRFeatures, typ, c.Nocr)
	case "ms":
		names = fs.Names(c.MS.Features, typ, c.Nocr)
	default:
		panic("bad type: " + typ)
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
