package print

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"github.com/spf13/cobra"
)

// CMD defines the apoco train command.
var datCMD = &cobra.Command{
	Use:   "dat [FILES...]",
	Short: "Print data for gnuplot",
	Run:   run,
	Long: `
Prints the data for gnuplot from FILES. Reads
from stdin, if no FILES.`,
}

var datFlags = struct {
	typ      string
	replace  string
	limit    int
	noshorts bool
}{}

func init() {
	datCMD.Flags().StringVarP(&datFlags.typ, "type", "t", "acc", "set type of evaluation")
	datCMD.Flags().StringVarP(&datFlags.replace, "substitute", "e", "",
		"set expression applied to file names (sed s/// syntax)")
	datCMD.Flags().BoolVarP(&datFlags.noshorts, "noshort", "s", false,
		"exclude short tokens (len<4) from the evaluation")
	datCMD.Flags().IntVarP(&datFlags.limit, "limit", "m", 0, "set candidate limit")
	CMD.AddCommand(datCMD)
}

func run(_ *cobra.Command, args []string) {
	switch datFlags.typ {
	case "acc":
		replacer, err := newReplacer(datFlags.replace)
		chk(err)
		acc{replacer, datFlags.noshorts}.run(args)
	case "err":
		err{datFlags.limit, datFlags.noshorts}.run(args)
	default:
		panic("bad type: " + datFlags.typ)
	}
}

type acc struct {
	replacer replacer
	noshorts bool
}

func (a acc) run(files []string) {
	data := make(map[string][]accPair)
	var fyear, fsuf string
	var before, after, total int
	eachStok(files, func(year, suf string, new bool, stok internal.Stok) {
		if new {
			if total > 0 {
				addpairs(data, fyear, fsuf, before, after, total)
				before, after, total = 0, 0, 0
			}
			fyear, fsuf = year, a.replacer.replace(suf)
		}
		if stok.Short && a.noshorts {
			return
		}
		total++
		if stok.ErrBefore() {
			before++
		}
		if stok.ErrAfter() {
			after++
		}
	})
	if total > 0 {
		addpairs(data, fyear, fsuf, before, after, total)
	}

	// Sort keys for a consistent order
	years := make([]string, 0, len(data))
	for year := range data {
		years = append(years, year)
	}
	sort.Slice(years, func(i, j int) bool {
		return years[i] < years[j]
	})
	var max int
	for _, year := range years {
		if max < len(data[year]) {
			max = len(data[year])
		}
	}

	for i := 0; i < max; i++ {
		if i == 0 {
			fmt.Print("#")
			for _, name := range years {
				fmt.Printf(" %s", name)
			}
			fmt.Println()
		}
		fmt.Printf("%q", data[years[0]][i].name)
		for _, year := range years {
			fmt.Printf(" %g", 1-data[year][i].data)
		}
		fmt.Println()
	}
}

func addpairs(data map[string][]accPair, name, suf string, before, after, total int) {
	if len(data[name]) == 0 {
		data[name] = append(data[name], accPair{"OCR", float64(before) / float64(total)})
	}
	data[name] = append(data[name], accPair{suf, float64(after) / float64(total)})
}

type accPair struct {
	name string
	data float64
}

type err struct {
	limit    int
	noshorts bool
}

const (
	infelc = "infel c (2c)"
	falsef = "false friends (1d)"
	missop = "missed op (2b)"
	shorte = "short erros (1a)"
	badlim = "bad limit (1c)"
	badrnk = "bad rank (2a)"
	miscor = "missing c (1b)"
)

