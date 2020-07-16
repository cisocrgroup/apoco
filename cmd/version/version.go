package version

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

var version = "v0.0.1"

// CMD defines the apoco version command.
var CMD = &cobra.Command{
	Use:   "version",
	Short: "Print apoco's version",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	fmt.Printf("%s version: %s [%s/%s]\n", os.Args[0], version, runtime.GOOS, runtime.GOARCH)
}
