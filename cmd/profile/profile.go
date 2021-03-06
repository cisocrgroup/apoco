package profile

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/spf13/cobra"
)

// Cmd defines the apoco profile command.
var Cmd = &cobra.Command{
	Use:   "profile [DIRS...] OUT",
	Short: "Create profiles of documents",
	Args:  cobra.MinimumNArgs(1),
	Run:   runProfile,
	Long: `
Writes gzipped profiles to OUT. Reads stat tokens from
stdin if no DIRS are given. Otherwise tokens are read from
DIRS.`,
}

var flags = struct {
	extensions  []string
	parameter   string
	corrections bool
}{}

func init() {
	Cmd.PersistentFlags().StringVarP(&flags.parameter, "parameter", "p", "config.toml",
		"set the path to the configuration file")
	Cmd.PersistentFlags().StringSliceVarP(&flags.extensions, "extensions", "e", nil,
		"set the input file extensions")
	Cmd.PersistentFlags().BoolVarP(&flags.corrections, "corrections", "c", false,
		"add corrections to the profiler")
}

func runProfile(_ *cobra.Command, args []string) {
	c, err := internal.ReadConfig(flags.parameter)
	chk(err)
	// If called with only one output file, read stat tokens from stdin.
	if len(args) == 1 {
		chk(apoco.Pipe(
			context.Background(),
			readStoks(os.Stdin, flags.corrections),
			apoco.FilterBad(1),
			apoco.Normalize(),
			apoco.FilterShort(4),
			writeProfile(c, args[len(args)-1]),
		))
		return
	}
	// Otherwise read files from given directories.
	p := internal.Piper{
		Exts: flags.extensions,
		Dirs: args[:len(args)-1],
	}
	chk(p.Pipe(
		context.Background(),
		apoco.FilterBad(1),
		apoco.Normalize(),
		apoco.FilterShort(4),
		writeProfile(c, args[len(args)-1]),
	))
}

func writeProfile(c *internal.Config, name string) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		var ts []apoco.T
		err := apoco.EachToken(ctx, in, func(t apoco.T) error {
			ts = append(ts, t)
			return nil
		})
		if err != nil {
			return err
		}
		profile, err := apoco.RunProfiler(ctx, c.Profiler.Exe, c.Profiler.Config, ts...)
		if err != nil {
			return err
		}
		return apoco.WriteProfile(name, profile)
	}
}

func readStoks(in io.Reader, corrections bool) apoco.StreamFunc {
	return func(ctx context.Context, _ <-chan apoco.T, out chan<- apoco.T) error {
		return eachStok(in, func(stok internal.Stok) error {
			t := apoco.T{
				Tokens: []string{stok.OCR, stok.GT},
			}
			if corrections && !stok.Skipped && stok.Cor {
				t.Cor = stok.Sug
			}
			return apoco.SendTokens(ctx, out, t)
		})
	}
}

func eachStok(in io.Reader, f func(internal.Stok) error) error {
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		// Skip commented lines (lines starting with #).
		if len(scanner.Text()) > 0 && scanner.Text()[0] == '#' {
			continue
		}
		t, err := internal.MakeStokFromLine(scanner.Text())
		if err != nil {
			return err
		}
		if err := f(t); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
