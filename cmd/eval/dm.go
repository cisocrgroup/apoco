package eval

import (
	"context"
	"fmt"
	"os"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/spf13/cobra"
	"gonum.org/v1/gonum/mat"
)

// dmCMD defines the apoco train command.
var dmCMD = &cobra.Command{
	Use:   "dm  [DIRS...]",
	Short: "Evaluate a decision maker model",
	Run:   dmRun,
}

func dmRun(_ *cobra.Command, args []string) {
	c, err := internal.ReadConfig(flags.parameter)
	chk(err)
	c.Overwrite(flags.model, "", flags.nocr, flags.cache, false)
	m, err := apoco.ReadModel(c.Model, c.Ngrams)
	chk(err)
	lr, fs, err := m.Get("rr", c.Nocr)
	chk(err)
	p := internal.Piper{
		Exts: flags.extensions,
		Dirs: args,
	}
	chk(p.Pipe(
		context.Background(),
		apoco.FilterBad(c.Nocr+1), // at least n ocr + ground truth
		apoco.Normalize(),
		apoco.FilterShort(4),
		apoco.ConnectLanguageModel(m.Ngrams),
		apoco.ConnectUnigrams(),
		internal.ConnectProfile(c, "-profile.json.gz"),
		apoco.FilterLexiconEntries(),
		apoco.ConnectCandidates(),
		apoco.ConnectRankings(lr, fs, c.Nocr),
		dmEval(c, m),
	))
}

func dmEval(c *internal.Config, m apoco.Model) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		lr, fs, err := m.Get("dm", c.Nocr)
		if err != nil {
			return fmt.Errorf("evaldm: %v", err)
		}
		var xs, ys []float64
		var tokens []apoco.T
		err = apoco.EachToken(ctx, in, func(t apoco.T) error {
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
		return s.print(os.Stdout, "dm", c.Nocr)
	}
}

func dmGT(t apoco.T) float64 {
	candidate := t.Payload.([]apoco.Ranking)[0].Candidate
	gt := t.Tokens[len(t.Tokens)-1]
	// return ml.Bool(candidate.Suggestion == gt && t.Tokens[0] != gt)
	return ml.Bool(candidate.Suggestion == gt)
}
