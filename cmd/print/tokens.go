package print

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/spf13/cobra"
)

var tokensFlags = struct {
	ifgs, extensions []string
	mets             string
	normalize, file  bool
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
	tokensCMD.Flags().BoolVarP(&tokensFlags.file, "file", "f", false, "print file path of tokens")
}

func runTokens(_ *cobra.Command, args []string) {
	var stream []apoco.StreamFunc
	if tokensFlags.normalize {
		stream = append(stream, apoco.Normalize)
	}
	if flags.json {
		stream = append(stream, pjson())
	} else {
		stream = append(stream, cat(tokensFlags.file))
	}
	chk(pipe(context.Background(),
		tokensFlags.mets, tokensFlags.ifgs,
		tokensFlags.extensions, args, stream...))
}

func cat(file bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			if file {
				_, err := fmt.Printf("%s@%s\n", t.File, token2string(t))
				return err
			}
			_, err := fmt.Printf("%s\n", token2string(t))
			return err
		})
	}
}

func token2string(t apoco.T) string {
	ret := make([]string, len(t.Tokens)+1)
	ret[0] = t.ID
	for i, tok := range t.Tokens {
		ret[i+1] = strings.ReplaceAll(tok, " ", "_")
	}
	return strings.Join(ret, " ")
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
