package version

import (
	"fmt"
	"os"
	"runtime"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"github.com/spf13/cobra"
)

// CMD defines the apoco version command.
var CMD = &cobra.Command{
	Use:   "version",
	Short: "Print apoco's version",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	fmt.Printf("%s version: %s [%s/%s]\n", os.Args[0], internal.Version, runtime.GOOS, runtime.GOARCH)
}
