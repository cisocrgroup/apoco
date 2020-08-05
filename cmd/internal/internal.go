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
	Dirs   string // Commands-separated list of input directories
	Params string // Path to the configuration file
}

// Init initializes the standard commandline arguments for the given
// subcommand.
func (flags *Flags) Init(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&flags.METS, "mets", "m", "mets.xml", "set path to mets file")
	cmd.Flags().StringVarP(&flags.IFGs, "input-file-grp", "I", "", "set input file groups")
	cmd.Flags().StringVarP(&flags.Exts, "extensions", "E", "", "set snippet file extensions")
	cmd.Flags().StringVarP(&flags.Dirs, "dirs", "D", "", "set input directories")
	cmd.Flags().StringVarP(&flags.Params, "parameters", "P", "config.json", "set path to configuration file")
}

// Tokenize tokenizes input.  If len(Exts) == 0, tokens are read from
// the according file groups files from the mets file.  Otherwise if
// len(Exts) > 0, tokens are read from the snippets with the given
// extensions from the files withtin the given directories.
func (flags *Flags) Tokenize() apoco.StreamFunc {
	if len(flags.Exts) == 0 {
		ifgs := strings.Split(flags.IFGs, ",")
		return pagexml.Tokenize(flags.METS, ifgs...)
	}
	exts := strings.Split(flags.Exts, ",")
	dirs := strings.Split(flags.Dirs, ",")
	e := snippets.Extensions(exts)
	return e.Tokenize(dirs...)
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
