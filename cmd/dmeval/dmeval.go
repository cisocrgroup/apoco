package dmeval

import (
	"context"
	"fmt"
	"log"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
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
	Use:   "dmeval",
	Short: "Evaluate a decision maker model",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	c, err := apoco.ReadConfig(flags.Params)
	chk(err)
	c.Overwrite(flags.model, flags.nocr, false, flags.cache)
	m, err := apoco.ReadModel(c.Model, c.Ngrams)
	chk(err)
	lr, fs, err := m.Get("rr", c.Nocr)
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
		apoco.ConnectRankings(lr, fs, c.Nocr),
		evaldm(c, m))
	chk(g.Wait())
}

func evaldm(c *apoco.Config, m apoco.Model) apoco.StreamFunc {
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
				ys = append(ys, gt(t))
				tokens = append(tokens, t)
				return nil
			})
			if err != nil {
				return fmt.Errorf("evaldm: %v", err)
			}
			runStats(lr, xs, ys, tokens, c.Nocr)
			return nil
		})
		return nil
	}
}

type stats struct {
	tn, tp, fn, fp int
}

func (s *stats) add(y, p float64) {
	if y == ml.True {
		if y == p {
			s.tp++
		} else {
			s.fn++
		}
	} else {
		if y == p {
			s.tn++
		} else {
			s.fp++
		}
	}
}

func (s *stats) recall() float64 {
	return float64(s.tp) / float64(s.tp+s.fn)
}

func (s *stats) precision() float64 {
	return float64(s.tp) / float64(s.tp+s.fp)
}

func (s *stats) f1() float64 {
	return 2 * s.precision() * s.recall() / (s.precision() + s.recall())
}

func runStats(lr *ml.LR, xs, ys []float64, tokens []apoco.Token, nocr int) {
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
	fmt.Printf("dm,tp,%d,%d\n", nocr, s.tp)
	fmt.Printf("dm,fp,%d,%d\n", nocr, s.fp)
	fmt.Printf("dm,tn,%d,%d\n", nocr, s.tn)
	fmt.Printf("dm,fn,%d,%d\n", nocr, s.fn)
	fmt.Printf("dm,pr,%d,%f\n", nocr, s.precision())
	fmt.Printf("dm,re,%d,%f\n", nocr, s.recall())
	fmt.Printf("dm,f1,%d,%f\n", nocr, s.f1())
}

func gt(t apoco.Token) float64 {
	candidate := t.Payload.([]apoco.Ranking)[0].Candidate
	gt := t.Tokens[len(t.Tokens)-1]
	// return ml.Bool(candidate.Suggestion == gt && t.Tokens[0] != gt)
	return ml.Bool(candidate.Suggestion == gt)
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
