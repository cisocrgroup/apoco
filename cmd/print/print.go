package print

import (
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

func tokenize(mets string, ifgs, exts, args []string) apoco.StreamFunc {
	if len(ifgs) != 0 {
		return pagexml.Tokenize(mets, ifgs...)
	}
	if len(exts) == 1 && exts[0] == ".xml" {
		return pagexml.TokenizeDirs(exts[0], args...)
	}
	e := snippets.Extensions(exts)
	return e.Tokenize(args...)
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
