package print

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

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
from stdin, if no FILES.
	`,
}

var datFlags = struct {
	typ    string
	limit  int
	noshorts bool
}{}

func init() {
	datCMD.Flags().StringVarP(&datFlags.typ, "type", "t", "acc", "set type of data")
	datCMD.Flags().BoolVarP(&datFlags.noshorts, "noshort", "s", false,
		"exclude short tokens (len<4) from the evaluation")
	datCMD.Flags().IntVarP(&datFlags.limit, "limit", "m", 0, "set candidate limit")
	CMD.AddCommand(datCMD)
}

func run(_ *cobra.Command, args []string) {
	switch datFlags.typ {
	case "acc":
		acc{datFlags.noshorts}.run(args)
	case "err":
		err{datFlags.limit, datFlags.noshorts}.run(args)
	default:
		panic("bad type: " + datFlags.typ)
	}
}

type acc struct{
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
			fyear, fsuf = year, suf
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
	names := make([]string, 0, len(data))
	for name := range data {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return names[i] < names[j]
	})
	var max int
	for _, name := range names {
		if max < len(data[name]) {
			max = len(data[name])
		}
	}

	for i := 0; i < max; i++ {
		if i == 0 {
			fmt.Print("#")
			for _, name := range names {
				fmt.Printf(" %s", name)
			}
			fmt.Println()
		}
		fmt.Printf("%s", data[names[0]][i].name)
		for _, name := range names {
			fmt.Printf(" %g", 1-data[name][i].data)
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
	limit  int
	noshorts bool
}

func (e err) run(files []string) {
	data := make(map[string]map[string]int)
	eachStok(files, func(year, suf string, new bool, stok internal.Stok) {
		if new {
			data[year] = make(map[string]int)
		}
		if stok.Short && e.noshorts {
			return
		}
		switch stok.Type() {
		case internal.InfelicitousCorrection:
			data[year]["infel c"]++
		case internal.FalseFriend:
			data[year]["false friends"]++
		case internal.MissedOpportunity:
			data[year]["missed op"]++
		}
		if stok.ErrAfter() {
			if stok.Short {
				data[year]["short errors"]++
			}
			switch stok.Cause(e.limit) {
			case internal.BadLimit:
				data[year]["bad limit"]++
			case internal.MissingCandidate:
				data[year]["missing c"]++
			case internal.BadRank:
				data[year]["bad rank"]++
			}
			data[year]["total"]++
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
	for k := range data {
		fmt.Printf(" %s", k)
	}
	fmt.Println()
	names := []string{"short errors", "missing c", "bad limit", "bad rank", "missed op", "infel c"}
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

func eachStokInFile(file string, f func(string, string, bool, internal.Stok)) {
	r, err := os.Open(file)
	chk(err)
	defer r.Close()
	eachStokReader(r, f)
}

func eachStokReader(r io.Reader, f func(string, string, bool, internal.Stok)) {
	var fname, year, suf string
	chk(internal.EachStok(r, func(name string, stok internal.Stok) {
		if fname == "" || fname != name {
			fname = name
			name = filepath.Base(name)
			year, suf = name[0:4], name[5:]
			f(year, suf, true, stok)
			return
		}
		f(year, suf, false, stok)
	}))
}
