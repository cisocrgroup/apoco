package print

import (
	"log"

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
	CMD.AddCommand(statsCMD, tokensCMD, modelCMD, protocolCMD, profileCMD)
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
