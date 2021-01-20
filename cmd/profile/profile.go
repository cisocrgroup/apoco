package profile

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/pagexml"
	"git.sr.ht/~flobar/apoco/pkg/apoco/snippets"
	"github.com/finkf/gofiler"
	"github.com/spf13/cobra"
)

// CMD defines the apoco profile command.
var CMD = &cobra.Command{
	Use:   "profile DIR [DIRS...] OUT",
	Short: "Create profiles of documents",
	Args:  cobra.MinimumNArgs(2),
	Run:   runProfile,
}

var flags = struct {
	extensions []string
	parameters string
}{}

func init() {
	CMD.PersistentFlags().StringVarP(&flags.parameters, "parameters", "P", "config.toml",
		"set the path to the configuration file")
	CMD.PersistentFlags().StringSliceVarP(&flags.extensions, "extensions", "e", []string{".xml"},
		"set the input file extensions")
}

func runProfile(_ *cobra.Command, args []string) {
	c, err := apoco.ReadConfig(flags.parameters)
	chk(err)
	chk(pipe(context.Background(), flags.extensions, args[:len(args)-1],
		apoco.Normalize,
		apoco.FilterShort(4),
		writeProfile(c, args[len(args)-1]),
	))
}

func writeProfile(c *apoco.Config, name string) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		var ts []apoco.T
		err := apoco.EachToken(ctx, in, func(t apoco.T) error {
			ts = append(ts, t)
			return nil
		})
		if err != nil {
			return err
		}
		var lm apoco.LanguageModel
		if err := lm.LoadProfile(ctx, c.ProfilerBin, c.ProfilerConfig, false, ts...); err != nil {
			return err
		}
		return writeJSON(name, lm.Profile)
	}
}

func writeJSON(name string, profile gofiler.Profile) error {
	out, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("write json %s: %v", name, err)
	}
	defer out.Close()
	w := gzip.NewWriter(out)
	defer w.Close()
	if err := json.NewEncoder(w).Encode(profile); err != nil {
		return fmt.Errorf("write json %s: %v", name, err)
	}
	return nil
}

func pipe(ctx context.Context, exts, dirs []string, fns ...apoco.StreamFunc) error {
	if len(exts) == 1 && exts[0] == ".xml" {
		fns = append([]apoco.StreamFunc{pagexml.TokenizeDirs(exts[0], dirs...)}, fns...)
	} else {
		e := snippets.Extensions(exts)
		fns = append([]apoco.StreamFunc{e.ReadLines(dirs...), e.TokenizeLines}, fns...)
	}
	return apoco.Pipe(ctx, fns...)
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
