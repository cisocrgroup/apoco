package eval

import (
	"fmt"
	"io"
	"log"

	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/spf13/cobra"
)

// CMD defines the apoco eval command.
var CMD = &cobra.Command{
	Use:   "eval",
	Short: "Evaluate models",
}

var flags = struct {
	extensions              []string
	parameters, model       string
	nocr                    int
	cache, cautious, update bool
}{}

func init() {
	// Eval flags
	CMD.PersistentFlags().StringVarP(&flags.parameters, "parameters", "P", "config.toml",
		"set the path to the configuration file")
	CMD.PersistentFlags().StringSliceVarP(&flags.extensions, "extensions", "e", []string{".xml"},
		"set the input file extensions")
	CMD.PersistentFlags().StringVarP(&flags.model, "model", "M", "",
		"set the model path (overwrites the setting in the configuration file)")
	CMD.PersistentFlags().IntVarP(&flags.nocr, "nocr", "n", 0,
		"set the number of parallel OCRs (overwrites the setting in the configuration file)")
	CMD.PersistentFlags().BoolVarP(&flags.cache, "cache", "c", false,
		"enable caching of profiles (overwrites the setting in the configuration file)")
	// Subcommands
	CMD.AddCommand(rrCMD, dmCMD)
}

type stats struct {
	tn, tp, fn, fp int
}

func (s *stats) add(y, p float64) {
	if y == ml.True {
		if y == p {
			s.tp++
		} else {
			s.fn++
		}
	} else {
		if y == p {
			s.tn++
		} else {
			s.fp++
		}
	}
}

func (s *stats) recall() float64 {
	return float64(s.tp) / float64(s.tp+s.fn)
}

func (s *stats) precision() float64 {
	return float64(s.tp) / float64(s.tp+s.fp)
}

func (s *stats) f1() float64 {
	return 2 * s.precision() * s.recall() / (s.precision() + s.recall())
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
