package correct

import (
	"context"
	"fmt"
	"log"

	"example.com/apoco/pkg/apoco"
	"example.com/apoco/pkg/apoco/pagexml"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func init() {
	CMD.Flags().StringVarP(&flags.mets, "mets", "m", "mets.xml", "set mets file")
	CMD.Flags().StringVarP(&flags.inputFileGrp, "input-file-grp", "I", "", "set input file group")
	CMD.Flags().StringVarP(&flags.outputFileGrp, "output-file-grp", "O", "", "set input file group")
	CMD.Flags().StringVarP(&flags.parameters, "parameters", "P", "config.json", "set configuration file")
	CMD.Flags().IntVarP(&flags.nocr, "nocr", "n", 0, "set nocr (overwrites setting in the configuration file)")
	CMD.Flags().BoolVarP(&flags.cache, "cache", "c", false, "enable caching of profiles (overwrites setting in the configuration file)")
	CMD.Flags().StringVarP(&flags.model, "model", "M", "", "set model path (overwrites setting in the configuration file)")
}

var flags = struct {
	mets, inputFileGrp, outputFileGrp string
	parameters, model                 string
	nocr                              int
	cache                             bool
}{}

// CMD runs the apoco correct command.
var CMD = &cobra.Command{
	Use:   "correct",
	Short: "Automatically correct documents",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	c, err := apoco.ReadConfig(flags.parameters)
	noerr(err)
	c.Overwrite(flags.model, flags.nocr, !flags.cache)
	m, err := apoco.ReadModel(c.Model, c.Ngrams)
	noerr(err)
	rrlr, rrfs, err := m.Load("rr", c.Nocr)
	noerr(err)
	dmlr, dmfs, err := m.Load("dm", c.Nocr)
	noerr(err)
	cor := corrector{
		mets: flags.mets,
		ifg:  flags.inputFileGrp,
		ofg:  flags.outputFileGrp,
	}
	g, ctx := errgroup.WithContext(context.Background())
	_ = apoco.Pipe(ctx, g,
		pagexml.Tokenize(flags.mets, flags.inputFileGrp),
		apoco.Normalize,
		apoco.FilterShort,
		apoco.ConnectLM(c, m.Ngrams),
		filterLex(&cor),
		apoco.ConnectCandidates,
		apoco.ConnectRankings(rrlr, rrfs, c.Nocr),
		analyzeRankings(&cor),
		apoco.ConnectCorrections(dmlr, dmfs, c.Nocr),
		correct(&cor),
	)
	noerr(g.Wait())
	noerr(cor.correct())
}

func correct(cor *corrector) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
		g.Go(func() error {
			return apoco.EachToken(ctx, in, func(t apoco.Token) error {
				cor.addCorrected(t)
				return nil
			})
		})
		return nil
	}
}

func filterLex(cor *corrector) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
		out := make(chan apoco.Token)
		g.Go(func() error {
			defer close(out)
			return apoco.EachToken(ctx, in, func(t apoco.Token) error {
				if t.IsLexiconEntry() {
					cor.addLex(t)
					return nil
				}
				if err := apoco.SendTokens(ctx, out, t); err != nil {
					return fmt.Errorf("filterLex: %v", err)
				}
				return nil
			})
		})
		return out
	}
}

func analyzeRankings(cor *corrector) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
		out := make(chan apoco.Token)
		g.Go(func() error {
			defer close(out)
			return apoco.EachToken(ctx, in, func(t apoco.Token) error {
				var rank int
				for i, r := range t.Payload.([]apoco.Ranking) {
					if r.Candidate.Suggestion == t.Tokens[len(t.Tokens)-1] {
						rank = i + 1
						break
					}
				}
				cor.addRank(t, rank)
				if err := apoco.SendTokens(ctx, out, t); err != nil {
					return fmt.Errorf("analyzeRankings: %v", err)
				}
				return nil
			})
		})
		return out
	}
}

func noerr(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
