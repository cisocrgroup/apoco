package correct

import (
	"context"
	"fmt"
	"log"
	"unicode/utf8"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func init() {
	flags.Flags.Init(CMD)
	CMD.Flags().StringVarP(&flags.outputFileGrp, "output-file-grp", "O", "", "set input file group")
	CMD.Flags().IntVarP(&flags.nocr, "nocr", "n", 0, "set nocr (overwrites setting in the configuration file)")
	CMD.Flags().BoolVarP(&flags.cache, "cache", "c", false, "enable caching of profiles (overwrites setting in the configuration file)")
	CMD.Flags().BoolVarP(&flags.protocol, "protocol", "p", false, "add evaluation protocol")
	CMD.Flags().StringVarP(&flags.model, "model", "M", "", "set model path (overwrites setting in the configuration file)")
	CMD.Flags().BoolVarP(&flags.simple, "simple", "s", false, "do not correct only output")
}

var flags = struct {
	internal.Flags
	outputFileGrp           string
	model                   string
	nocr                    int
	cache, simple, protocol bool
}{}

// CMD runs the apoco correct command.
var CMD = &cobra.Command{
	Use:   "correct [INPUT...]",
	Short: "Automatically correct documents",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	c, err := apoco.ReadConfig(flags.Params)
	chk(err)
	c.Overwrite(flags.model, flags.nocr, false, flags.cache)
	m, err := apoco.ReadModel(c.Model, c.Ngrams)
	chk(err)
	rrlr, rrfs, err := m.Get("rr", c.Nocr)
	chk(err)
	dmlr, dmfs, err := m.Get("dm", c.Nocr)
	chk(err)
	infoMap := make(infoMap)
	g, ctx := errgroup.WithContext(context.Background())
	_ = apoco.Pipe(ctx, g,
		flags.Flags.Tokenize(args),
		apoco.FilterBad(c.Nocr+1), // at least n ocr + ground truth
		apoco.Normalize,
		register(infoMap),
		filterShort(infoMap),
		apoco.ConnectLM(c, m.Ngrams),
		filterLex(infoMap),
		apoco.ConnectCandidates,
		apoco.ConnectRankings(rrlr, rrfs, c.Nocr),
		analyzeRankings(infoMap),
		apoco.ConnectCorrections(dmlr, dmfs, c.Nocr),
		correct(infoMap),
	)
	chk(g.Wait())
	log.Printf("correction %d pages (%d tokens)", len(infoMap), infoMap.numberOfTokens())
	if flags.simple {
		for _, ids := range infoMap {
			for _, info := range ids {
				fmt.Printf("%s\n", info)
			}
		}
	} else {
		cor := corrector{
			info:     infoMap,
			mets:     flags.METS,
			ifgs:     append(args, flags.IFGs()...),
			ofg:      flags.outputFileGrp,
			protocol: flags.protocol,
		}
		chk(cor.correct())
	}
}

func correct(m infoMap) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
		g.Go(func() error {
			return apoco.EachToken(ctx, in, func(t apoco.Token) error {
				info := m.get(t)
				info.skipped = false
				info.cor = t.Payload.(apoco.Correction).Conf > 0.5
				info.conf = t.Payload.(apoco.Correction).Conf
				info.sug = t.Payload.(apoco.Correction).Candidate.Suggestion
				return nil
			})
		})
		return nil
	}
}

func register(m infoMap) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
		out := make(chan apoco.Token)
		g.Go(func() error {
			defer close(out)
			return apoco.EachToken(ctx, in, func(t apoco.Token) error {
				// Each token is skipped as default.
				// If a token is not skipped, skipped
				// must be explicitly set to false.
				m.get(t).skipped = true
				if err := apoco.SendTokens(ctx, out, t); err != nil {
					return fmt.Errorf("register: %v", err)
				}
				return nil
			})
		})
		return out
	}
}

func filterLex(m infoMap) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
		out := make(chan apoco.Token)
		g.Go(func() error {
			defer close(out)
			return apoco.EachToken(ctx, in, func(t apoco.Token) error {
				if t.IsLexiconEntry() {
					m.get(t).lex = true
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

func filterShort(m infoMap) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
		out := make(chan apoco.Token)
		g.Go(func() error {
			defer close(out)
			return apoco.EachToken(ctx, in, func(t apoco.Token) error {
				if utf8.RuneCountInString(t.Tokens[0]) <= 3 {
					m.get(t).short = true
					return nil
				}
				if err := apoco.SendTokens(ctx, out, t); err != nil {
					return fmt.Errorf("filterShort: %v", err)
				}
				return nil
			})
		})
		return out
	}
}

func analyzeRankings(m infoMap) apoco.StreamFunc {
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
				m.get(t).rank = rank
				if err := apoco.SendTokens(ctx, out, t); err != nil {
					return fmt.Errorf("analyzeRankings: %v", err)
				}
				return nil
			})
		})
		return out
	}
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
