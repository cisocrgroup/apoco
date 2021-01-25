package print

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

// charsetCMD runs the apoco print charset subcommand.
var charsetCMD = &cobra.Command{
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
		var skip, short, lex, cor bool
		var rank int
		var ocr, sug, gt string
		chk(parseDTD(line, &skip, &short, &lex, &cor, &rank, &ocr, &sug, &gt))
		gtset.add(gt)
		ocrset.add(ocr)
		sugset.add(sug)
		if skip {
			continue
		}
		if sug == gt {
			continue
		}
		corrs = append(corrs, corr{ocr, sug, gt, "", cor, false})
	}
	chk(s.Err())
	if flags.json {
		printCharsetJSON(corrs, gtset, ocrset, sugset)
	} else {
		printCharset(corrs, gtset, ocrset, sugset)
	}
}

func printCharset(corrs []corr, gtset, ocrset, sugset cset) {
	for _, c := range corrs {
		chars := gtset.extractNotInSet(c.Sug)
		bad := chars != ""
		fmt.Printf("badchars=%t taken=%t ocr=%s sug=%s gt=%s chars=%s\n",
			bad, c.Taken, c.OCR, c.Sug, c.GT, e(chars))
	}
	fmt.Printf("gtcharset=%s\n", gtset)
	fmt.Printf("ocrcharset=%s\n", ocrset)
	fmt.Printf("sugcharset=%s\n", sugset)
}

func printCharsetJSON(corrs []corr, gtset, ocrset, sugset cset) {
	for i := range corrs {
		corrs[i].Chars = gtset.extractNotInSet(corrs[i].Sug)
		corrs[i].BadChars = corrs[i].Chars != ""
	}
	data := struct {
		Corrs                 []corr
		GTSet, OCRSet, SugSet string
	}{corrs, gtset.String(), ocrset.String(), sugset.String()}
	chk(json.NewEncoder(os.Stdout).Encode(data))
}

type corr struct {
	OCR, Sug, GT, Chars string
	Taken, BadChars     bool
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
