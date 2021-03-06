//go:build ignore

package csv

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

// msCmd defines the apoco train ms command.
var msCmd = &cobra.Command{
	Use:   "ms [[DIR...] | [FILE...]]",
	Short: "Extract merge-split features to csv",
	Run:   msRun,
}

func msRun(_ *cobra.Command, args []string) {
	// Handle configuration file.
	c, err := internal.ReadConfig(flags.parameter)
	chk(err)

	internal.UpdateInConfig(&c.Model, flags.model)
	internal.UpdateInConfig(&c.Nocr, flags.nocr)
	internal.UpdateInConfig(&c.Cache, flags.cache)
	internal.UpdateInConfig(&c.AlignLev, flags.alev)
	m, err := internal.ReadModel(c.Model, c.LM, true)
	chk(err)
	p := internal.Piper{
		Exts:     flags.extensions,
		Dirs:     args,
		AlignLev: c.AlignLev,
	}
	chk(p.Pipe(
		context.Background(),
		apoco.FilterBad(c.Nocr+1), // at least n ocr + ground truth
		apoco.Normalize(),
		apoco.FilterShort(1), // skip empty token
		countMerges(),
		apoco.ConnectLanguageModel(m.LM),
		apoco.ConnectUnigrams(),
		apoco.ConnectMergesWithGT(),
		internal.ConnectProfile(c, "-ms-profile.json.gz"),
		apoco.AddShortTokensToProfile(3),
		apoco.ConnectSplitCandidates(),
		// apoco.FilterLexiconEntries(),
		// apoco.ConnectCandidates(),
		msTrain(c, m, flags.update),
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

func msTrain(c *internal.Config, m *internal.Model, update bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		lr, fs, err := loadMSModel(c, m, flags.update)
		if err != nil {
			return fmt.Errorf("train ms: %v", err)
		}
		var xs, ys []float64
		err = apoco.EachToken(ctx, in, func(t apoco.T) error {
			gt := msGT(t)
			if len(t.Payload.(apoco.Split).Candidates) == 0 {
				return fmt.Errorf("token with no candidates")
			}
			if gt == ml.True {
				splits++
				typ := "unclean"
				if t.Tokens[0] == t.Tokens[len(t.Tokens)-1] {
					cleanSplits++
					typ = "clean"
				}
				apoco.Log("merge: %s (%s)", t, typ)
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
			return fmt.Errorf("train ms: no input")
		}
		x := mat.NewDense(n, len(xs)/n, xs)
		y := mat.NewVecDense(n, ys)
		if err := ml.Normalize(x); err != nil {
			return fmt.Errorf("train ms: %v", err)
		}
		chk(logCorrelationMat(c, fs, x, "ms"))
		apoco.Log("train ms: fitting %d toks, %d feats, nocr=%d, lr=%g, ntrain=%d",
			n, len(xs)/n, c.Nocr, lr.LearningRate, lr.Ntrain)
		ferr := lr.Fit(x, y)
		apoco.Log("train ms: remaining error: %g", ferr)
		m.Put("ms", c.Nocr, lr, c.MS.Features)
		if err := m.Write(c.Model); err != nil {
			return fmt.Errorf("train ms: %v", err)
		}
		apoco.Log("total: %d", total)
		apoco.Log("splits: %d/%d/%d", splits, cleanSplits, splits-cleanSplits)
		apoco.Log("merges: %d/%d/%d", merges, cleanMerges, merges-cleanMerges)
		fmt.Printf("splits: %d %d\n", total, splits)
		return nil
	}
}

func loadMSModel(c *internal.Config, m *internal.Model, update bool) (*ml.LR, apoco.FeatureSet, error) {
	if update {
		return m.Get("ms", c.Nocr)
	}
	fs, err := apoco.NewFeatureSet(c.MS.Features...)
	if err != nil {
		return nil, nil, err
	}
	lr := &ml.LR{
		LearningRate: c.MS.LearningRate,
		Ntrain:       c.MS.Ntrain,
	}
	return lr, fs, nil
}

func msGT(t apoco.T) float64 {
	return ml.Bool(t.Payload.(apoco.Split).Valid)
	// ts := t.Payload.(apoco.Split).Tokens
	// if ts[0].Tokens[len(ts[0].Tokens)-1] == t.Tokens[len(t.Tokens)-1] {
	// 	log.Printf("%q == %q", ts[0].Tokens[len(ts[0].Tokens)-1], t.Tokens[len(t.Tokens)-1])
	// 	log.Printf("TRUE")
	// 	return ml.True
	// }
	// return ml.False
}
