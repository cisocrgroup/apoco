package print

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"github.com/spf13/cobra"
)

// charsetCmd runs the apoco print charset subcommand.
var charsetCmd = &cobra.Command{
	Use:   "charset",
	Short: "Extract differences in character sets",
	Run:   runCharset,
}

func runCharset(_ *cobra.Command, args []string) {
	s := bufio.NewScanner(os.Stdin)
	gtset := make(cset)
	ocrset := make(cset)
	sugset := make(cset)
	var corrs []corr
	for s.Scan() {
		line := s.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		t, err := internal.MakeStokFromLine(line)
		chk(err)
		gtset.add(t.GT)
		ocrset.add(t.OCR)
		sugset.add(t.Sug)
		if t.Skipped {
			continue
		}
		if t.Sug == t.GT {
			continue
		}
		corrs = append(corrs, corr{t.OCR, t.Sug, t.GT, "", t.Cor, false})
	}
	chk(s.Err())
	if flags.json {
		printCharsetJSON(corrs, gtset, ocrset, sugset)
	} else {
		printCharset(corrs, gtset, ocrset, sugset)
	}
}

func printCharset(corrs []corr, gtset, ocrset, sugset cset) {
	e := internal.E
	for _, c := range corrs {
		chars := gtset.extractNotInSet(c.Sug)
		bad := chars != ""
		fmt.Printf("hasbadchars=%t taken=%t ocr=%s sug=%s gt=%s badchars=%s\n",
			bad, c.Taken, e(c.OCR), e(c.Sug), e(c.GT), e(chars))
	}
	fmt.Printf("gtcharset=%s\n", e(gtset.String()))
	fmt.Printf("ocrcharset=%s\n", e(ocrset.String()))
	fmt.Printf("sugcharset=%s\n", e(sugset.String()))
}

func printCharsetJSON(corrs []corr, gtset, ocrset, sugset cset) {
	for i := range corrs {
		corrs[i].BadChars = gtset.extractNotInSet(corrs[i].Sug)
		corrs[i].HasBadChars = corrs[i].BadChars != ""
	}
	data := struct {
		Corrs                 []corr
		GTSet, OCRSet, SugSet string
	}{corrs, gtset.String(), ocrset.String(), sugset.String()}
	chk(json.NewEncoder(os.Stdout).Encode(data))
}

type corr struct {
	OCR, Sug, GT, BadChars string
	Taken, HasBadChars     bool
}

type cset map[string]struct{}

func (s cset) add(str string) {
	for g, r := nextGlyph(str); r != ""; g, r = nextGlyph(r) {
		s[g] = struct{}{}
	}
}

func nextGlyph(str string) (string, string) {
	if str == "" {
		return "", ""
	}
	var b strings.Builder
	for i, r := range str {
		isComb := unicode.In(r, unicode.M)
		if i == 0 {
			if isComb {
				b.WriteRune('◌')
			}
			b.WriteRune(r)
			continue
		}
		if isComb {
			b.WriteRune(r)
			continue
		}
		return b.String(), str[i:]
	}
	return b.String(), ""
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

func (s cset) extractNotInSet(str string) string {
	var b strings.Builder
	for g, r := nextGlyph(str); r != ""; g, r = nextGlyph(r) {
		if _, ok := s[g]; !ok {
			b.WriteString(g)
		}
	}
	return b.String()
}

func (s cset) String() string {
	var b strings.Builder
	for _, str := range s.sort() {
		b.WriteString(str)
	}
	return b.String()
}
