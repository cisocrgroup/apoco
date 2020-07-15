package internal

import (
	"example.com/apoco/pkg/apoco"
	"example.com/apoco/pkg/apoco/pagexml"
	"example.com/apoco/pkg/apoco/snippets"
)

// Tokenize tokenizes input.  If len(exts) == 0, fgs is assumed to be
// a list of input file groups to be read from the given mets file
// path.  Otherwise if len(exts) > 0, fgs is assumed to be a list of
// directories from which the snippets with the given file extensions
// are read.
func Tokenize(mets string, exts, fgs []string) apoco.StreamFunc {
	if len(exts) == 0 {
		return pagexml.Tokenize(mets, fgs...)
	}
	e := snippets.Extensions(exts)
	return e.Tokenize(fgs...)
}
