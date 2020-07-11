package version

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = ""

// CMD defines the apoco version command.
var CMD = &cobra.Command{
	Use:   "version",
	Short: "Print apoco's version",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	fmt.Printf("apoco version: %s\n", version)
}
