package profile

import "github.com/spf13/cobra"

// CMD defines the apoco profile command.
var CMD = &cobra.Command{
	Use:   "profile",
	Short: "Create profiles of documents",
}

var flags = struct {
	extensions []string
	parameters string
}{}

func init() {
	// Train flags
	CMD.PersistentFlags().StringVarP(&flags.parameters, "parameters", "P", "config.toml",
		"set the path to the configuration file")
	CMD.PersistentFlags().StringSliceVarP(&flags.extensions, "extensions", "e", []string{".xml"},
		"set the input file extensions")
}
