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
	ifgs, exts                             []string
	ofg, mets, model, params, profile, suf string
	nocr, cands                            int
	cache, gt, correct                     bool
}{}

// Cmd runs the apoco correct command.
var Cmd = &cobra.Command{
	Use:   "correct [DIRS...]",
	Short: "Automatically post-correct documents",
	Run:   run,
}

func init() {
	Cmd.Flags().StringSliceVarP(&flags.ifgs, "input-file-grp", "I",
		nil, "set input file groups")
	Cmd.Flags().StringSliceVarP(&flags.exts, "extensions", "e",
		[]string{".xml"}, "set input file extensions")
	Cmd.Flags().StringVarP(&flags.ofg, "output-file-grp", "O",
		"", "set output file group")
	Cmd.Flags().StringVarP(&flags.mets, "mets", "m",
		"mets.xml", "set path to the mets file")
	Cmd.Flags().StringVarP(&flags.params, "parameter", "p",
		"config.toml", "set path to the configuration file")
	Cmd.Flags().StringVarP(&flags.profile, "profile", "f",
		"", "set external profile file")
	Cmd.Flags().StringVarP(&flags.suf, "suffix", "s",
		".cor.txt", "set the suffix for correction snippet files")
	Cmd.Flags().IntVarP(&flags.nocr, "nocr", "n",
		0, "set nocr (overwrites setting in the configuration file)")
	Cmd.Flags().IntVarP(&flags.cands, "cands", "d",
		-1, "output candidates for tokens (0=all, -1=no)")
	Cmd.Flags().StringVarP(&flags.model, "model", "M", "",
		"set model path (overwrites setting in the configuration file)")
	Cmd.Flags().BoolVarP(&flags.cache, "cache", "c", false, "enable caching of profile")
	Cmd.Flags().BoolVarP(&flags.gt, "gt", "g", false, "enable ground-truth data")
	Cmd.Flags().BoolVarP(&flags.correct, "correct", "C", false, "do not output stoks; correct files directly")
}

func run(_ *cobra.Command, args []string) {
	c, err := internal.ReadConfig(flags.params)
	chk(err)
	internal.UpdateInConfig(&c.Model, flags.model)
	internal.UpdateInConfig(&c.Nocr, flags.nocr)
	internal.UpdateInConfig(&c.Cache, flags.cache)
	internal.UpdateInConfig(&c.GT, flags.gt)
	m, err := internal.ReadModel(c.Model, c.LM, false)
	chk(err)
	rrlr, rrfs, err := m.Get("rr", c.Nocr)
	chk(err)
	dmlr, dmfs, err := m.Get("dm", c.Nocr)
	chk(err)
	stoks := make(stokMap)
	p := internal.Piper{
		IFGS: flags.ifgs,
		METS: flags.mets,
		Exts: flags.exts,
		Dirs: args,
	}
	chk(p.Pipe(
		context.Background(),
		apoco.FilterBad(c.Nocr),
		register(stoks),
		apoco.Normalize(),
		addTokens(stoks, flags.gt),
		filterShort(stoks),
		apoco.ConnectLanguageModel(m.LM),
		apoco.ConnectUnigrams(),
		connectProfile(c, m.LM, flags.profile),
		filterLex(stoks),
		apoco.ConnectCandidates(),
		apoco.ConnectRankings(rrlr, rrfs, c.Nocr),
		analyzeRankings(stoks, flags.gt),
		apoco.ConnectCorrections(dmlr, dmfs, c.Nocr),
		correct(stoks),
	))
	apoco.Log("correcting %d pages (%d tokens)", len(stoks), stoks.numberOfTokens())
	// Add additional arguments to the input file groups.
	flags.ifgs = append(args, flags.ifgs...)
	cor, err := mkcorrector(stoks)
	chk(err)
	chk(cor.correct())
}

func mkcorrector(stoks stokMap) (corrector, error) {
	if flags.correct && flags.ofg == "" {
		return snippetCorrector{stoks, flags.exts[0], flags.suf}, nil
	}
	if flags.correct {
		return newMETSCorrector(flags.mets, flags.ofg, stoks, flags.ifgs...)
	}
	return stokCorrector{stoks}, nil
}

func correct(m stokMap) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			stok := m.get(t)
			stok.Skipped = false
			stok.Cor = t.Payload.(apoco.Correction).Conf > 0.5
			stok.Conf = t.Payload.(apoco.Correction).Conf
			stok.Sug = t.Payload.(apoco.Correction).Candidate.Suggestion
			return nil
		})
	}
}

func register(m stokMap) apoco.StreamFunc {
	// Register each (unnormalized) token and asign each token
	// the raw OCR-token and an ordering.
	order := 0
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			stok := m.get(t)
			stok.raw = t.Tokens[0]
			stok.order = order
			order++
			if err := apoco.SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("register: %v", err)
			}
			return nil
		})
	}
}

func addTokens(m stokMap, withGT bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			// Each token gets its ID. It is skipped by default. If a token
			// should not be skipped, skipped must be explicitly set to false.
			stok := m.get(t)
			stok.Stok = internal.MakeStokFromT(t, withGT)
			stok.ID = t.ID
			stok.document = t.Document
			stok.Skipped = true
			if err := apoco.SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("add tokens: %v", err)
			}
			return nil
		})
	}
}

func filterLex(m stokMap) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			if t.IsLexiconEntry() {
				m.get(t).Lex = true
				return nil
			}
			m.get(t).Lex = t.ContainsLexiconEntry()
			if err := apoco.SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("filterLex: %v", err)
			}
			return nil
		})
	}
}

func filterShort(m stokMap) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			if utf8.RuneCountInString(t.Tokens[0]) <= 3 {
				m.get(t).Short = true
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
			info := m.get(t)
			info.rankings = t.Payload.([]apoco.Ranking)
			if withGT {
				var rank int
				for i, r := range info.rankings {
					if r.Candidate.Suggestion == t.Tokens[len(t.Tokens)-1] {
						rank = i + 1
						break
					}
				}
				m.get(t).Rank = rank
			}
			if err := apoco.SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("analyzeRankings: %v", err)
			}
			return nil
		})
	}
}

func connectProfile(c *internal.Config, lm map[string]*apoco.FreqList, profile string) apoco.StreamFunc {
	if profile == "" {
		return internal.ConnectProfile(c, "-profiler.json.gz")
	}
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		profile, err := apoco.ReadProfile(profile)
		if err != nil {
			return err
		}
		return apoco.ConnectProfile(profile)(ctx, in, out)
	}
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
