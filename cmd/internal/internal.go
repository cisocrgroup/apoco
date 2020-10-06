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

// Version defines the version of apoco.
const Version = "v0.0.2"

// Flags is used to define the standard command-line parameters for
// apoco sub commands.
type Flags struct {
	ifgs   string // Comma-separated list of input file groups
	exts   string // Comma-separated list of file extensions
	METS   string // METS file path
	Params string // Path to the configuration file
}

// Init initializes the standard commandline arguments for the given
// subcommand.
func (flags *Flags) Init(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&flags.METS, "mets", "m", "mets.xml", "set path to mets file")
	cmd.Flags().StringVarP(&flags.ifgs, "input-file-grp", "I", "", "set input file groups")
	cmd.Flags().StringVarP(&flags.exts, "extensions", "E", "", "set snippet file extensions")
	cmd.Flags().StringVarP(&flags.Params, "parameters", "P", "config.json", "set path to configuration file")
}

// IFGs returns the list of input file groups.
func (flags *Flags) IFGs() []string {
	return strings.FieldsFunc(flags.ifgs, func(r rune) bool { return r == ',' })
}

// Extensions return sthe list of file extensions.
func (flags *Flags) Extensions() []string {
	return strings.FieldsFunc(flags.exts, func(r rune) bool { return r == ',' })
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
	if flags.exts == "" {
		return pagexml.Tokenize(flags.METS, append(args, flags.IFGs()...)...)
	}
	e := snippets.Extensions(flags.Extensions())
	return e.Tokenize(args...)
}

// IDFromFilePath generates an id based on the file group and the file
// path.
func IDFromFilePath(path, fg string) string {
	// Use base path and remove file extensions.
	path = filepath.Base(path)
	path = path[0 : len(path)-len(filepath.Ext(path))]
	// Split everything after the last `_`.
	splits := strings.Split(path, "_")
	return fmt.Sprintf("%s_%s", fg, splits[len(splits)-1])
}
