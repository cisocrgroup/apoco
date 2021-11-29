package eval

import (
	"context"
	"fmt"
	"os"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/spf13/cobra"
)

// dmCMD defines the apoco train command.
var dmCMD = &cobra.Command{
	Use:   "dm  [DIRS...]",
	Short: "Evaluate a decision maker model",
	Run:   dmRun,
}

var dmFlags = struct {
	filter string
}{}

func init() {
	dmCMD.Flags().StringVarP(&dmFlags.filter, "filter", "f", "courageous",
		"use cautious training (overwrites the setting in the configuration file)")
}

func dmRun(_ *cobra.Command, args []string) {
	c, err := internal.ReadConfig(flags.parameter)
	chk(err)

	internal.UpdateInConfig(&c.Model, flags.model)
	internal.UpdateInConfig(&c.Nocr, flags.nocr)
	internal.UpdateInConfig(&c.Cache, flags.cache)
	internal.UpdateInConfig(&c.AlignLev, flags.alev)
	internal.UpdateInConfig(&c.DM.Filter, dmFlags.filter)

	m, err := internal.ReadModel(c.Model, c.LM, false)
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
		apoco.ConnectLanguageModel(m.LM),
		apoco.ConnectUnigrams(),
		internal.ConnectProfile(c, "-profile.json.gz"),
		apoco.FilterLexiconEntries(),
		apoco.ConnectCandidates(),
		apoco.ConnectRankings(lr, fs, c.Nocr),
		dmEval(c, m),
	))
}

func dmEval(c *internal.Config, m *internal.Model) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		fail := func(err error) error {
			return fmt.Errorf("eval dm/%d: %v", c.Nocr, err)
		}
		lr, fs, err := m.Get("dm", c.Nocr)
		if err != nil {
			return fail(err)
		}
		var xs, ys []float64
		err = apoco.EachToken(ctx, in, func(t apoco.T) error {
			xs = fs.Calculate(xs, t, c.Nocr)
			ys = append(ys, dmGT(t))
			return nil
		})
		if err != nil {
			return fail(err)
		}
		var s stats
		s.eval(lr, 0.5, xs, ys)
		return s.print(os.Stdout, "dm", c.Nocr)
	}
}

func dmGT(t apoco.T) float64 {
	candidate := t.Payload.([]apoco.Ranking)[0].Candidate
	gt := t.Tokens[len(t.Tokens)-1]
	// return ml.Bool(candidate.Suggestion == gt && t.Tokens[0] != gt)
	return ml.Bool(candidate.Suggestion == gt)
}
