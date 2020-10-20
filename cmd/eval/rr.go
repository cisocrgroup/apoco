package eval

import (
	"context"
	"fmt"
	"os"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/finkf/gofiler"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gonum.org/v1/gonum/mat"
)

// rrCMD defines the apoco eval rr command.
var rrCMD = &cobra.Command{
	Use:   "rr [DIR...]",
	Short: "Evaluate an apoco re-ranking model",
	Run:   rrRun,
}

func rrRun(_ *cobra.Command, args []string) {
	c, err := apoco.ReadConfig(flags.parameters)
	chk(err)
	c.Overwrite(flags.model, flags.nocr, false, flags.cache)
	m, err := apoco.ReadModel(c.Model, c.Ngrams)
	chk(err)
	g, ctx := errgroup.WithContext(context.Background())
	_ = apoco.Pipe(ctx, g,
		tokenize(flags.extensions, args),
		apoco.FilterBad(c.Nocr+1), // at least n ocr + ground truth
		apoco.Normalize,
		apoco.FilterShort,
		apoco.ConnectLM(c, m.Ngrams),
		apoco.FilterLexiconEntries,
		apoco.ConnectCandidates,
		rrEval(c, m))
	chk(g.Wait())
}

func rrEval(c *apoco.Config, m apoco.Model) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
		g.Go(func() error {
			lr, fs, err := m.Get("rr", c.Nocr)
			if err != nil {
				return fmt.Errorf("rreval: %v", err)
			}
			var xs, ys []float64
			err = apoco.EachToken(ctx, in, func(t apoco.Token) error {
				xs = fs.Calculate(xs, t, c.Nocr)
				ys = append(ys, rrGT(t))
				return nil
			})
			if err != nil {
				return fmt.Errorf("rreval: %v", err)
			}
			n := len(ys)
			x := mat.NewDense(n, len(xs)/n, xs)
			y := mat.NewVecDense(n, ys)
			p := lr.Predict(x, 0.5)
			var s stats
			for i := 0; i < n; i++ {
				s.add(y.AtVec(i), p.AtVec(i))
			}
			return s.print(os.Stdout, "rr", c.Nocr)
		})
		return nil
	}
}

func rrGT(t apoco.Token) float64 {
	candidate := t.Payload.(*gofiler.Candidate)
	return ml.Bool(candidate.Suggestion == t.Tokens[len(t.Tokens)-1])
}
