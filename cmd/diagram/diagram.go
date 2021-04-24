package diagram

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

var CMD = &cobra.Command{
	Use:   "diagram",
	Short: "write digrams",
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
		if strings.HasPrefix(s.Text(), "Char") {
			fields := strings.Split(s.Text(), "=")
			var ocr, other float64
			_, err := fmt.Sscanf(fields[1], "%g/%g", &ocr, &other)
			log.Printf("%s-%s: %g/%g", name, suf, ocr, other)
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

	// Plot the data
	p := plot.New()
	p.Title.Text = "xy"
	var i int
	for name := range data {
		var vals plotter.Values
		for _, pair := range data[name] {
			vals = append(vals, pair.d())
		}
		log.Printf("%s: %v", name, vals)
		bars, err := plotter.NewBarChart(vals, 2)
		chk(err)
		bars.XMin = float64(i * (len(data[name]) + 1))
		p.Add(bars)
		i++
	}
	chk(p.Save(5*vg.Inch, 3*vg.Inch, "hist.png"))
}

type pair struct {
	name string
	data float64
}

func (p pair) d() float64 {
	// return 1 - p.data
	return p.data
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
