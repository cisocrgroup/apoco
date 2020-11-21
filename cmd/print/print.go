package print

import (
	"context"
	"log"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/pagexml"
	"git.sr.ht/~flobar/apoco/pkg/apoco/snippets"
	"github.com/spf13/cobra"
)

// CMD defines the apoco print command.
var CMD = &cobra.Command{
	Use:   "print",
	Short: "Print out information",
}

var flags = struct {
	json bool
}{}

func init() {
	CMD.PersistentFlags().BoolVarP(&flags.json, "json", "J", false, "set json output")
	// Subcommands
	CMD.AddCommand(statsCMD, tokensCMD, modelCMD)
}

func pipe(ctx context.Context, mets string, ifgs, exts, dirs []string, fns ...apoco.StreamFunc) error {
	if len(ifgs) != 0 {
		fns = append([]apoco.StreamFunc{pagexml.Tokenize(mets, ifgs...)}, fns...)
	} else if len(exts) == 1 && exts[0] == ".xml" {
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
