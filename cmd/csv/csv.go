package csv

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/spf13/cobra"
)

// CMD defines the apoco train command.
var CMD = &cobra.Command{
	Use:   "csv",
	Short: "Extract training-features to csv",
}

var flags = struct {
	extensions            []string
	parameter, model, out string
	nocr, bufs            int
	cache, alev           bool
}{}

const bufs int = 64 * 1024

func init() {
	// Train flags
	CMD.PersistentFlags().StringVarP(&flags.parameter, "parameter", "p", "config.toml",
		"set the path to the configuration file")
	CMD.PersistentFlags().StringSliceVarP(&flags.extensions, "extensions", "e", []string{".xml"},
		"set the input file extensions")
	CMD.PersistentFlags().IntVarP(&flags.nocr, "nocr", "n", 0,
		"set the number of parallel OCRs (overwrites the setting in the configuration file)")
	CMD.PersistentFlags().StringVarP(&flags.model, "model", "M", "",
		"set the model path (overwrites the setting in the configuration file)")
	CMD.PersistentFlags().BoolVarP(&flags.cache, "cache", "c", false,
		"enable caching of profiles (overwrites the setting in the configuration file)")
	CMD.PersistentFlags().BoolVarP(&flags.alev, "alignlev", "v", false,
		"align using Levenshtein (matrix) alignment")
	CMD.PersistentFlags().StringVarP(&flags.out, "out", "o", "out.csv", "set output file")

	// Subcommands
	CMD.AddCommand(rrCMD, dmCMD) //, msCMD)
}

func csv(features []string, nocr int, gt func(apoco.T) (float64, bool)) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		fail := func(err error) error {
			return fmt.Errorf("csv: %v", err)
		}

		// Create feature set.
		fs, err := apoco.NewFeatureSet(features...)
		if err != nil {
			return fail(err)
		}

		// Open buffered output file writer.
		f, err := os.Create(flags.out)
		if err != nil {
			return fail(err)
		}
		defer f.Close()
		w := bufio.NewWriterSize(f, bufs)

		// Write feature weights and ground-truth to the file.
		data := make([]float64, 0, len(fs)+1)
		err = apoco.EachToken(ctx, in, func(t apoco.T) error {
			gt, use := gt(t)
			if !use {
				return nil
			}
			data = fs.Calculate(data, t, nocr)
			data = append(data, gt)
			if err := write(w, data); err != nil {
				return err
			}
			data = data[0:0]
			return nil
		})
		if err != nil {
			return fail(err)
		}
		return nil
	}
}

func write(w io.Writer, xs []float64) error {
	var buf []byte
	for i := range xs {
		if i != 0 {
			buf = append(buf, ',')
		}
		buf = strconv.AppendFloat(buf, xs[i], 'g', -1, 64)

	}
	buf = append(buf, '\n')
	_, err := w.Write(buf)
	return err
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
