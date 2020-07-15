package main

import (
	"example.com/apoco/cmd/align"
	"example.com/apoco/cmd/cat"
	"example.com/apoco/cmd/correct"
	"example.com/apoco/cmd/dmeval"
	"example.com/apoco/cmd/dmtrain"
	"example.com/apoco/cmd/rreval"
	"example.com/apoco/cmd/rrtrain"
	"example.com/apoco/cmd/stats"
	"example.com/apoco/cmd/version"
	"github.com/spf13/cobra"
)

var root = &cobra.Command{
	Use:   "apoco",
	Short: "A̲utomatic p̲o̲st c̲o̲rrection of (historical) OCR",
}

func init() {
	root.AddCommand(align.CMD)
	root.AddCommand(cat.CMD)
	root.AddCommand(correct.CMD)
	root.AddCommand(dmeval.CMD)
	root.AddCommand(dmtrain.CMD)
	root.AddCommand(rreval.CMD)
	root.AddCommand(rrtrain.CMD)
	root.AddCommand(stats.CMD)
	root.AddCommand(version.CMD)
}

func main() {
	root.Execute()
}
