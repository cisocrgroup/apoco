package eval

import (
	"context"
	"fmt"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gonum.org/v1/gonum/mat"
)

// dmCMD defines the apoco train command.
var dmCMD = &cobra.Command{
	Use:   "dm  [DIRS...]",
	Short: "Evaluate a decision maker model",
	Run:   dmRun,
}

func dmRun(_ *cobra.Command, args []string) {
	c, err := apoco.ReadConfig(flags.parameters)
	chk(err)
	c.Overwrite(flags.model, flags.nocr, false, flags.cache)
	m, err := apoco.ReadModel(c.Model, c.Ngrams)
	chk(err)
	lr, fs, err := m.Get("rr", c.Nocr)
	chk(err)
	g, ctx := errgroup.WithContext(context.Background())
	_ = apoco.Pipe(ctx, g,
		tokenize(flags.extensions, args...),
		apoco.FilterBad(c.Nocr+1), // at least n ocr + ground truth
		apoco.Normalize,
		apoco.FilterShort,
		apoco.ConnectLM(c, m.Ngrams),
		apoco.FilterLexiconEntries,
		apoco.ConnectCandidates,
		apoco.ConnectRankings(lr, fs, c.Nocr),
		dmEval(c, m))
	chk(g.Wait())
}

func dmEval(c *apoco.Config, m apoco.Model) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
		g.Go(func() error {
			lr, fs, err := m.Get("dm", c.Nocr)
			if err != nil {
				return fmt.Errorf("evaldm: %v", err)
			}
			var xs, ys []float64
			var tokens []apoco.Token
			err = apoco.EachToken(ctx, in, func(t apoco.Token) error {
				xs = fs.Calculate(xs, t, c.Nocr)
				ys = append(ys, dmGT(t))
				tokens = append(tokens, t)
				return nil
			})
			if err != nil {
				return fmt.Errorf("evaldm: %v", err)
			}
			n := len(ys)
			x := mat.NewDense(n, len(xs)/n, xs)
			y := mat.NewVecDense(n, ys)
			p := lr.Predict(x, 0.5)
			var s stats
			for i := 0; i < n; i++ {
				// cor := tokens[i].Payload.([]apoco.Ranking)[0].Candidate.Suggestion
				// mocr := tokens[i].Tokens[0]
				s.add(y.AtVec(i), p.AtVec(i))
			}
			fmt.Printf("dm,tp,%d,%d\n", c.Nocr, s.tp)
			fmt.Printf("dm,fp,%d,%d\n", c.Nocr, s.fp)
			fmt.Printf("dm,tn,%d,%d\n", c.Nocr, s.tn)
			fmt.Printf("dm,fn,%d,%d\n", c.Nocr, s.fn)
			fmt.Printf("dm,pr,%d,%f\n", c.Nocr, s.precision())
			fmt.Printf("dm,re,%d,%f\n", c.Nocr, s.recall())
			fmt.Printf("dm,f1,%d,%f\n", c.Nocr, s.f1())
			return nil
		})
		return nil
	}
}

func dmGT(t apoco.Token) float64 {
	candidate := t.Payload.([]apoco.Ranking)[0].Candidate
	gt := t.Tokens[len(t.Tokens)-1]
	// return ml.Bool(candidate.Suggestion == gt && t.Tokens[0] != gt)
	return ml.Bool(candidate.Suggestion == gt)
}
