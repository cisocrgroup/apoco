package catprot

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"unicode/utf8"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"github.com/spf13/cobra"
)

func init() {
	flags.Init(CMD)
}

var flags internal.Flags

// CMD defines the apoco catprot command.
var CMD = &cobra.Command{
	Use:   "catprot",
	Short: "Output stats from a-i-pocoto protocols",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	for _, arg := range args {
		cat(arg)
	}
}

type data struct {
	Corrections map[string]correction `json:"corrections"`
}

type correction struct {
	Rankings []ranking `json:"rankings"`
	GT       gt        `json:"gt"`
	OCR      string    `json:"ocrNormalized"`
	Cor      string    `json:"corNormalized"`
	Conf     float64   `json:"confidence"`
	Taken    bool      `json:"taken"`
}

type gt struct {
	GT      string `json:"gt"`
	Present bool   `json:"present"`
}

type ranking struct {
	Candidate candidate `json:"candidate"`
	Ranking   float64   `json:"ranking"`
}

type candidate struct {
	Suggestion string
	Distance   int
}

func (c *correction) rank() int {
	for i, r := range c.Rankings {
		if r.Candidate.Suggestion == e(c.GT.GT) {
			return i + 1
		}
	}
	return 0
}

func (c *correction) lex() bool {
	return len(c.Rankings) == 1 && c.Rankings[0].Candidate.Distance == 0
}

func cat(name string) {
	is, err := os.Open(name)
	chk(err)
	defer is.Close()
	var d data
	chk(json.NewDecoder(is).Decode(&d))
	for _, cor := range d.Corrections {
		short := utf8.RuneCountInString(cor.OCR) <= 3
		lex := cor.lex()
		_, err := fmt.Printf("skipped=%t short=%t lex=%t cor=%t rank=%d ocr=%s sug=%s gt=%s\n",
			lex || short, short, lex, cor.Taken, cor.rank(), e(cor.OCR), e(cor.Cor), e(cor.GT.GT))
		chk(err)
	}
}

func e(str string) string {
	if str == "" {
		return "ε"
	}
	return strings.ToLower(str)
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
