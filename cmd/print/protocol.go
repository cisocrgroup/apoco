package print

import (
	"encoding/json"
	"fmt"
	"os"
	"unicode/utf8"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"github.com/spf13/cobra"
)

var protocolCMD = &cobra.Command{
	Use:   "protocol [INPUT...]",
	Short: "Output stats from a-i-pocoto protocol files",
	Run:   runProtocol,
}

func runProtocol(_ *cobra.Command, args []string) {
	for _, arg := range args {
		catp(arg)
	}
}

type data struct {
	Tokens      map[string]token      `json:"tokens"`
	Corrections map[string]correction `json:"corrections"`
}

type token struct {
	Candidates []candidate `json:"candidates"`
	GT         gt          `json:"gt"`
	OCR        string      `json:"ocrNormalized"`
	Cor        string      `json:"corNormalized"`
	Conf       float64     `json:"confidence"`
	Taken      bool        `json:"taken"`
}

type gt struct {
	GT      string `json:"gt"`
	Present bool   `json:"present"`
}

type candidate struct {
	HistDistance int    `json:"histDistance"`
	OCRDistance  int    `json:"ocrDistance"`
	Suggestion   string `json:"suggestion"`
}

type correction struct {
	Rankings []ranking `json:"rankings"`
}

type ranking struct {
	Candidate struct {
		Suggestion string `json:"Suggestion"`
	} `json:"candidate"`
}

func (t *token) rank() int {
	gt := e(t.GT.GT)
	for i, c := range t.Candidates {
		if c.Suggestion == gt {
			return i + 1
		}
	}
	return 0
}

func (t *token) lex() bool {
	return len(t.Candidates) == 1 &&
		t.Candidates[0].OCRDistance == 0 &&
		t.Candidates[0].HistDistance == 0
}

func (c *correction) rank(gt string) int {
	for i, r := range c.Rankings {
		if r.Candidate.Suggestion == gt {
			return i + 1
		}
	}
	return 0
}

func catp(name string) {
	_, err := fmt.Printf("#filename=%s\n", name)
	chk(err)
	is, err := os.Open(name)
	chk(err)
	defer is.Close()
	var d data
	chk(json.NewDecoder(is).Decode(&d))
	for id, t := range d.Tokens {
		gt := e(t.GT.GT)
		rank := t.rank()
		if ranking, ok := d.Corrections[id]; ok {
			trank := ranking.rank(t.GT.GT)
			if trank != 0 {
				rank = trank
			}
		}
		nosuggs := len(t.Candidates) == 0
		short := utf8.RuneCountInString(t.OCR) <= 3
		lex := t.lex()
		stok := internal.Stok{
			ID:      id,
			Skipped: lex || short || nosuggs,
			Short:   short,
			Lex:     lex,
			Cor:     t.Taken,
			Conf:    t.Conf,
			Rank:    rank,
			OCR:     t.OCR,
			Sug:     t.Cor,
			GT:      gt,
		}
		_, err := fmt.Printf("%s\n", stok)
		chk(err)
	}
}

func e(str string) string {
	return internal.E(str)
}
