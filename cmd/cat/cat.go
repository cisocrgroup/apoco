package cat

import (
	"context"
	"fmt"
	"log"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func init() {
	flags.Init(CMD)
}

var flags internal.Flags

// CMD defines the apoco cat command.
var CMD = &cobra.Command{
	Use:   "cat [INPUT...]",
	Short: "Output tokens",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	g, ctx := errgroup.WithContext(context.Background())
	_ = apoco.Pipe(ctx, g, flags.Tokenize(args), apoco.Normalize, cat)
	chk(g.Wait())
}

func cat(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
	g.Go(func() error {
		return apoco.EachToken(ctx, in, func(t apoco.Token) error {
			_, err := fmt.Printf("%s\n", t)
			return err
		})
	})
	return nil
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
