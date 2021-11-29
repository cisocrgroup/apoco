package eval

import (
	"fmt"
	"io"
	"log"

	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/spf13/cobra"
	"gonum.org/v1/gonum/mat"
)

// CMD defines the apoco eval command.
var CMD = &cobra.Command{
	Use:   "eval",
	Short: "Evaluate post-correction models",
}

var flags = struct {
	extensions                    []string
	parameter, model              string
	nocr                          int
	cache, cautious, update, alev bool
}{}

func init() {
	// Eval flags
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
	CMD.PersistentFlags().BoolVarP(&flags.alev, "alignlev", "v", false,
		"align using Levenshtein (matrix) alignment")
	// Subcommands
	CMD.AddCommand(rrCMD, dmCMD, msCMD, ffCMD)
}

type stats struct {
	tn, tp, fn, fp int
}

type typ int

const (
	tp typ = iota
	tn
	fp
	fn
)

func (s *stats) eval(p ml.Predictor, t float64, xs, ys []float64) {
	xlen := len(xs)
	ylen := len(ys)
	x := mat.NewDense(ylen, xlen/ylen, xs)
	y := mat.NewVecDense(ylen, ys)
	ps := p.Predict(x)
	ml.ApplyThreshold(ps, t)
	for i := 0; i < ylen; i++ {
		s.add(y.AtVec(i), ps.AtVec(i))
	}
}

func (s *stats) add(y, p float64) typ {
	if y == ml.True {
		if y == p {
			s.tp++
			return tp
		} else {
			s.fn++
			return fn
		}
	} else {
		if y == p {
			s.tn++
			return tn
		} else {
			s.fp++
			return fp
		}
	}
}

func (s *stats) recall() float64 {
	if s.tp == 0 && s.fn == 0 {
		return 0
	}
	return float64(s.tp) / float64(s.tp+s.fn)
}

func (s *stats) precision() float64 {
	if s.tp == 0 && s.fp == 0 {
		return 0
	}
	return float64(s.tp) / float64(s.tp+s.fp)
}

func (s *stats) f1() float64 {
	p, r := s.precision(), s.recall()
	if p == 0 && r == 0 {
		return 0
	}
	return (2 * p * r) / (p + r)
}

func (s *stats) print(out io.Writer, typ string, nocr int) error {
	f := formater{out: out}
	f.printf("%s/%d tp %d\n", typ, nocr, s.tp)
	f.printf("%s/%d fp %d\n", typ, nocr, s.fp)
	f.printf("%s/%d tn %d\n", typ, nocr, s.tn)
	f.printf("%s/%d fn %d\n", typ, nocr, s.fn)
	f.printf("%s/%d pr %f\n", typ, nocr, s.precision())
	f.printf("%s/%d re %f\n", typ, nocr, s.recall())
	f.printf("%s/%d f1 %f\n", typ, nocr, s.f1())
	return f.err
}

type formater struct {
	out io.Writer
	err error
}

func (f *formater) printf(format string, args ...interface{}) {
	if f.err != nil {
		return
	}
	_, err := fmt.Fprintf(f.out, format, args...)
	f.err = err
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
