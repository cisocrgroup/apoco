package eval

import (
	"context"
	"fmt"
	"os"
	"strings"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/spf13/cobra"
	"gonum.org/v1/gonum/mat"
)

// mrgCMD defines the apoco train rr command.
var mrgCMD = &cobra.Command{
	Use:   "mrg [DIRS...]",
	Short: "Train an apoco merge model",
	Run:   mrgRun,
}

var mrgFlags struct {
	max int
}

func init() {
	mrgCMD.Flags().IntVarP(&mrgFlags.max, "max", "m", 2, "set the maximum tokens for merges")
}

func mrgRun(_ *cobra.Command, args []string) {
	c, err := internal.ReadConfig(flags.parameter)
	chk(err)
	c.Overwrite(flags.model, flags.nocr, flags.cautious, flags.cache, false)
	m, err := apoco.ReadModel(c.Model, c.Ngrams)
	chk(err)
	p := internal.Piper{
		Exts: flags.extensions,
		Dirs: args,
	}
	chk(p.Pipe(
		context.Background(),
		apoco.FilterBad(c.Nocr+1), // at least n ocr + ground truth
		apoco.Normalize(),
		apoco.FilterShort(1), // skip empty token
		apoco.ConnectDocument(m.Ngrams),
		apoco.ConnectUnigrams(),
		apoco.ConnectMergesWithGT(mrgFlags.max),
		apoco.ConnectProfile(c.ProfilerBin, c.ProfilerConfig, false),
		apoco.AddShortTokensToProfile(3),
		apoco.ConnectSplitCandidates(),
		// apoco.FilterLexiconEntries(),
		// apoco.ConnectCandidates(),
		mrgEval(c, m, flags.update),
	))
}

func mrgEval(c *internal.Config, m apoco.Model, update bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		lr, fs, err := m.Get("mrg", c.Nocr)
		if err != nil {
			return fmt.Errorf("eval mrg: %v", err)
		}
		var xs []float64
		var x *mat.Dense
		var s stats
		err = apoco.EachToken(ctx, in, func(t apoco.T) error {
			slice := t.Payload.(apoco.Split).Tokens
			gt := mrgGT(slice)

			xs = fs.Calculate(xs, t, c.Nocr)
			if x == nil {
				x = mat.NewDense(1, len(xs), xs)
			}
			pred := lr.Predict(x, 0.5)
			xs = xs[0:0]
			switch s.add(gt, pred.AtVec(0)) {
			case tp:
				apoco.Log("true positive: %s", tstr(t))
			case fp:
				apoco.Log("false positive: %s", tstr(t))
			case fn:
				apoco.Log("false negative: %s", tstr(t))
			}
			return nil
		})
		if err != nil {
			return err
		}
		return s.print(os.Stdout, "mrg", c.Nocr)
	}
}

func tstr(t apoco.T) string {
	var b strings.Builder
	b.WriteString(t.String())
	pre := " ("
	for _, tx := range t.Payload.(apoco.Split).Tokens {
		b.WriteString(pre)
		b.WriteString(tx.String())
		pre = " "
	}
	b.WriteString(")")
	return b.String()
}

func mrgGT(ts []apoco.T) float64 {
	for i := 1; i < len(ts); i++ {
		if ts[i-1].Tokens[len(ts[i-1].Tokens)-1] != ts[i].Tokens[len(ts[i].Tokens)-1] {
			return ml.False
		}
	}
	return ml.True
}
