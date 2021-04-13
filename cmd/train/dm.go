package train

import (
	"context"
	"fmt"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/spf13/cobra"
	"gonum.org/v1/gonum/mat"
)

// dmCMD defines the apoco train command.
var dmCMD = &cobra.Command{
	Use:   "dm [DIRS...]",
	Short: "Train a decision maker model",
	Run:   dmRun,
}

func dmRun(_ *cobra.Command, args []string) {
	c, err := apoco.ReadConfig(flags.parameter)
	chk(err)
	c.Overwrite(flags.model, flags.nocr, flags.cautious, flags.cache, false)
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
		apoco.ConnectLM(m.Ngrams),
		apoco.ConnectUnigrams(),
		apoco.ConnectProfile(c.ProfilerBin, c.ProfilerConfig, c.Cache),
		apoco.FilterLexiconEntries(),
		apoco.ConnectCandidates(),
		apoco.ConnectRankings(lr, fs, c.Nocr),
		dmTrain(c, m, flags.update),
	))
}

func dmTrain(c *apoco.Config, m apoco.Model, update bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		lr, fs, err := loadDMModel(c, m, update)
		if err != nil {
			return fmt.Errorf("traindm: %v", err)
		}
		var xs, ys []float64
		err = apoco.EachToken(ctx, in, func(t apoco.T) error {
			if !useTokenForDMTraining(t, c.Cautious) {
				return nil
			}
			xs = fs.Calculate(xs, t, c.Nocr)
			ys = append(ys, dmGT(t))
			return nil
		})
		if err != nil {
			return fmt.Errorf("traindm: %v", err)
		}
		x := mat.NewDense(len(ys), len(xs)/len(ys), xs)
		y := mat.NewVecDense(len(ys), ys)
		if flags.correlation {
			chk(printCorrelationMat(c, fs, x, true))
		}
		if err := ml.Normalize(x); err != nil {
			return fmt.Errorf("traindm: %v", err)
		}
		apoco.Log("dmtrain: fitting %d toks, %d feats, nocr=%d, lr=%f, ntrain=%d, cautious=%t",
			len(ys), len(xs)/len(ys), c.Nocr, lr.LearningRate, lr.Ntrain, flags.cautious)
		ferr := lr.Fit(x, y)
		apoco.Log("dmtrain: remaining error %f", ferr)
		m.Put("dm", c.Nocr, lr, c.DMFeatures)
		if err := m.Write(c.Model); err != nil {
			return fmt.Errorf("traindm: %v", err)
		}
		return nil
	}
}

func loadDMModel(c *apoco.Config, m apoco.Model, update bool) (*ml.LR, apoco.FeatureSet, error) {
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

func useTokenForDMTraining(t apoco.T, cautious bool) bool {
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

func dmGT(t apoco.T) float64 {
	candidate := t.Payload.([]apoco.Ranking)[0].Candidate
	gt := t.Tokens[len(t.Tokens)-1]
	return ml.Bool(candidate.Suggestion == gt)
}
