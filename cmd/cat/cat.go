package cat

import (
	"context"
	"fmt"
	"log"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/pagexml"
	"git.sr.ht/~flobar/apoco/pkg/apoco/snippets"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var flags = struct {
	ifgs, extensions []string
	mets             string
	normalize, file  bool
}{}

// CMD defines the apoco cat command.
var CMD = &cobra.Command{
	Use:   "cat [DIRS...]",
	Short: "Output tokens to stdout",
	Run:   run,
}

func init() {
	CMD.Flags().StringSliceVarP(&flags.ifgs, "input-file-grp", "I", nil, "set input file groups")
	CMD.Flags().StringSliceVarP(&flags.extensions, "extensions", "e", []string{".xml"},
		"set input file extensions")
	CMD.Flags().StringVarP(&flags.mets, "mets", "m", "mets.xml", "set path to the mets file")
	CMD.Flags().BoolVarP(&flags.normalize, "normalize", "N", false, "normalize tokens")
	CMD.Flags().BoolVarP(&flags.file, "file", "f", false, "print file path of tokens")
}

func run(_ *cobra.Command, args []string) {
	g, ctx := errgroup.WithContext(context.Background())
	if flags.normalize {
		_ = apoco.Pipe(ctx, g,
			tokenize(flags.mets, flags.ifgs, flags.extensions, args), apoco.Normalize, cat(flags.file))
	} else {
		_ = apoco.Pipe(ctx, g, tokenize(flags.mets, flags.ifgs, flags.extensions, args), cat(flags.file))
	}
	chk(g.Wait())
}

func cat(file bool) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
		g.Go(func() error {
			return apoco.EachToken(ctx, in, func(t apoco.Token) error {
				if file {
					_, err := fmt.Printf("%s@%s\n", t.File, t)
					return err
				} else {
					_, err := fmt.Printf("%s\n", t)
					return err
				}
			})
		})
		return nil
	}
}

func tokenize(mets string, ifgs, exts, args []string) apoco.StreamFunc {
	if len(ifgs) != 0 {
		return pagexml.Tokenize(mets, ifgs...)
	}
	if len(exts) == 1 && exts[0] == ".xml" {
		return pagexml.TokenizeDirs(exts[0], args...)
	}
	e := snippets.Extensions(exts)
	return e.Tokenize(args...)
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
