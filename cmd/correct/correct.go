package correct

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"unicode/utf8"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/pagexml"
	"git.sr.ht/~flobar/apoco/pkg/apoco/snippets"
	"github.com/finkf/gofiler"
	"github.com/spf13/cobra"
)

var flags = struct {
	ifgs, extensions                      []string
	ofg, mets, model, parameters, profile string
	nocr                                  int
	cache                                 bool
}{}

// CMD runs the apoco correct command.
var CMD = &cobra.Command{
	Use:   "correct [DIRS...]",
	Short: "Automatically correct documents",
	Run:   run,
}

func init() {
	var loglevel string
	CMD.Flags().StringVarP(&loglevel, "log-level", "l", "INFO", "set log level [ignored]")
	CMD.Flags().StringSliceVarP(&flags.ifgs, "input-file-grp", "I", nil, "set input file groups")
	CMD.Flags().StringSliceVarP(&flags.extensions, "extensions", "e", []string{".xml"},
		"set input file extensions")
	CMD.Flags().StringVarP(&flags.ofg, "output-file-grp", "O", "", "set output file group")
	CMD.Flags().StringVarP(&flags.mets, "mets", "m", "mets.xml", "set path to the mets file")
	CMD.Flags().StringVarP(&flags.parameters, "parameters", "P", "config.toml",
		"set path to the configuration file")
	CMD.Flags().StringVarP(&flags.profile, "profile", "p", "", "set external profile file")
	CMD.Flags().IntVarP(&flags.nocr, "nocr", "n", 0,
		"set nocr (overwrites setting in the configuration file)")
	CMD.Flags().StringVarP(&flags.model, "model", "M", "",
		"set model path (overwrites setting in the configuration file)")
	CMD.Flags().BoolVarP(&flags.cache, "cache", "c", false, "enable caching of profile")
}

func run(_ *cobra.Command, args []string) {
	c, err := apoco.ReadConfig(flags.parameters)
	chk(err)
	c.Overwrite(flags.model, flags.nocr, false, flags.cache)
	m, err := apoco.ReadModel(c.Model, c.Ngrams)
	chk(err)
	rrlr, rrfs, err := m.Get("rr", c.Nocr)
	chk(err)
	dmlr, dmfs, err := m.Get("dm", c.Nocr)
	chk(err)
	infoMap := make(infoMap)
	chk(pipe(context.Background(),
		flags.mets,
		flags.ifgs,
		flags.extensions,
		args,
		apoco.FilterBad(c.Nocr+1), // at least n ocr + ground truth
		apoco.Normalize(),
		register(infoMap),
		filterShort(infoMap),
		connectlm(c, m.Ngrams, flags.profile),
		filterLex(infoMap),
		apoco.ConnectCandidates(),
		apoco.ConnectRankings(rrlr, rrfs, c.Nocr),
		analyzeRankings(infoMap),
		apoco.ConnectCorrections(dmlr, dmfs, c.Nocr),
		correct(infoMap),
	))
	log.Printf("correcting %d pages (%d tokens)", len(infoMap), infoMap.numberOfTokens())
	if len(flags.ifgs) == 0 {
		for _, ids := range infoMap {
			for _, info := range ids {
				fmt.Printf("%s\n", info)
			}
		}
		return
	}
	cor := corrector{
		info: infoMap,
		ifgs: append(args, flags.ifgs...),
		ofg:  flags.ofg,
	}
	chk(cor.correct(flags.mets))
}

func correct(m infoMap) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			info := m.get(t)
			info.skipped = false
			info.cor = t.Payload.(apoco.Correction).Conf > 0.5
			info.conf = t.Payload.(apoco.Correction).Conf
			info.sug = t.Payload.(apoco.Correction).Candidate.Suggestion
			return nil
		})
	}
}

func register(m infoMap) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			// Each token is skipped as default.
			// If a token is not skipped, skipped
			// must be explicitly set to false.
			m.get(t).skipped = true
			if err := apoco.SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("register: %v", err)
			}
			return nil
		})
	}
}

func filterLex(m infoMap) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			if t.IsLexiconEntry() {
				m.get(t).lex = true
				return nil
			}
			if err := apoco.SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("filterLex: %v", err)
			}
			return nil
		})
	}
}

func filterShort(m infoMap) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			if utf8.RuneCountInString(t.Tokens[0]) <= 3 {
				m.get(t).short = true
				return nil
			}
			if err := apoco.SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("filterShort: %v", err)
			}
			return nil
		})
	}
}

func analyzeRankings(m infoMap) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
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
	}
}

func connectlm(c *apoco.Config, ngrams apoco.FreqList, profile string) apoco.StreamFunc {
	if profile == "" {
		return apoco.ConnectLM(c, ngrams)
	}
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		lm := apoco.LanguageModel{Ngrams: ngrams}
		prof, err := readProfile(profile)
		if err != nil {
			return err
		}
		lm.Profile = prof
		return apoco.EachToken(ctx, in, func(t apoco.T) error {
			t.LM = &lm
			t.LM.AddUnigram(t.Tokens[0])
			return apoco.SendTokens(ctx, out, t)
		})
	}
}

func readProfile(name string) (gofiler.Profile, error) {
	in, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("read profile %s: %v", name, err)
	}
	defer in.Close()
	r, err := gzip.NewReader(in)
	if err != nil {
		return nil, fmt.Errorf("read profile %s: %v", name, err)
	}
	var profile gofiler.Profile
	if err := json.NewDecoder(r).Decode(&profile); err != nil {
		return nil, fmt.Errorf("read profile %s: %v", name, err)
	}
	return profile, nil
}

func pipe(ctx context.Context, mets string, ifgs, exts, dirs []string, fns ...apoco.StreamFunc) error {
	if len(ifgs) != 0 {
		fns = append([]apoco.StreamFunc{pagexml.Tokenize(mets, ifgs...)}, fns...)
	} else if len(exts) == 1 && exts[0] == ".xml" {
		fns = append([]apoco.StreamFunc{pagexml.TokenizeDirs(exts[0], dirs...)}, fns...)
	} else {
		e := snippets.Extensions(exts)
		fns = append([]apoco.StreamFunc{e.ReadLines(dirs...), e.TokenizeLines}, fns...)
	}
	return apoco.Pipe(ctx, fns...)
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
