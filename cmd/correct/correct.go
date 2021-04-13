package correct

import (
	"context"
	"fmt"
	"log"
	"unicode/utf8"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/spf13/cobra"
)

var flags = struct {
	ifgs, extensions                     []string
	ofg, mets, model, parameter, profile string
	nocr                                 int
	cache, nogt                          bool
}{}

// CMD runs the apoco correct command.
var CMD = &cobra.Command{
	Use:   "correct [DIRS...]",
	Short: "Automatically post-correct documents",
	Run:   run,
}

func init() {
	CMD.Flags().StringSliceVarP(&flags.ifgs, "input-file-grp", "I",
		nil, "set input file groups")
	CMD.Flags().StringSliceVarP(&flags.extensions, "extensions", "e",
		[]string{".xml"}, "set input file extensions")
	CMD.Flags().StringVarP(&flags.ofg, "output-file-grp", "O",
		"", "set output file group")
	CMD.Flags().StringVarP(&flags.mets, "mets", "m",
		"mets.xml", "set path to the mets file")
	CMD.Flags().StringVarP(&flags.parameter, "parameter", "p",
		"config.toml", "set path to the configuration file")
	CMD.Flags().StringVarP(&flags.profile, "profile", "f",
		"", "set external profile file")
	CMD.Flags().IntVarP(&flags.nocr, "nocr", "n",
		0, "set nocr (overwrites setting in the configuration file)")
	CMD.Flags().StringVarP(&flags.model, "model", "M", "",
		"set model path (overwrites setting in the configuration file)")
	CMD.Flags().BoolVarP(&flags.cache, "cache", "c",
		false, "enable caching of profile")
	CMD.Flags().BoolVarP(&flags.nogt, "nogt", "g", false, "no ground-truth data")
}

func run(_ *cobra.Command, args []string) {
	c, err := apoco.ReadConfig(flags.parameter)
	chk(err)
	c.Overwrite(flags.model, flags.nocr, false, flags.cache)
	m, err := apoco.ReadModel(c.Model, c.Ngrams)
	chk(err)
	rrlr, rrfs, err := m.Get("rr", c.Nocr)
	chk(err)
	dmlr, dmfs, err := m.Get("dm", c.Nocr)
	chk(err)
	stoks := make(stokMap)
	p := internal.Piper{
		IFGS: flags.ifgs,
		METS: flags.mets,
		Exts: flags.extensions,
		Dirs: args,
	}
	chk(p.Pipe(
		context.Background(),
		apoco.FilterBad(c.Nocr),
		apoco.Normalize(),
		register(stoks, !flags.nogt),
		filterShort(stoks, !flags.nogt),
		connectlm(c, m.Ngrams, flags.profile),
		filterLex(stoks, !flags.nogt),
		apoco.ConnectCandidates(),
		apoco.ConnectRankings(rrlr, rrfs, c.Nocr),
		analyzeRankings(stoks, !flags.nogt),
		apoco.ConnectCorrections(dmlr, dmfs, c.Nocr),
		correct(stoks, !flags.nogt),
	))
	apoco.Log("correcting %d pages (%d tokens)", len(stoks), stoks.numberOfTokens())
	// If no output file group is given, we do not need to correct
	// the according page XML files.  We just output the stoks.  So
	// if input file groups are given we output the stoks.  Only if
	// an output file group is given, we do correct the according page
	// XML files within the output file group.
	if flags.ofg == "" {
		for _, ids := range stoks {
			for _, info := range ids {
				fmt.Printf("%s\n", info)
			}
		}
		return
	}
	// We need to correct the according page XML files.
	cor := corrector{
		stoks: stoks,
		ifgs:  append(args, flags.ifgs...),
		ofg:   flags.ofg,
	}
	chk(cor.correct(flags.mets))
}

func correct(m stokMap, withGT bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			stok := m.get(t, withGT)
			stok.Skipped = false
			stok.Cor = t.Payload.(apoco.Correction).Conf > 0.5
			stok.Conf = t.Payload.(apoco.Correction).Conf
			stok.Sug = t.Payload.(apoco.Correction).Candidate.Suggestion
			return nil
		})
	}
}

func register(m stokMap, withGT bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			// Each token gets its ID and is skipped as default.
			// If a token is not skipped, skipped
			// must be explicitly set to false.
			stok := m.get(t, withGT)
			stok.ID = t.ID
			stok.Skipped = true
			if err := apoco.SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("register: %v", err)
			}
			return nil
		})
	}
}

func filterLex(m stokMap, withGT bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			if t.IsLexiconEntry() {
				m.get(t, withGT).Lex = true
				return nil
			}
			if err := apoco.SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("filterLex: %v", err)
			}
			return nil
		})
	}
}

func filterShort(m stokMap, withGT bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			if utf8.RuneCountInString(t.Tokens[0]) <= 3 {
				m.get(t, withGT).Short = true
				return nil
			}
			if err := apoco.SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("filterShort: %v", err)
			}
			return nil
		})
	}
}

func analyzeRankings(m stokMap, withGT bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			if withGT {
				var rank int
				for i, r := range t.Payload.([]apoco.Ranking) {
					if r.Candidate.Suggestion == t.Tokens[len(t.Tokens)-1] {
						rank = i + 1
						break
					}
				}
				m.get(t, withGT).Rank = rank
			}
			if err := apoco.SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("analyzeRankings: %v", err)
			}
			return nil
		})
	}
}

func connectlm(c *apoco.Config, ngrams apoco.FreqList, profile string) apoco.StreamFunc {
	if profile == "" {
		return apoco.ConnectLM(c, ngrams)
	}
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		lm := apoco.LanguageModel{Ngrams: ngrams}
		prof, err := apoco.ReadProfile(profile)
		if err != nil {
			return err
		}
		lm.Profile = prof
		return apoco.EachTokenGroup(ctx, in, func(g string, ts ...apoco.T) error {
			for _, t := range ts {
				lm.AddUnigram(t.Tokens[0])
			}
			for i := range ts {
				ts[i].LM = &lm
			}
			return apoco.SendTokens(ctx, out, ts...)
		})
	}
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
