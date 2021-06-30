package train

import (
	"context"
	"fmt"
	"io"
	"os"

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

var dmFlags = struct {
	instances string
	filter    string
}{}

func init() {
	dmCMD.Flags().StringVarP(&dmFlags.filter, "filter", "f", "courageous",
		"use cautious training (overwrites the setting in the configuration file)")
	dmCMD.Flags().StringVarP(&dmFlags.instances, "instances", "i", "",
		"set output path of training instances")
}

func dmRun(_ *cobra.Command, args []string) {
	c, err := internal.ReadConfig(flags.parameter)
	chk(err)

	internal.UpdateInConfig(&c.Model, flags.model)
	internal.UpdateInConfig(&c.Nocr, flags.nocr)
	internal.UpdateInConfig(&c.Cache, flags.cache)
	internal.UpdateInConfig(&c.AlignLev, flags.alev)
	internal.UpdateInConfig(&c.DM.Filter, dmFlags.filter)

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
		dmTrain(c, m, dmFlags.instances, flags.update),
	))
}

func dmTrain(c *internal.Config, m apoco.Model, instances string, update bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		lr, fs, err := loadDMModel(c, m, update)
		if err != nil {
			return fmt.Errorf("train dm: %v", err)
		}
		w, err := instanceWriter(instances)
		if err != nil {
			return fmt.Errorf("train dm: %v", err)
		}
		defer w.Close()
		var xs, ys []float64
		err = apoco.EachToken(ctx, in, func(t apoco.T) error {
			if !useTokenForDMTraining(t, c.DM.Filter) {
				return nil
			}
			_, err := fmt.Fprintf(w, "id=%s ocr=%s sug=%s gt=%s val=%g\n",
				t.ID, t.Tokens[0], t.Payload.([]apoco.Ranking)[0].Candidate.Suggestion,
				internal.E(t.Tokens[len(t.Tokens)-1]), dmGT(t))
			if err != nil {
				return err
			}
			xs = fs.Calculate(xs, t, c.Nocr)
			ys = append(ys, dmGT(t))
			return nil
		})
		if err != nil {
			return fmt.Errorf("train dm: %v", err)
		}
		if len(ys) == 0 {
			return fmt.Errorf("train dm: no input")
		}
		x := mat.NewDense(len(ys), len(xs)/len(ys), xs)
		y := mat.NewVecDense(len(ys), ys)
		chk(logCorrelationMat(c, fs, x, "dm"))
		if err := ml.Normalize(x); err != nil {
			return fmt.Errorf("train dm: %v", err)
		}
		apoco.Log("train dm: fitting %d toks, %d feats, nocr=%d, lr=%g, ntrain=%d, filter=%s",
			len(ys), len(xs)/len(ys), c.Nocr, lr.LearningRate, lr.Ntrain, c.DM.Filter)
		ferr := lr.Fit(x, y)
		apoco.Log("train dm: remaining error: %g", ferr)
		m.Put("dm", c.Nocr, lr, c.DM.Features)
		if err := m.Write(c.Model); err != nil {
			return fmt.Errorf("train dm: %v", err)
		}
		return nil
	}
}

func loadDMModel(c *internal.Config, m apoco.Model, update bool) (*ml.LR, apoco.FeatureSet, error) {
	if update {
		return m.Get("dm", c.Nocr)
	}
	fs, err := apoco.NewFeatureSet(c.DM.Features...)
	if err != nil {
		return nil, nil, err
	}
	lr := &ml.LR{
		LearningRate: c.DM.LearningRate,
		Ntrain:       c.DM.Ntrain,
	}
	return lr, fs, nil
}

type noopWriteCloser struct{}

func (noopWriteCloser) Close() error                { return nil }
func (noopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }

func instanceWriter(path string) (io.WriteCloser, error) {
	if path == "" {
		return noopWriteCloser{}, nil
	}
	return os.Create(path)
}

func useTokenForDMTraining(t apoco.T, filter string) bool {
	if filter == internal.Cautious {
		return true
	}
	ocr := t.Tokens[0]
	gt := t.Tokens[len(t.Tokens)-1]
	// If ocr != gt we use the token if the correction suggestion is correct.
	// We skip token with "don't care corrections" (incorrect correction
	// for an incorrect ocr token).
	if ocr != gt {
		return t.Payload.([]apoco.Ranking)[0].Candidate.Suggestion == gt
	}
	// We do not want to train with redundant corrections (ocr == gt && sugg == gt).
	// If ocr == gt and sugg == gt we skip the token for the training.
	// Note that at this point ocr == gt holds (see above).
	if filter == internal.Redundant {
		return t.Payload.([]apoco.Ranking)[0].Candidate.Suggestion != gt
	}
	return true
}

func dmGT(t apoco.T) float64 {
	candidate := t.Payload.([]apoco.Ranking)[0].Candidate
	gt := t.Tokens[len(t.Tokens)-1]
	return ml.Bool(candidate.Suggestion == gt)
}
