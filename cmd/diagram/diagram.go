package diagram

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
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
			fields := strings.Fields(s.Text())
			var ocr, other float64
			log.Printf("line=%q, fields=%q", s.Text(), fields[2])
			_, err := fmt.Sscanf(fields[2], "%g/%g", &ocr, &other)
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
	for i := 0; i < max; i++ {
		for name := range data {
			if i == 0 {
				fmt.Print(name)
			}
			fmt.Printf(" %g", data[name][i].data)
		}
		fmt.Println()
	}
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
