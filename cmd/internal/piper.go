package internal

import (
	"context"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/pagexml"
	"git.sr.ht/~flobar/apoco/pkg/apoco/snippets"
)

type Piper struct {
	IFGS, Exts, Dirs []string
	METS             string
}

func (p Piper) Pipe(ctx context.Context, fns ...apoco.StreamFunc) error {
	if len(p.IFGS) > 0 {
		return apoco.Pipe(
			ctx,
			append([]apoco.StreamFunc{pagexml.Tokenize(p.METS, p.IFGS...)}, fns...)...,
		)
	}
	if len(p.Exts) == 1 && p.Exts[0] == ".xml" {
		return apoco.Pipe(
			ctx,
			append([]apoco.StreamFunc{pagexml.TokenizeDirs(p.Exts[0], p.Dirs...)}, fns...)...,
		)
	}
	e := snippets.Extensions(p.Exts)
	return apoco.Pipe(
		ctx,
		append([]apoco.StreamFunc{e.ReadLines(p.Dirs...), e.TokenizeLines}, fns...)...,
	)
}
