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
	if s.Skipped {
		var suf string
		if s.OCR != s.GT {
			suf = "-error"
		}
		if s.Lex {
			return "skipped-lexical" + suf
		}
		if s.Short {
			return "skipped-short" + suf
		}
		return "skipped-unkown" + suf
	}
	if s.Cor {
		if s.OCR != s.GT {
			if s.Sug == s.GT {
				return "successful-correction"
			}
			return "do-not-care-correction"
		}
		if s.Sug == s.GT {
			return "redundant-correction"
		}
		return "bad-correction"
	}
	if s.OCR != s.GT {
		if s.Sug == s.GT {
			return "missed-opportunity"
		}
		return "do-not-care"
	}
	if s.Sug == s.GT {
		return "accept"
	}
	return "dodged-bullet"
}

type stok struct {
	internal.Stok
	Type string
}
