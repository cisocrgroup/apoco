package dmtrain

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
	CMD.Flags().BoolVarP(&flags.cautious, "cautious", "C", false, "cautious dm tranining (overwrites setting in the configuration file)")
	CMD.Flags().StringVarP(&flags.model, "model", "M", "", "set model path (overwrites setting in the configuration file)")
	CMD.Flags().BoolVarP(&flags.update, "update", "u", false, "update existing model")
}

var flags = struct {
	internal.Flags
	model    string
	nocr     int
	cache    bool
	cautious bool
	update   bool
}{}

// CMD defines the apoco train command.
var CMD = &cobra.Command{
	Use:   "dmtrain",
	Short: "Train a decision maker model",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	c, err := apoco.ReadConfig(flags.Params)
	chk(err)
	c.Overwrite(flags.model, flags.nocr, flags.cautious, flags.cache)
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
		traindm(c, m, flags.update))
	chk(g.Wait())
}

func traindm(c *apoco.Config, m apoco.Model, update bool) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
		g.Go(func() error {
			lr, fs, err := load(c, m, update)
			if err != nil {
				return fmt.Errorf("traindm: %v", err)
			}
			var xs, ys []float64
			err = apoco.EachToken(ctx, in, func(t apoco.Token) error {
				if !use(t, c.Cautious) {
					return nil
				}
				xs = fs.Calculate(xs, t, c.Nocr)
				ys = append(ys, gt(t))
				return nil
			})
			if err != nil {
				return fmt.Errorf("traindm: %v", err)
			}
			x := mat.NewDense(len(ys), len(xs)/len(ys), xs)
			y := mat.NewVecDense(len(ys), ys)
			if err := ml.Normalize(x); err != nil {
				return fmt.Errorf("traindm: %v", err)
			}
			log.Printf("dmtrain: fitting %d toks, %d feats, nocr=%d, lr=%f, ntrain=%d",
				len(ys), len(xs)/len(ys), c.Nocr, lr.LearningRate, lr.Ntrain)
			lr.Fit(x, y)
			log.Printf("dmtrain: fitted %d toks, %d feats, nocr=%d, lr=%f, ntrain=%d",
				len(ys), len(xs)/len(ys), c.Nocr, lr.LearningRate, lr.Ntrain)
			m.Put("dm", c.Nocr, lr, c.DMFeatures)
			if err := m.Write(c.Model); err != nil {
				return fmt.Errorf("traindm: %v", err)
			}
			return nil
		})
		return nil
	}
}

func load(c *apoco.Config, m apoco.Model, update bool) (*ml.LR, apoco.FeatureSet, error) {
	if update {
		return m.Get("dm", c.Nocr)
	}
	fs, err := apoco.NewFeatureSet(c.DMFeatures...)
	if err != nil {
		return nil, nil, err
	}
	lr := &ml.LR{
		LearningRate: c.LearningRate,
		Ntrain:       c.Ntrain,
	}
	return lr, fs, nil
}

func gt(t apoco.Token) float64 {
	candidate := t.Payload.([]apoco.Ranking)[0].Candidate
	gt := t.Tokens[len(t.Tokens)-1]
	//return ml.Bool(candidate.Suggestion == gt && t.Tokens[0] != gt)
	return ml.Bool(candidate.Suggestion == gt)
}

func use(t apoco.Token, cautious bool) bool {
	if cautious {
		return true
	}
	ocr := t.Tokens[0]
	gt := t.Tokens[len(t.Tokens)-1]
	if gt != ocr {
		return t.Payload.([]apoco.Ranking)[0].Candidate.Suggestion == gt
	}
	return true
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
