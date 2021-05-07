package train

import (
	"context"
	"fmt"
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
		countMerges(),
		apoco.ConnectDocument(m.Ngrams),
		apoco.ConnectUnigrams(),
		apoco.ConnectMergesWithGT(mrgFlags.max),
		apoco.ConnectProfile(c.ProfilerBin, c.ProfilerConfig, false),
		apoco.AddShortTokensToProfile(3),
		apoco.ConnectSplitCandidates(),
		// apoco.FilterLexiconEntries(),
		// apoco.ConnectCandidates(),
		mrgTrain(c, m, flags.update),
	))
}

var total, splits, cleanSplits, merges, cleanMerges int

func countMerges() apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			total++
			if strings.Contains(t.Tokens[len(t.Tokens)-1], "_") {
				merges++
				if strings.Contains(t.Tokens[len(t.Tokens)-1], t.Tokens[0]) {
					cleanMerges++
				}
			}
			return apoco.SendTokens(ctx, out, t)
		})
	}
}

func mrgTrain(c *internal.Config, m apoco.Model, update bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		lr, fs, err := loadMRGModel(c, m, flags.update)
		if err != nil {
			return fmt.Errorf("train mrg: %v", err)
		}
		var xs, ys []float64
		err = apoco.EachToken(ctx, in, func(t apoco.T) error {
			gt := mrgGT(t)
			apoco.Log("%s has %d candidates", t, len(t.Payload.(apoco.Split).Candidates))
			if len(t.Payload.(apoco.Split).Candidates) > 0 {
				apoco.Log("first candidate: %s", t.Payload.(apoco.Split).Candidates[0])
			} else {
				return fmt.Errorf("token with no candidates")
			}
			if gt == ml.True {
				splits++
				if t.Tokens[0] == t.Tokens[len(t.Tokens)-1] {
					cleanSplits++
				}
				apoco.Log("merge: %s", t)
				for _, xt := range t.Payload.(apoco.Split).Tokens {
					apoco.Log(" - %s", xt)
				}
			}
			xs = fs.Calculate(xs, t, c.Nocr)
			ys = append(ys, gt)
			return nil
		})
		if err != nil {
			return err
		}
		n := len(ys) // number or training tokens
		if n == 0 {
			return fmt.Errorf("train mrg: no input")
		}
		x := mat.NewDense(n, len(xs)/n, xs)
		y := mat.NewVecDense(n, ys)
		if err := ml.Normalize(x); err != nil {
			return fmt.Errorf("train mrg: %v", err)
		}
		chk(logCorrelationMat(c, fs, x, "mrg"))
		apoco.Log("train mrg: fitting %d toks, %d feats, nocr=%d, lr=%g, ntrain=%d",
			n, len(xs)/n, c.Nocr, lr.LearningRate, lr.Ntrain)
		ferr := lr.Fit(x, y)
		apoco.Log("train mrg: remaining error: %g", ferr)
		m.Put("mrg", c.Nocr, lr, c.MRGFeatures)
		if err := m.Write(c.Model); err != nil {
			return fmt.Errorf("train mrg: %v", err)
		}
		apoco.Log("total: %d", total)
		apoco.Log("splits: %d/%d/%d", splits, cleanSplits, splits-cleanSplits)
		apoco.Log("merges: %d/%d/%d", merges, cleanMerges, merges-cleanMerges)
		return nil
	}
}

func loadMRGModel(c *internal.Config, m apoco.Model, update bool) (*ml.LR, apoco.FeatureSet, error) {
	if update {
		return m.Get("mrg", c.Nocr)
	}
	fs, err := apoco.NewFeatureSet(c.MRGFeatures...)
	if err != nil {
		return nil, nil, err
	}
	lr := &ml.LR{
		LearningRate: c.LearningRate,
		Ntrain:       c.Ntrain,
	}
	return lr, fs, nil
}

func mrgGT(t apoco.T) float64 {
	ts := t.Payload.(apoco.Split).Tokens
	if ts[0].Tokens[len(ts[0].Tokens)-1] == t.Tokens[len(t.Tokens)-1] {
		return ml.True
	}
	return ml.False
}
