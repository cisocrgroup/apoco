package version

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var version = "v0.0.0"

// CMD defines the apoco version command.
var CMD = &cobra.Command{
	Use:   "version",
	Short: "Print apoco's version",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	fmt.Printf("apoco version: %s [%s/%s]\n", version, runtime.GOOS, runtime.GOARCH)
}
