package correct

import (
	"context"
	"fmt"
	"log"
	"sort"
	"unicode/utf8"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/spf13/cobra"
)

var flags = struct {
	ifgs, extensions                     []string
	ofg, mets, model, parameter, profile string
	nocr, cands                          int
	cache, gt                            bool
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
	CMD.Flags().IntVarP(&flags.cands, "cands", "d",
		-1, "output candidatess for tokens; 0 for all candidates, -1 for no candidates")
	CMD.Flags().StringVarP(&flags.model, "model", "M", "",
		"set model path (overwrites setting in the configuration file)")
	CMD.Flags().BoolVarP(&flags.cache, "cache", "c", false, "enable caching of profile")
	CMD.Flags().BoolVarP(&flags.gt, "gt", "g", false, "enable ground-truth data")
}

func run(_ *cobra.Command, args []string) {
	c, err := internal.ReadConfig(flags.parameter)
	chk(err)
	internal.UpdateInConfig(&c.Model, flags.model)
	internal.UpdateInConfig(&c.Nocr, flags.nocr)
	internal.UpdateInConfig(&c.Cache, flags.cache)
	internal.UpdateInConfig(&c.GT, flags.gt)
	m, err := internal.ReadModel(c.Model, c.LM)
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
		register(stoks, flags.gt),
		filterShort(stoks, flags.gt),
		apoco.ConnectLanguageModel(m.LM),
		apoco.ConnectUnigrams(),
		connectProfile(c, m.LM, flags.profile),
		filterLex(stoks, flags.gt),
		apoco.ConnectCandidates(),
		apoco.ConnectRankings(rrlr, rrfs, c.Nocr),
		analyzeRankings(stoks, flags.gt),
		apoco.ConnectCorrections(dmlr, dmfs, c.Nocr),
		correct(stoks, flags.gt),
	))
	apoco.Log("correcting %d pages (%d tokens)", len(stoks), stoks.numberOfTokens())
	// If no output file group is given, we do not need to correct
	// the according page XML files.  We just output the stoks according
	// to their ordering.  So if any input file group is given, we output
	// the stoks.  Only if an output file group is given, we do correct
	// the according page XML files within the output file group.
	if flags.ofg == "" {
		sorted := make([]*stok, 0, len(stoks))
		for _, ids := range stoks {
			for _, info := range ids {
				sorted = append(sorted, info)
			}
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].order < sorted[j].order
		})
		var doc *apoco.Document
		for _, info := range sorted {
			if info.document != doc {
				fmt.Printf("#name=%s\n", info.document.Group)
				doc = info.document
			}
			switch {
			case flags.cands == -1:
				fmt.Printf("%s\n", info.Stok)
			case len(info.rankings) > 0:
				fmt.Printf("%s cands=%s\n", info.Stok, rankings2string(info.rankings, flags.cands))
			default:
				i := info.document.Profile[info.OCR]
				fmt.Printf("%s cands=%s\n", info.Stok, candidates2string(i.Candidates, flags.cands))
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
	order := 0
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			// Each token gets its ID, its order and is skipped as default.
			// If a token is not skipped, skipped must be explicitly set to false.
			stok := m.get(t, withGT)
			stok.ID = t.ID
			stok.document = t.Document
			stok.Skipped = true
			stok.order = order
			order++
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
			m.get(t, withGT).Lex = t.ContainsLexiconEntry()
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
			info := m.get(t, withGT)
			info.rankings = t.Payload.([]apoco.Ranking)
			if withGT {
				var rank int
				for i, r := range info.rankings {
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
