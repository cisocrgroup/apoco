package main

import (
	"os"
	"strings"

	"git.sr.ht/~flobar/apoco/cmd/align"
	"git.sr.ht/~flobar/apoco/cmd/correct"
	"git.sr.ht/~flobar/apoco/cmd/eval"
	"git.sr.ht/~flobar/apoco/cmd/print"
	"git.sr.ht/~flobar/apoco/cmd/profile"
	"git.sr.ht/~flobar/apoco/cmd/train"
	"git.sr.ht/~flobar/apoco/cmd/version"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/spf13/cobra"
)

var root = &cobra.Command{
	Use:   "apoco",
	Short: "A̲utomatic p̲o̲st c̲o̲rrection of (historical) OCR",
}

var logLevel string

func init() {
	root.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "INFO", "set log level")
	root.AddCommand(
		align.CMD,
		correct.CMD,
		eval.CMD,
		print.CMD,
		profile.CMD,
		train.CMD,
		version.CMD,
	)
}

func main() {
	root.ParseFlags(os.Args)
	apoco.SetLog(strings.ToLower(logLevel) == "debug")
	root.Execute()
}
