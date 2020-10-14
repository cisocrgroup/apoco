package charset

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

var flags = struct {
	ifgs, extensions []string
	mets, parameters string
	nocr             int
	cache, normalize bool
}{}

// CMD runs the apoco charset command.
var CMD = &cobra.Command{
	Use:   "charset",
	Short: "Extract differences in character sets",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	s := bufio.NewScanner(os.Stdin)
	gtset := make(cset)
	var corrs []struct {
		gt, sug string
		taken   bool
	}
	for s.Scan() {
		var skip, short, lex, cor bool
		var rank int
		var ocr, sug, gt string
		chk(parseDTD(s.Text(), &skip, &short, &lex, &cor, &rank, &ocr, &sug, &gt))
		gtset.add(gt)
		if sug == gt {
			continue
		}
		corrs = append(corrs, struct {
			gt, sug string
			taken   bool
		}{gt, sug, cor})
	}
	chk(s.Err())
	for _, cor := range corrs {
		bad := gtset.extractNotInSet(cor.sug)
		fmt.Printf("bad=%q taken=%t %s %s", bad, cor.taken, cor.gt, cor.sug)
	}
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

func parseDTD(dtd string, skip, short, lex, cor *bool, rank *int, ocr, sug, gt *string) error {
	const dtdFormat = "skipped=%t short=%t lex=%t cor=%t rank=%d ocr=%s sug=%s gt=%s"
	_, err := fmt.Sscanf(dtd, dtdFormat, skip, short, lex, cor, rank, ocr, sug, gt)
	if err != nil {
		return fmt.Errorf("parseDTD: cannot parse %q: %v", dtd, err)
	}
	return nil
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
