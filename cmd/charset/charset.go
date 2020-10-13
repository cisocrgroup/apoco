package charset

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"unicode"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/pagexml"
	"git.sr.ht/~flobar/apoco/pkg/apoco/snippets"
	"github.com/finkf/gofiler"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var flags = struct {
	ifgs, extensions []string
	mets, parameters string
	nocr             int
	cache            bool
}{}

// CMD runs the apoco charset command.
var CMD = &cobra.Command{
	Use:   "charset [DIRS...]",
	Short: "Extract different character sets",
	Run:   run,
}

func init() {
	CMD.Flags().StringVarP(&flags.mets, "mets", "m", "mets.xml", "set path to the mets file")
	CMD.Flags().StringSliceVarP(&flags.ifgs, "input-file-grp", "I", nil, "set input file groups")
	CMD.Flags().StringSliceVarP(&flags.extensions, "extensions", "e", []string{".xml"},
		"set input file extensions")
	CMD.Flags().StringVarP(&flags.parameters, "parameters", "P", "config.toml",
		"set path to the configuration file")
	CMD.Flags().IntVarP(&flags.nocr, "nocr", "n", 0,
		"set nocr (overwrites setting in the configuration file)")
	CMD.Flags().BoolVarP(&flags.cache, "cache", "c", false, "enable caching of profile")
}

func run(_ *cobra.Command, args []string) {
	c, err := apoco.ReadConfig(flags.parameters)
	chk(err)
	c.Overwrite("", flags.nocr, false, flags.cache)
	g, ctx := errgroup.WithContext(context.Background())
	gt, ocr, cor := make(cset), make(cset), make(cset)
	_ = apoco.Pipe(ctx, g,
		tokenize(flags.mets, flags.ifgs, flags.extensions, args),
		apoco.FilterBad(c.Nocr+1), // at least n ocr + ground truth
		apoco.Normalize,
		apoco.ConnectLM(c, apoco.FreqList{}),
		apoco.ConnectCandidates,
		charset(gt, ocr, cor),
	)
	chk(g.Wait())
	output(gt, ocr, cor)
}

func charset(gt, ocr, cor cset) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan apoco.Token) <-chan apoco.Token {
		g.Go(func() error {
			return apoco.EachToken(ctx, in, func(t apoco.Token) error {
				n := len(t.Tokens)
				gt.add(t.Tokens[n-1])
				for i := 0; i < n-1; i++ {
					ocr.add(t.Tokens[i])
				}
				cor.add(t.Payload.(*gofiler.Candidate).Suggestion)
				return nil
			})
		})
		return nil
	}
}

func output(gt, ocr, cor cset) {
	fmt.Printf("gt:      %s\n", gt)
	fmt.Printf("ocr:     %s\n", ocr)
	fmt.Printf("cor:     %s\n", cor)
	fmt.Printf("gt\\ocr:  %s\n", gt.dif(ocr))
	fmt.Printf("gt\\cor:  %s\n", gt.dif(cor))
	fmt.Printf("ocr\\gt:  %s\n", ocr.dif(gt))
	fmt.Printf("ocr\\cor: %s\n", ocr.dif(cor))
	fmt.Printf("cor\\gt:  %s\n", cor.dif(gt))
	fmt.Printf("cor\\ocr: %s\n", cor.dif(ocr))
}

func tokenize(mets string, ifgs, exts, args []string) apoco.StreamFunc {
	if len(ifgs) != 0 {
		return pagexml.Tokenize(mets, ifgs...)
	}
	if len(exts) == 1 && exts[0] == ".xml" {
		return pagexml.TokenizeDirs(exts[0], args...)
	}
	e := snippets.Extensions(exts)
	return e.Tokenize(args...)
}

type cset map[string]struct{}

func (s cset) add(str string) {
	var b strings.Builder
	for _, r := range str {
		// Combine combining characters with their
		// predecessors.
		if unicode.In(r, unicode.Mc) {
			b.WriteRune(r)
			continue
		}
		s[b.String()] = struct{}{}
		b.Reset()
		b.WriteRune(r)
	}
	// Handle last rune(s).
	s[b.String()] = struct{}{}
}

func (s cset) sort() []string {
	ret := make([]string, len(s))
	i := 0
	for str := range s {
		ret[i] = str
		i++
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i] < ret[j]
	})
	return ret
}

func (s cset) dif(o cset) cset {
	ret := make(cset)
	for str := range s {
		if _, ok := o[str]; !ok {
			ret[str] = struct{}{}
		}
	}
	return ret
}

func (s cset) String() string {
	var b strings.Builder
	for _, str := range s.sort() {
		b.WriteString(str)
	}
	return b.String()
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
