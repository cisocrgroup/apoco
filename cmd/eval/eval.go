package eval

import (
	"log"
	"strings"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"git.sr.ht/~flobar/apoco/pkg/apoco/pagexml"
	"git.sr.ht/~flobar/apoco/pkg/apoco/snippets"
	"github.com/spf13/cobra"
)

// CMD defines the apoco eval command.
var CMD = &cobra.Command{
	Use:   "eval",
	Short: "Evaluate models",
}

var flags = struct {
	parameters, extensions, model string
	nocr                          int
	cache, cautious, update       bool
}{}

func init() {
	// Eval flags
	CMD.PersistentFlags().StringVarP(&flags.parameters, "parameters", "P", "config.toml",
		"set the path to configuration file")
	CMD.PersistentFlags().StringVarP(&flags.extensions, "extensions", "e", ".xml",
		"set the input file extensions")
	CMD.PersistentFlags().StringVarP(&flags.model, "model", "m", "",
		"set the model path (overwrites the setting in the configuration file)")
	CMD.PersistentFlags().IntVarP(&flags.nocr, "nocr", "n", 0,
		"set the number of parallel OCRs (overwrites the setting in the configuration file)")
	CMD.PersistentFlags().BoolVarP(&flags.cache, "cache", "c", false,
		"enable caching of profiles (overwrites the setting in the configuration file)")
	// Subcommands
	CMD.AddCommand(rrCMD, dmCMD)
}

func tokenize(ext string, dirs ...string) apoco.StreamFunc {
	if ext == ".xml" {
		return pagexml.TokenizeDirs(ext, dirs...)
	}
	e := snippets.Extensions(strings.FieldsFunc(ext, func(r rune) bool { return r == ',' }))
	return e.Tokenize(dirs...)
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

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
