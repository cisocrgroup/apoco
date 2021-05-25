package print

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"github.com/spf13/cobra"
)

// CMD defines the apoco train command.
var datCMD = &cobra.Command{
	Use:   "dat",
	Short: "Print data for gnuplot",
	Run:   run,
}

func run(_ *cobra.Command, _ []string) {
	s := bufio.NewScanner(os.Stdin)
	data := make(map[string][]pair)
	var name, suf string
	var before, after, total int
	for s.Scan() {
		line := s.Text()
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#name=") {
			if len(data) > 0 {
				addpairs(data, name, suf, before, after, total)
			}
			tmp := line[6:]
			pos := strings.Index(tmp, "-")
			if pos < 1 {
				panic(fmt.Sprintf("bad name: %s", tmp))
			}
			name, suf = tmp[:pos], tmp[pos+1:]
			before, after, total = 0, 0, 0
			continue
		}
		stok, err := internal.MakeStok(line)
		chk(err)
		total++
		if stok.ErrBefore() {
			before++
		}
		if stok.ErrAfter() {
			after++
		}
	}
	chk(s.Err())
	addpairs(data, name, suf, before, after, total)

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
