package print

import (
	"fmt"
	"os"
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
from stdin, if no FILES.
	`,
}

var datFlags = struct {
	typ    string
	limit  int
	shorts bool
}{}

func init() {
	datCMD.Flags().StringVarP(&datFlags.typ, "type", "t", "acc", "set type of data")
	datCMD.Flags().BoolVarP(&datFlags.shorts, "shorts", "s", false, "consider short errors")
	datCMD.Flags().IntVarP(&datFlags.limit, "limit", "m", 0, "set candidate limit")
	CMD.AddCommand(datCMD)
}

func run(_ *cobra.Command, args []string) {
	switch datFlags.typ {
	case "acc":
		runAcc(args)
	case "err":
		runErr(datFlags.limit, datFlags.shorts, args)
	default:
		panic("bad type: " + datFlags.typ)
	}
}

func runAcc(files []string) {
	data := make(map[string][]accPair)
	var year, suf string
	var before, after, total int
	eachStok(files, func(name string, stok internal.Stok) {
		if year == "" || !strings.HasPrefix(name, year) {
			pos := strings.Index(name, "-")
			if pos < 1 {
				panic("bad name: " + name)
			}
			year, suf = name[:pos], name[pos+1:]
			before, after, total = 0, 0, 0
		}
		total++
		if stok.ErrBefore() {
			before++
		}
		if stok.ErrAfter() {
			after++
		}
	})
	addpairs(data, year, suf, before, after, total)

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

func runErr(limit int, shorts bool, files []string) {
	var year string
	data := make(map[string]map[string]int)
	eachStok(files, func(name string, stok internal.Stok) {
		if len(name) >= 4 && (year == "" || !strings.HasPrefix(name, year)) {
			year = name[0:4]
			data[year] = make(map[string]int)
		}
		if stok.Short && !shorts {
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
			switch stok.Cause(limit) {
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
	printErrs(shorts, data)
}

func printErrs(shorts bool, data map[string]map[string]int) {
	fmt.Printf("#")
	for k := range data {
		fmt.Printf(" %s", k)
	}
	fmt.Println()
	names := []string{"short errors", "missing c", "bad limit", "bad rank", "missed op", "infel c"}
	if !shorts {
		names = names[1:]
	}
	for _, t := range names {
		fmt.Printf("%q", t)
		for k := range data {
			fmt.Printf(" %g", float64(data[k][t])/float64(data[k]["total"]))
		}
		fmt.Println()
	}
}

func eachStok(files []string, f func(string, internal.Stok)) {
	if len(files) == 0 {
		chk(internal.EachStok(os.Stdin, f))
		return
	}
	for _, file := range files {
		eachStokInFile(file, f)
	}
}

func eachStokInFile(file string, f func(string, internal.Stok)) {
	r, err := os.Open(file)
	chk(err)
	defer r.Close()
	chk(internal.EachStok(r, f))
}
