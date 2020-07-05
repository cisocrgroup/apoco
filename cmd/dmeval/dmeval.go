package dmeval

import (
	"context"
	"fmt"
	"log"
	"strings"

	"example.com/apoco/pkg/apoco"
	"example.com/apoco/pkg/apoco/ml"
	"example.com/apoco/pkg/apoco/pagexml"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gonum.org/v1/gonum/mat"
)

func init() {
	CMD.Flags().StringVarP(&flags.mets, "mets", "m", "mets.xml", "set mets file")
	CMD.Flags().StringVarP(&flags.inputFileGrp, "input-file-grp", "I", "", "set input file group")
	CMD.Flags().StringVarP(&flags.parameters, "parameters", "P", "config.json", "set configuration file")
	CMD.Flags().IntVarP(&flags.nocr, "nocr", "n", 0, "set nocr (overwrites setting in the configuration file)")
	CMD.Flags().BoolVarP(&flags.nocache, "nocache", "c", false, "disable caching of profiles (overwrites setting in the configuration file)")
	CMD.Flags().StringVarP(&flags.model, "model", "M", "", "set model path (overwrites setting in the configuration file)")
}

var flags = struct {
	mets, inputFileGrp string
	parameters, model  string
	nocr               int
	nocache            bool
}{}

// CMD defines the apoco train command.
var CMD = &cobra.Command{
	Use:   "dmeval",
	Short: "Evaluate a decision maker model",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	c, err := apoco.ReadConfig(flags.parameters)
	noerr(err)
	c.Overwrite(flags.model, flags.nocr, flags.nocache)
	m, err := apoco.ReadModel(c.Model, c.Ngrams)
	noerr(err)
	lr, fs, err := m.Load("rr", c.Nocr)
	noerr(err)
	g, ctx := errgroup.WithContext(context.Background())
	_ = apoco.Pipe(ctx, g,
		pagexml.Tokenize(flags.mets, strings.Split(flags.inputFileGrp, ",")...),
		apoco.Normalize,
		apoco.FilterShort,
		apoco.ConnectLM(c, m.Ngrams),
		apoco.FilterLexiconEntries,
		apoco.ConnectCandidates,
		apoco.ConnectRankings(lr, fs, c.Nocr),
		evaldm(c, m))
	noerr(g.Wait())
}

func evaldm(c *apoco.Config, m apoco.Model) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
		g.Go(func() error {
			lr, fs, err := m.Load("dm", c.Nocr)
			if err != nil {
				return fmt.Errorf("evaldm: %v", err)
			}
			var xs, ys []float64
			var tokens []apoco.Token
			err = apoco.EachToken(ctx, in, func(t apoco.Token) error {
				vals := fs.Calculate(t, c.Nocr)
				xs = append(xs, vals...)
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
	return ml.Bool(candidate.Suggestion == t.Tokens[len(t.Tokens)-1])
}

func noerr(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
