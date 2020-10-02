package internal

import (
	"fmt"
	"path/filepath"
	"strings"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/pagexml"
	"git.sr.ht/~flobar/apoco/pkg/apoco/snippets"
	"github.com/spf13/cobra"
)

// apoco version
const Version = "v0.0.1"

// Flags is used to define the standard command-line parameters for
// apoco sub commands.
type Flags struct {
	METS   string // METS file path
	IFGs   string // Comma-separated list of input file groups
	Exts   string // Comma-separated list of file extensions
	Params string // Path to the configuration file
}

// Init initializes the standard commandline arguments for the given
// subcommand.
func (flags *Flags) Init(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&flags.METS, "mets", "m", "mets.xml", "set path to mets file")
	cmd.Flags().StringVarP(&flags.IFGs, "input-file-grp", "I", "", "set input file groups")
	cmd.Flags().StringVarP(&flags.Exts, "extensions", "E", "", "set snippet file extensions")
	cmd.Flags().StringVarP(&flags.Params, "parameters", "P", "config.json", "set path to configuration file")
}

// Tokenize tokenizes input.  The directories or input file groups are
// read from the args and in the case of input file groups
// additionally from the comma seperated list of the
// -I/--input-file-grp command line argument.  If len(Exts) == 0,
// tokens are read from the according input file groups from the mets
// file.  Otherwise if len(Exts) > 0, tokens are read from the
// snippets with the given extensions from the files withtin the given
// input directories.
func (flags *Flags) Tokenize(args []string) apoco.StreamFunc {
	if len(flags.Exts) == 0 {
		ifgs := append(args, strings.Split(flags.IFGs, ",")...)
		return pagexml.Tokenize(flags.METS, ifgs...)
	}
	exts := strings.Split(flags.Exts, ",")
	e := snippets.Extensions(exts)
	return e.Tokenize(args...)
}

// IDFromFilePath returns the proper id given a file path and a file
// group.
func IDFromFilePath(path, fg string) string {
	// Use base path and remove file extensions.
	path = filepath.Base(path)
	path = path[0 : len(path)-len(filepath.Ext(path))]
	// Split everything after the last `_`.
	splits := strings.Split(path, "_")
	return fmt.Sprintf("%s_%s", fg, splits[len(splits)-1])
}
