package cat

import (
	"context"
	"fmt"
	"log"

	"example.com/apoco/cmd/internal"
	"example.com/apoco/pkg/apoco"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func init() {
	flags.Init(CMD)
}

var flags internal.Flags

// CMD defines the apoco cat command.
var CMD = &cobra.Command{
	Use:   "cat",
	Short: "Output tokens",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	g, ctx := errgroup.WithContext(context.Background())
	_ = apoco.Pipe(ctx, g, flags.Tokenize(), cat)
	noerr(g.Wait())
}

func cat(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
	g.Go(func() error {
		return apoco.EachToken(ctx, in, func(t apoco.Token) error {
			fmt.Printf("%s", t)
			return nil
		})
	})
	return nil
}

func noerr(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
