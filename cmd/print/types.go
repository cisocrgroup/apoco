package print

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"github.com/spf13/cobra"
)

// typesCMD runs the apoco print errtypes.
var typesCMD = &cobra.Command{
	Use:   "types [DIRS...]",
	Short: "Augment stat tokens with types",
	Args:  cobra.ExactArgs(0),
	Run:   runTypes,
}

func runTypes(_ *cobra.Command, _ []string) {
	switch {
	case flags.json:
		printTypesJSON()
	default:
		printTypes()
	}
}

func printTypesJSON() {
	stoks := make(map[string][]stok)
	var name string
	eachLine(func(line string) {
		switch {
		case strings.HasPrefix(line, "#name="):
			_, err := fmt.Sscanf(line, "#name=%s", &name)
			chk(err)
		default:
			s, err := internal.MakeStok(line)
			chk(err)
			stoks[name] = append(stoks[name], stok{Stok: s, Type: typ(s)})
		}
	})
	chk(json.NewEncoder(os.Stdout).Encode(stoks))
}

func printTypes() {
	eachLine(func(line string) {
		switch {
		case strings.HasPrefix(line, "#name="):
			_, err := fmt.Println(line)
			chk(err)
		default:
			s, err := internal.MakeStok(line)
			chk(err)
			_, err = fmt.Printf("%s type=%s\n", s, typ(s))
			chk(err)
		}
	})
}

func eachLine(f func(string)) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		f(scanner.Text())
	}
	chk(scanner.Err())
}

func typ(s internal.Stok) string {
	t := s.Type()
	switch {
	case t.Skipped():
		return t.String()
	case t.Err():
		return t.String() + s.Cause(0).String()
	default:
		return t.String()
	}
}

type stok struct {
	internal.Stok
	Type string
}
