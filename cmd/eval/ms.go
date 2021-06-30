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

// msCMD defines the apoco train rr command.
var msCMD = &cobra.Command{
	Use:   "ms [DIRS...]",
	Short: "Evaluate an apoco merge split model",
	Run:   msRun,
}

var msFlags struct {
	threshold float64
	window    int
}

func init() {
	msCMD.Flags().Float64VarP(&msFlags.threshold, "threshold", "t", 0.5, "set the threshold for the merge confidence")
	msCMD.Flags().IntVarP(&msFlags.window, "window", "w", 2, "set the maximum window size")
}

func msRun(_ *cobra.Command, args []string) {
	c, err := internal.ReadConfig(flags.parameter)
	chk(err)

	internal.UpdateInConfig(&c.Model, flags.model)
	internal.UpdateInConfig(&c.Nocr, flags.nocr)
	internal.UpdateInConfig(&c.MS.Window, msFlags.window)
	internal.UpdateInConfig(&c.Cache, flags.cache)
	internal.UpdateInConfig(&c.AlignLev, flags.alev)

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
		apoco.ConnectLanguageModel(m.Ngrams),
		apoco.ConnectUnigrams(),
		apoco.ConnectMergesWithGT(c.MS.Window),
		internal.ConnectProfile(c, "-ms-profile.json.gz"),
		apoco.AddShortTokensToProfile(3),
		apoco.ConnectSplitCandidates(),
		// apoco.FilterLexiconEntries(),
		// apoco.ConnectCandidates(),
		msEval(c, m, msFlags.threshold, flags.update),
	))
}

func msEval(c *internal.Config, m apoco.Model, threshold float64, update bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		lr, fs, err := m.Get("ms", c.Nocr)
		if err != nil {
			return fmt.Errorf("eval ms: %v", err)
		}
		var xs []float64
		var x *mat.Dense
		var s stats
		err = apoco.EachToken(ctx, in, func(t apoco.T) error {
			slice := t.Payload.(apoco.Split).Tokens
			gt := msGT(slice)

			xs = fs.Calculate(xs, t, c.Nocr)
			if x == nil {
				x = mat.NewDense(1, len(xs), xs)
			}
			// pred := lr.Predict(x, threshold)
			probs := lr.PredictProb(x)
			switch s.add(gt, ml.Bool(probs.AtVec(0) > threshold)) { //} pred.AtVec(0)) {
			case tp:
				apoco.Log("true positive: %s (%g) %s", tstr(t), probs.AtVec(0), fs2str(xs))
			case fp:
				apoco.Log("false positive: %s (%g) %s", tstr(t), probs.AtVec(0), fs2str(xs))
			case fn:
				apoco.Log("false negative: %s (%g) %s", tstr(t), probs.AtVec(0), fs2str(xs))
			}
			xs = xs[0:0]
			return nil
		})
		if err != nil {
			return err
		}
		return s.print(os.Stdout, "ms", c.Nocr)
	}
}

func fs2str(xs []float64) string {
	var b strings.Builder
	pre := ""
	for _, x := range xs {
		b.WriteString(pre)
		pre = ","
		b.WriteString(fmt.Sprintf("%g", x))
	}
	return b.String()
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

func msGT(ts []apoco.T) float64 {
	for i := 1; i < len(ts); i++ {
		if ts[i-1].Tokens[len(ts[i-1].Tokens)-1] != ts[i].Tokens[len(ts[i].Tokens)-1] {
			return ml.False
		}
	}
	return ml.True
}
