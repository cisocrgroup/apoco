package train

import (
	"context"
	"log"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/pagexml"
	"git.sr.ht/~flobar/apoco/pkg/apoco/snippets"
	"github.com/spf13/cobra"
)

// CMD defines the apoco train command.
var CMD = &cobra.Command{
	Use:   "train",
	Short: "Train models",
}

var flags = struct {
	extensions              []string
	parameters, model       string
	nocr                    int
	cache, cautious, update bool
}{}

func init() {
	// Train flags
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
	CMD.PersistentFlags().BoolVarP(&flags.cautious, "cautious", "a", false,
		"use cautious training (overwrites the setting in the configuration file)")
	CMD.PersistentFlags().BoolVarP(&flags.update, "update", "u", false,
		"update the model if it already exists")
	// Subcommands
	CMD.AddCommand(rrCMD, dmCMD)
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
