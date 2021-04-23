package diagram

import (
	"bufio"
	"fmt"
	"log"
	"os"
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
			log.Printf("line=%q, fields=%q", s.Text(), fields[1])
			_, err := fmt.Sscanf(fields[1], "%g/%g", &ocr, &other)
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

	p := plot.New()
	p.Title.Text = "myplot"
	for name := range data {
		var vals plotter.Values
		for _, val := range data[name] {
			vals = append(vals, val.data)
		}
		bars, err := plotter.NewBarChart(vals, 15)
		chk(err)
		p.Add(bars)
	}
	chk(p.Save(3*vg.Inch, 3*vg.Inch, name+".png"))
}

type pair struct {
	name string
	data float64
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
