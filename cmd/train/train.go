package train

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"

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
	extensions                           []string
	parameter, model                     string
	nocr                                 int
	cache, cautious, update, correlation bool
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
	CMD.PersistentFlags().BoolVarP(&flags.cautious, "cautious", "a", false,
		"use cautious training (overwrites the setting in the configuration file)")
	CMD.PersistentFlags().BoolVarP(&flags.update, "update", "u", false,
		"update the model if it already exists")
	CMD.PersistentFlags().BoolVarP(&flags.correlation, "correlation", "C", false,
		"print correlation matrix for features")
	// Subcommands
	CMD.AddCommand(rrCMD, dmCMD)
}

func printCorrelationMat(c *apoco.Config, fs apoco.FeatureSet, x *mat.Dense, dm bool) error {
	var names []string
	if dm {
		names = fs.Names(c.DMFeatures, c.Nocr, dm)
	} else {
		names = fs.Names(c.RRFeatures, c.Nocr, dm)
	}
	cor := correlationMat(x)
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', tabwriter.AlignRight)
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
	return w.Flush()
}

func correlationMat(m *mat.Dense) *mat.Dense {
	_, c := m.Dims()
	ret := mat.NewDense(c, c, nil)
	for i := 0; i < c; i++ {
		for j := 0; j < c; j++ {
			ret.Set(i, j, pearson(m, i, j))
		}
	}
	return ret
}

func pearson(m *mat.Dense, x, y int) float64 {
	xs := col(m, x)
	ys := col(m, y)
	cov := stat.Covariance(xs, ys, nil)
	σx := stat.StdDev(xs, nil)
	σy := stat.StdDev(ys, nil)
	return cov / (σx * σy)
}

func col(m *mat.Dense, c int) []float64 {
	r, _ := m.Dims()
	ret := make([]float64, r)
	for i := 0; i < r; i++ {
		ret[i] = m.At(i, c)
	}
	return ret
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
