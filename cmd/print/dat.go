package print

import (
	"bufio"
	"fmt"
	"io"
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
	typ string
}{}

func init() {
	datCMD.Flags().StringVarP(&datFlags.typ, "type", "t", "general", "set type of data")
	CMD.AddCommand(datCMD)
}

func run(_ *cobra.Command, args []string) {
	switch datFlags.typ {
	case "general":
		runGeneral(args)
	default:
		panic("bad type: " + datFlags.typ)
	}
}

func runGeneral(files []string) {
	data := make(map[string][]pair)
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

func eachStok(files []string, f func(string, internal.Stok)) {
	if len(files) == 0 {
		eachStokR(os.Stdin, f)
		return
	}
	for _, file := range files {
		eachStokF(file, f)
	}
}

func eachStokF(file string, f func(string, internal.Stok)) {
	r, err := os.Open(file)
	chk(err)
	defer r.Close()
	eachStokR(r, f)
}

func eachStokR(r io.Reader, f func(string, internal.Stok)) {
	s := bufio.NewScanner(r)
	var name string
	for s.Scan() {
		line := s.Text()
		if len(line) == 0 {
			continue
		}
		if strings.HasPrefix(line, "#name=") {
			name = strings.Trim(line[6:], " \t\n")
			continue
		}
		if line[0] == '#' {
			continue
		}
		stok, err := internal.MakeStok(line)
		chk(err)
		f(name, stok)
	}
	chk(s.Err())
}

func addpairs(data map[string][]pair, name, suf string, before, after, total int) {
	if len(data[name]) == 0 {
		data[name] = append(data[name], pair{"OCR", float64(before) / float64(total)})
	}
	data[name] = append(data[name], pair{suf, float64(after) / float64(total)})
}

type pair struct {
	name string
	data float64
}
