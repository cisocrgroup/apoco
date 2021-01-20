package train

import (
	"context"
	"fmt"
	"log"

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
	c, err := apoco.ReadConfig(flags.parameters)
	chk(err)
	c.Overwrite(flags.model, flags.nocr, flags.cautious, flags.cache)
	m, err := apoco.ReadModel(c.Model, c.Ngrams)
	chk(err)
	chk(pipe(context.Background(), flags.extensions, args,
		apoco.FilterBad(c.Nocr+1), // at least n ocr + ground truth
		apoco.Normalize(),
		apoco.FilterShort(4),
		apoco.ConnectLM(c, m.Ngrams),
		apoco.FilterLexiconEntries(),
		apoco.ConnectCandidates(),
		rrTrain(c, m, flags.update)))
}

func rrTrain(c *apoco.Config, m apoco.Model, update bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		lr, fs, err := loadRRModel(c, m, update)
		if err != nil {
			return fmt.Errorf("rrtrain: %v", err)
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
			return fmt.Errorf("rrtrain: %v", err)
		}
		n := len(ys) // number or training tokens
		x := mat.NewDense(n, len(xs)/n, xs)
		y := mat.NewVecDense(n, ys)
		if err := ml.Normalize(x); err != nil {
			return fmt.Errorf("rrtrain: %v", err)
		}
		log.Printf("rrtrain: fitting %d toks, %d feats, nocr=%d, lr=%f, ntrain=%d",
			n, len(xs)/n, c.Nocr, lr.LearningRate, lr.Ntrain)
		lr.Fit(x, y)
		log.Printf("rrtrain: fitted %d toks, %d feats, nocr=%d, lr=%f, ntrain=%d",
			len(ys), len(xs)/len(ys), c.Nocr, lr.LearningRate, lr.Ntrain)
		m.Put("rr", c.Nocr, lr, c.RRFeatures)
		m.GlobalHistPatterns = lms.globalHistPatternMeans()
		m.GlobalOCRPatterns = lms.globalOCRPatternMeans()
		if err := m.Write(c.Model); err != nil {
			return fmt.Errorf("rrtrain: %v", err)
		}
		return nil
	}
}

func loadRRModel(c *apoco.Config, m apoco.Model, update bool) (*ml.LR, apoco.FeatureSet, error) {
	if update {
		return m.Get("rr", c.Nocr)
	}
	fs, err := apoco.NewFeatureSet(c.RRFeatures...)
	if err != nil {
		return nil, nil, err
	}
	lr := &ml.LR{
		LearningRate: c.LearningRate,
		Ntrain:       c.Ntrain,
	}
	return lr, fs, nil
}

func rrGT(t apoco.T) float64 {
	candidate := t.Payload.(*gofiler.Candidate)
	return ml.Bool(candidate.Suggestion == t.Tokens[len(t.Tokens)-1])
}

type lms map[*apoco.LanguageModel]struct{}

func (lms lms) add(t apoco.T) {
	lms[t.LM] = struct{}{}
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
	n := len(xs)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return xs[0]
	}
	keys := make(map[string]struct{})
	for _, x := range xs {
		for key := range x {
			keys[key] = struct{}{}
		}
	}
	means := make(map[string]float64)
	for key := range keys {
		sum := 0.0
		for _, x := range xs {
			val, _ := x[key]
			sum += val
		}
		means[key] = sum / float64(n)
	}
	return means
}
