package rrtrain

import (
	"context"
	"fmt"
	"log"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/finkf/gofiler"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gonum.org/v1/gonum/mat"
)

func init() {
	flags.Flags.Init(CMD)
	CMD.Flags().IntVarP(&flags.nocr, "nocr", "n", 0, "set nocr (overwrites setting in the configuration file)")
	CMD.Flags().BoolVarP(&flags.cache, "cache", "c", false, "disable caching of profiles (overwrites setting in the configuration file)")
	CMD.Flags().StringVarP(&flags.model, "model", "M", "", "set model path (overwrites setting in the configuration file)")
}

var flags = struct {
	internal.Flags
	model string
	nocr  int
	cache bool
}{}

// CMD defines the apoco train command.
var CMD = &cobra.Command{
	Use:   "rrtrain",
	Short: "Train an apoco re-ranking model",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	c, err := apoco.ReadConfig(flags.Params)
	chk(err)
	c.Overwrite(flags.model, flags.nocr, flags.cache)
	m, err := apoco.ReadModel(c.Model, c.Ngrams)
	chk(err)
	g, ctx := errgroup.WithContext(context.Background())
	_ = apoco.Pipe(ctx, g,
		flags.Flags.Tokenize(),
		apoco.FilterBad(c.Nocr+1), // at least n ocr + ground truth
		apoco.Normalize,
		apoco.FilterShort,
		apoco.ConnectLM(c, m.Ngrams),
		apoco.FilterLexiconEntries,
		apoco.ConnectCandidates,
		rrtrain(c, m))
	chk(g.Wait())
}

func rrtrain(c *apoco.Config, m apoco.Model) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
		g.Go(func() error {
			fs, err := apoco.NewFeatureSet(c.RRFeatures...)
			if err != nil {
				return fmt.Errorf("rrtrain: %v", err)
			}
			var xs, ys []float64
			err = apoco.EachToken(ctx, in, func(t apoco.Token) error {
				xs = fs.Calculate(t, c.Nocr, xs)
				ys = append(ys, gt(t))
				return nil
			})
			if err != nil {
				return fmt.Errorf("rrtrain: %v", err)
			}
			n := len(ys) // number or training tokens
			x := mat.NewDense(n, len(xs)/n, xs)
			y := mat.NewVecDense(n, ys)
			if err := ml.Normalize(x); err != nil {
				return fmt.Errorf("rrtrain: %v", err)
			}
			lr := ml.LR{
				LearningRate: c.LearningRate,
				Ntrain:       c.Ntrain,
			}
			log.Printf("rrtrain: fitting %d tokens, %d features, nocr=%d, lr=%f, ntrain=%d",
				n, len(xs)/n, c.Nocr, lr.LearningRate, lr.Ntrain)
			lr.Fit(x, y)
			m.Put("rr", c.Nocr, &lr, c.RRFeatures)
			if err := m.Write(c.Model); err != nil {
				return fmt.Errorf("rrtrain: %v", err)
			}
			return nil
		})
		return nil
	}
}

func gt(t apoco.Token) float64 {
	candidate := t.Payload.(*gofiler.Candidate)
	return ml.Bool(candidate.Suggestion == t.Tokens[len(t.Tokens)-1])
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
