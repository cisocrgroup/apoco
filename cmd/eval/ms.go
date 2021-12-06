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

// msCmd defines the apoco train rr command.
var msCmd = &cobra.Command{
	Use:   "ms [DIRS...]",
	Short: "Evaluate an apoco merge split model",
	Run:   msRun,
}

var msFlags struct {
	threshold float64
}

func init() {
	msCmd.Flags().Float64VarP(&msFlags.threshold, "threshold", "t", 0.5, "set the threshold for the merge confidence")
}

func msRun(_ *cobra.Command, args []string) {
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
		apoco.FilterShort(1), // skip empty token
		apoco.ConnectLanguageModel(m.LM),
		apoco.ConnectUnigrams(),
		apoco.ConnectMergesWithGT(),
		internal.ConnectProfile(c, "-ms-profile.json.gz"),
		apoco.AddShortTokensToProfile(3),
		apoco.ConnectSplitCandidates(),
		// apoco.FilterLexiconEntries(),
		// apoco.ConnectCandidates(),
		msEval(c, m, msFlags.threshold, flags.update),
	))
}

func msEval(c *internal.Config, m *internal.Model, threshold float64, update bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		lr, fs, err := m.Get("ms", c.Nocr)
		if err != nil {
			return fmt.Errorf("eval ms: %v", err)
		}
		var xs []float64
		var x *mat.Dense
		var s stats
		names := fs.Names(c.MS.Features, "ms", c.Nocr)
		err = apoco.EachToken(ctx, in, func(t apoco.T) error {
			gt := msGT(t)

			xs = fs.Calculate(xs, t, c.Nocr)
			if x == nil {
				x = mat.NewDense(1, len(xs), xs)
			}
			// pred := lr.Predict(x, threshold)
			probs := lr.Predict(x)
			switch s.add(gt, ml.Bool(probs.AtVec(0) >= threshold)) { //} pred.AtVec(0)) {
			case tp:
				apoco.Log("true positive: %s (%g) %s", tstr(t), probs.AtVec(0), fs2str(xs, names))
				/*case fp:
					apoco.Log("false positive: %s (%g) %s", tstr(t), probs.AtVec(0), fs2str(xs, names))
				case fn:
					apoco.Log("false negative: %s (%g) %s", tstr(t), probs.AtVec(0), fs2str(xs, names))*/
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

func fs2str(xs []float64, names []string) string {
	var b strings.Builder
	pre := "\n- "
	for i, x := range xs {
		b.WriteString(pre)
		pre = "- "
		b.WriteString(fmt.Sprintf("%s: %g\n", names[i], x))
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

func msGT(t apoco.T) float64 {
	return ml.Bool(t.Payload.(apoco.Split).Valid)
}
