package print

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/spf13/cobra"
)

var tokensFlags = struct {
	ifgs, extensions []string
	mets             string
	normalize, gt    bool
}{}

// CMD defines the apoco cat command.
var tokensCMD = &cobra.Command{
	Use:   "tokens [DIRS...]",
	Short: "Output tokens to stdout",
	Run:   runTokens,
}

func init() {
	tokensCMD.Flags().StringSliceVarP(&tokensFlags.ifgs, "input-file-grp", "I", nil, "set input file groups")
	tokensCMD.Flags().StringSliceVarP(&tokensFlags.extensions, "extensions", "e", []string{".xml"},
		"set input file extensions")
	tokensCMD.Flags().StringVarP(&tokensFlags.mets, "mets", "m", "mets.xml", "set path to the mets file")
	tokensCMD.Flags().BoolVarP(&tokensFlags.normalize, "normalize", "N", false, "normalize tokens")
	tokensCMD.Flags().BoolVarP(&tokensFlags.gt, "gt", "g", false, "enable ground-truth data")
}

func runTokens(_ *cobra.Command, args []string) {
	var stream []apoco.StreamFunc
	if tokensFlags.normalize {
		stream = append(stream, apoco.Normalize())
	}
	if flags.json {
		stream = append(stream, pjson())
	} else {
		stream = append(stream, cat(tokensFlags.gt))
	}
	p := internal.Piper{
		METS: tokensFlags.mets,
		IFGS: tokensFlags.ifgs,
		Exts: tokensFlags.extensions,
		Dirs: args,
	}
	chk(p.Pipe(context.Background(), stream...))
}

func cat(gt bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			stok := internal.MakeStokFromT(t, gt)
			_, err := fmt.Println(stok.String())
			return err
		})
	}
}

func pjson() apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		var tokens []apoco.T
		err := apoco.EachToken(ctx, in, func(t apoco.T) error {
			tokens = append(tokens, t)
			return nil
		})
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(tokens)
	}
}
