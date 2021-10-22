package eval

import (
	"context"
	"fmt"
	"os"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/finkf/gofiler"
	"github.com/spf13/cobra"
)

// rrCMD defines the apoco eval rr command.
var rrCMD = &cobra.Command{
	Use:   "rr [DIR...]",
	Short: "Evaluate an apoco re-ranking model",
	Run:   rrRun,
}

func rrRun(_ *cobra.Command, args []string) {
	c, err := internal.ReadConfig(flags.parameter)
	chk(err)

	internal.UpdateInConfig(&c.Model, flags.model)
	internal.UpdateInConfig(&c.Nocr, flags.nocr)
	internal.UpdateInConfig(&c.Cache, flags.cache)
	internal.UpdateInConfig(&c.AlignLev, flags.alev)

	m, err := internal.ReadModel(c.Model, c.LM, false)
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
		apoco.ConnectLanguageModel(m.LM),
		apoco.ConnectUnigrams(),
		internal.ConnectProfile(c, "-profile.json.gz"),
		apoco.FilterLexiconEntries(),
		apoco.ConnectCandidates(),
		rrEval(c, m),
	))
}

func rrEval(c *internal.Config, m *internal.Model) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		fail := func(err error) error {
			return fmt.Errorf("eval rr/%d: %v", c.Nocr, err)
		}
		lr, fs, err := m.Get("rr", c.Nocr)
		if err != nil {
			return fail(err)
		}
		var xs, ys []float64
		err = apoco.EachToken(ctx, in, func(t apoco.T) error {
			xs = fs.Calculate(xs, t, c.Nocr)
			ys = append(ys, rrGT(t))
			return nil
		})
		if err != nil {
			return fail(err)
		}
		var s stats
		s.eval(lr, 0.5, xs, ys)
		return s.print(os.Stdout, "rr", c.Nocr)
	}
}

func rrGT(t apoco.T) float64 {
	candidate := t.Payload.(*gofiler.Candidate)
	return ml.Bool(candidate.Suggestion == t.Tokens[len(t.Tokens)-1])
}