func (e err) run(files []string) {
	data := make(map[string]map[string]int)
	eachStok(files, func(year, suf string, new bool, stok internal.Stok) {
		if new {
			data[year] = make(map[string]int)
		}
		if stok.Short && e.noshorts || !stok.ErrAfter() {
			return
		}

		data[year]["total"]++

		switch stok.Type() {
		case internal.InfelicitousCorrection:
			data[year][infelc]++
		case internal.FalseFriend:
			data[year][falsef]++
			return // Do not count causes of false friends.
		case internal.MissedOpportunity:
			data[year][missop]++
		case internal.SkippedShortErr:
			data[year][shorte]++
			return // Do not count causes of short errors.
		}
		switch stok.Cause(e.limit) {
		case internal.BadLimit:
			data[year][badlim]++
		case internal.MissingCandidate:
			data[year][miscor]++
		case internal.BadRank:
			data[year][badrnk]++
		}
	})
	e.print(data)
}

func (e err) print(data map[string]map[string]int) {
	years := make([]string, 0, len(data))
	for year := range data {
		years = append(years, year)
	}
	sort.Slice(years, func(i, j int) bool {
		return years[i] < years[j]
	})
	fmt.Printf("#")
	for _, year := range years {
		fmt.Printf(" %s", year)
	}
	fmt.Println()
	names := []string{shorte, miscor, badlim, falsef, badrnk, missop, infelc}
	if e.noshorts {
		names = names[1:]
	}
	for _, t := range names {
		fmt.Printf("%q", t)
		for _, y := range years {
			fmt.Printf(" %g", float64(data[y][t])/float64(data[y]["total"]))
		}
		fmt.Println()
	}
	// Absolute values.
	for _, t := range names {
		fmt.Printf("# [%s]", t)
		for _, y := range years {
			fmt.Printf(" %d (%d)", data[y][t], data[y]["total"])
		}
		fmt.Println()
	}
}

func eachStok(files []string, f func(string, string, bool, internal.Stok)) {
	if len(files) == 0 {
		eachStokReader(os.Stdin, f)
		return
	}
	for _, file := range files {
		eachStokInFile(file, f)
	}
}

func eachStokInFile(name string, f func(string, string, bool, internal.Stok)) {
	r, err := os.Open(name)
	chk(err)
	defer r.Close()
	year, suf, err := splitName(name)
	chk(err)
	new := true
	chk(internal.EachStok(r, func(name string, stok internal.Stok) error {
		f(year, suf, new, stok)
		new = false
		return nil
	}))
	eachStokReader(r, f)
}

func eachStokReader(r io.Reader, f func(string, string, bool, internal.Stok)) {
	var fname, year, suf string
	chk(internal.EachStok(r, func(name string, stok internal.Stok) error {
		if fname == "" || fname != name {
			var err error
			fname = name
			year, suf, err = splitName(name)
			f(year, suf, true, stok)
			return err
		}
		f(year, suf, false, stok)
		return nil
	}))
}

func splitName(name string) (string, string, error) {
	name = filepath.Base(name)
	pos := strings.Index(name, ".")
	if pos != -1 {
		name = name[:pos]
	}
	if len(name) < 6 {
		return "", "", fmt.Errorf("bad name %s: too short", name)
	}
	return name[:4], name[5:], nil
}

type replacer interface {
	replace(string) string
}

func newReplacer(expr string) (replacer, error) {
	if expr == "" {
		return noop{}, nil
	}
	if len(expr) < 4 || !strings.HasPrefix(expr, "s") {
		return nil, fmt.Errorf("invalid subustitute expression %q", expr)
	}
	fields := strings.Split(expr, expr[1:2])
	if len(fields) != 4 {
		return nil, fmt.Errorf("invalid expression %q", expr)
	}
	if fields[0] != "s" {
		return nil, fmt.Errorf("invalid expression %q", expr)
	}
	re, err := regexp.Compile(fields[1])
	if err != nil {
		return nil, fmt.Errorf("invalid expression %q: %v", expr, err)
	}
	return substitute{re: re, repl: fields[2]}, nil
}

type substitute struct {
	re   *regexp.Regexp
	repl string
}

func (s substitute) replace(str string) string {
	return s.re.ReplaceAllString(str, s.repl)
}

type noop struct{}

func (noop) replace(str string) string { return str }
