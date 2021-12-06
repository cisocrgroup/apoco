package print

import (
	"log"

	"github.com/spf13/cobra"
)

// Cmd defines the apoco print command.
var Cmd = &cobra.Command{
	Use:   "print",
	Short: "Print out information",
}

var flags = struct {
	json bool
}{}

func init() {
	Cmd.PersistentFlags().BoolVarP(&flags.json, "json", "J", false, "set json output")
	// Subcommands
	Cmd.AddCommand(statsCmd, tokensCmd, modelCmd, protocolCmd, profileCmd, charsetCmd,
		typesCmd, trigramsCmd)
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
