package train

import (
	"context"
	"fmt"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/finkf/gofiler"
	"github.com/spf13/cobra"
	"gonum.org/v1/gonum/mat"
)

// rrCMD defines the apoco train rr command.
var rrCMD = &cobra.Command{
	Use:   "rr [DIRS...]",
	Short: "Train an apoco re-ranking model",
	Run:   rrRun,
}

func rrRun(_ *cobra.Command, args []string) {
	c, err := internal.ReadConfig(flags.parameter)
	chk(err)

	internal.UpdateInConfig(&c.Model, flags.model)
	internal.UpdateInConfig(&c.Nocr, flags.nocr)
	internal.UpdateInConfig(&c.Cache, flags.cache)
	internal.UpdateInConfig(&c.AlignLev, flags.alev)

	m, err := apoco.ReadModel(c.Model, c.Ngrams)
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
		apoco.FilterShort(4),
		apoco.ConnectLanguageModel(m.Ngrams),
		apoco.ConnectUnigrams(),
		internal.ConnectProfile(c, "-profile.json.gz"),
		apoco.FilterLexiconEntries(),
		apoco.ConnectCandidates(),
		rrTrain(c, m, flags.update),
	))
}

func rrTrain(c *internal.Config, m apoco.Model, update bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		lr, fs, err := loadRRModel(c, m, update)
		if err != nil {
			return fmt.Errorf("rr train: %v", err)
		}
		var xs, ys []float64
		lms := make(lms)
		err = apoco.EachToken(ctx, in, func(t apoco.T) error {
			lms.add(t)
			xs = fs.Calculate(xs, t, c.Nocr)
			ys = append(ys, rrGT(t))
			return nil
		})
		if err != nil {
			return fmt.Errorf("train rr: %v", err)
		}
		n := len(ys) // number or training tokens
		if n == 0 {
			return fmt.Errorf("train rr: no input")
		}
		x := mat.NewDense(n, len(xs)/n, xs)
		y := mat.NewVecDense(n, ys)
		chk(logCorrelationMat(c, fs, x, "rr"))
		if err := ml.Normalize(x); err != nil {
			return fmt.Errorf("train rr: %v", err)
		}
		apoco.Log("train rr: fitting %d toks, %d feats, nocr=%d, lr=%g, ntrain=%d",
			n, len(xs)/n, c.Nocr, lr.LearningRate, lr.Ntrain)
		ferr := lr.Fit(x, y)
		apoco.Log("train rr: remaining error: %g", ferr)
		m.Put("rr", c.Nocr, lr, c.RR.Features)
		m.GlobalHistPatterns = lms.globalHistPatternMeans()
		m.GlobalOCRPatterns = lms.globalOCRPatternMeans()
		if err := m.Write(c.Model); err != nil {
			return fmt.Errorf("train rr: %v", err)
		}
		return nil
	}
}

func loadRRModel(c *internal.Config, m apoco.Model, update bool) (*ml.LR, apoco.FeatureSet, error) {
	if update {
		return m.Get("rr", c.Nocr)
	}
	fs, err := apoco.NewFeatureSet(c.RR.Features...)
	if err != nil {
		return nil, nil, err
	}
	lr := &ml.LR{
		LearningRate: c.RR.LearningRate,
		Ntrain:       c.RR.Ntrain,
	}
	return lr, fs, nil
}

func rrGT(t apoco.T) float64 {
	candidate := t.Payload.(*gofiler.Candidate)
	return ml.Bool(candidate.Suggestion == t.Tokens[len(t.Tokens)-1])
}

type lms map[*apoco.Document]struct{}

func (lms lms) add(t apoco.T) {
	lms[t.Document] = struct{}{}
}

func (lms lms) globalHistPatternMeans() map[string]float64 {
	xs := make([]map[string]float64, 0, len(lms))
	for lm := range lms {
		xs = append(xs, lm.Profile.GlobalHistPatterns())
	}
	return means(xs)
}

func (lms lms) globalOCRPatternMeans() map[string]float64 {
	xs := make([]map[string]float64, 0, len(lms))
	for lm := range lms {
		xs = append(xs, lm.Profile.GlobalOCRPatterns())
	}
	return means(xs)
}

func means(xs []map[string]float64) map[string]float64 {
	if len(xs) == 0 {
		return nil
	}
	if len(xs) == 1 {
		return xs[0]
	}
	means := make(map[string]float64)
	for _, x := range xs {
		for key := range x {
			means[key] = 0.0
		}
	}
	for key := range means {
		sum := 0.0
		for _, x := range xs {
			val := x[key]
			sum += val
		}
		means[key] = sum / float64(len(xs))
	}
	return means
}
