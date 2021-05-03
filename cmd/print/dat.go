package print

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// CMD defines the apoco train command.
var datCMD = &cobra.Command{
	Use:   "dat",
	Short: "Print data for diagrams",
	Run:   run,
}

func run(_ *cobra.Command, _ []string) {
	s := bufio.NewScanner(os.Stdin)
	data := make(map[string][]pair)
	var name, suf string
	var max int
	for s.Scan() {
		if strings.HasPrefix(s.Text(), "Name") {
			fields := strings.Fields(s.Text())
			split := strings.Split(fields[2], "-")
			name, suf = split[0], split[1]
		}
		if strings.HasPrefix(s.Text(), "Accuracy") {
			fields := strings.Split(s.Text(), "=")
			var ocr, other float64
			_, err := fmt.Sscanf(fields[1], "%g/%g", &ocr, &other)
			// log.Printf("%s-%s: %g/%g", name, suf, ocr, other)
			chk(err)
			if len(data[name]) == 0 {
				data[name] = append(data[name], pair{"OCR", ocr})
			}
			data[name] = append(data[name], pair{suf, other})
		}
		if len(data[name]) > max {
			max = len(data[name])
		}
	}
	chk(s.Err())

	// Sort keys for a consistent order
	names := make([]string, 0, len(data))
	for name := range data {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return names[i] < names[j]
	})

	for i := 0; i < max; i++ {
		if i == 0 {
			fmt.Print("#")
			for _, name := range names {
				fmt.Printf(" %s", name)
			}
			fmt.Println()
		}
		fmt.Print(data["1487"][i].name)
		for _, name := range names {
			fmt.Printf(" %g", data[name][i].data)
		}
		fmt.Println()
	}
}

type pair struct {
	name string
	data float64
}
