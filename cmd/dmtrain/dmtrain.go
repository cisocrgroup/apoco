package dmtrain

import (
	"context"
	"fmt"
	"log"
	"strings"

	"example.com/apoco/pkg/apoco"
	"example.com/apoco/pkg/apoco/ml"
	"example.com/apoco/pkg/apoco/pagexml"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gonum.org/v1/gonum/mat"
)

func init() {
	CMD.Flags().StringVarP(&flags.mets, "mets", "m", "mets.xml", "set mets file")
	CMD.Flags().StringVarP(&flags.inputFileGrp, "input-file-grp", "I", "", "set input file group")
	CMD.Flags().StringVarP(&flags.parameters, "parameters", "P", "config.json", "set configuration file")
	CMD.Flags().IntVarP(&flags.nocr, "nocr", "n", 0, "set nocr (overwrites setting in the configuration file)")
	CMD.Flags().BoolVarP(&flags.nocache, "nocache", "c", false, "disable caching of profiles (overwrites setting in the configuration file)")
	CMD.Flags().StringVarP(&flags.model, "model", "M", "", "set model path (overwrites setting in the configuration file)")
}

var flags = struct {
	mets, inputFileGrp string
	parameters, model  string
	nocr               int
	nocache            bool
}{}

// CMD defines the apoco train command.
var CMD = &cobra.Command{
	Use:   "dmtrain",
	Short: "Train a decision maker model",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	c, err := apoco.ReadConfig(flags.parameters)
	noerr(err)
	c.Overwrite(flags.model, flags.nocr, flags.nocache)
	m, err := apoco.ReadModel(c.Model, c.Ngrams)
	noerr(err)
	lr, fs, err := m.Load("dm", c.Nocr)
	noerr(err)
	g, ctx := errgroup.WithContext(context.Background())
	_ = apoco.Pipe(ctx, g,
		pagexml.Tokenize(flags.mets, strings.Split(flags.inputFileGrp, ",")...),
		apoco.Normalize,
		apoco.FilterShort,
		apoco.ConnectLM(c, m.Ngrams),
		apoco.FilterLexiconEntries,
		apoco.ConnectCandidates,
		apoco.ConnectRankings(lr, fs, c.Nocr),
		traindm(c, m))
	noerr(g.Wait())
}

func traindm(c *apoco.Config, m apoco.Model) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
		g.Go(func() error {
			fs, err := apoco.NewFeatureSet(c.DMFeatures...)
			if err != nil {
				return fmt.Errorf("traindm: %v", err)
			}
			var xs, ys []float64
			err = apoco.EachToken(ctx, in, func(t apoco.Token) error {
				vals := fs.Calculate(t, c.Nocr)
				xs = append(xs, vals...)
				ys = append(ys, gt(t))
				return nil
			})
			if err != nil {
				return fmt.Errorf("traindm: %v", err)
			}
			lr := ml.LR{
				LearningRate: c.LearningRate,
				Ntrain:       c.Ntrain,
			}
			x := mat.NewDense(len(ys), len(xs)/len(ys), xs)
			y := mat.NewVecDense(len(ys), ys)
			if err := ml.Normalize(x); err != nil {
				return fmt.Errorf("traindm: %v", err)
			}
			log.Printf("fitting %d tokens, %d features, nocr=%d, lr=%f, ntrain=%d",
				len(ys), len(xs)/len(ys), c.Nocr, lr.LearningRate, lr.Ntrain)
			lr.Fit(x, y)
			m.Put("dm", c.Nocr, &lr, c.DMFeatures)
			if err := m.Write(c.Model); err != nil {
				return fmt.Errorf("traindm: %v", err)
			}
			return nil
		})
		return nil
	}
}

func gt(t apoco.Token) float64 {
	candidate := t.Payload.([]apoco.Ranking)[0].Candidate
	return ml.Bool(candidate.Suggestion == t.Tokens[len(t.Tokens)-1])
}

func noerr(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
