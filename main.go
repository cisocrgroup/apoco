package main

import (
	"git.sr.ht/~flobar/apoco/cmd/align"
	"git.sr.ht/~flobar/apoco/cmd/cat"
	"git.sr.ht/~flobar/apoco/cmd/charset"
	"git.sr.ht/~flobar/apoco/cmd/correct"
	"git.sr.ht/~flobar/apoco/cmd/eval"
	"git.sr.ht/~flobar/apoco/cmd/printmodel"
	"git.sr.ht/~flobar/apoco/cmd/protocol"
	"git.sr.ht/~flobar/apoco/cmd/stats"
	"git.sr.ht/~flobar/apoco/cmd/train"
	"git.sr.ht/~flobar/apoco/cmd/version"
	"github.com/spf13/cobra"
)

var root = &cobra.Command{
	Use:   "apoco",
	Short: "A̲utomatic p̲o̲st c̲o̲rrection of (historical) OCR",
}

func init() {
	root.AddCommand(
		align.CMD,
		cat.CMD,
		charset.CMD,
		correct.CMD,
		eval.CMD,
		printmodel.CMD,
		protocol.CMD,
		stats.CMD,
		train.CMD,
		version.CMD,
	)
}

func main() {
	root.Execute()
}
