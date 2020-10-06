package train

import (
	"context"
	"fmt"
	"log"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/finkf/gofiler"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gonum.org/v1/gonum/mat"
)

// rrCMD defines the apoco train rr command.
var rrCMD = &cobra.Command{
	Use:   "rr [DIRS...]",
	Short: "Train an apoco re-ranking model",
	Run:   rrRun,
}

func rrRun(_ *cobra.Command, args []string) {
	c, err := apoco.ReadConfig(flags.parameters)
	chk(err)
	c.Overwrite(flags.model, flags.nocr, flags.cautious, flags.cache)
	m, err := apoco.ReadModel(c.Model, c.Ngrams)
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
		rrTrain(c, m, flags.update))
	chk(g.Wait())
}

func rrTrain(c *apoco.Config, m apoco.Model, update bool) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
		g.Go(func() error {
			lr, fs, err := loadRRModel(c, m, update)
			if err != nil {
				return fmt.Errorf("rrtrain: %v", err)
			}
			var xs, ys []float64
			err = apoco.EachToken(ctx, in, func(t apoco.Token) error {
				xs = fs.Calculate(xs, t, c.Nocr)
				ys = append(ys, rrGT(t))
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
			log.Printf("rrtrain: fitting %d toks, %d feats, nocr=%d, lr=%f, ntrain=%d",
				n, len(xs)/n, c.Nocr, lr.LearningRate, lr.Ntrain)
			lr.Fit(x, y)
			log.Printf("rrtrain: fitted %d toks, %d feats, nocr=%d, lr=%f, ntrain=%d",
				len(ys), len(xs)/len(ys), c.Nocr, lr.LearningRate, lr.Ntrain)
			m.Put("rr", c.Nocr, lr, c.RRFeatures)
			if err := m.Write(c.Model); err != nil {
				return fmt.Errorf("rrtrain: %v", err)
			}
			return nil
		})
		return nil
	}
}

func loadRRModel(c *apoco.Config, m apoco.Model, update bool) (*ml.LR, apoco.FeatureSet, error) {
	if update {
		return m.Get("rr", c.Nocr)
	}
	fs, err := apoco.NewFeatureSet(c.RRFeatures...)
	if err != nil {
		return nil, nil, err
	}
	lr := &ml.LR{
		LearningRate: c.LearningRate,
		Ntrain:       c.Ntrain,
	}
	return lr, fs, nil
}

func rrGT(t apoco.Token) float64 {
	candidate := t.Payload.(*gofiler.Candidate)
	return ml.Bool(candidate.Suggestion == t.Tokens[len(t.Tokens)-1])
}
