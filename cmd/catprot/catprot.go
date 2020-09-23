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

var flags = internal.Flags{}

func init() {
	flags.Init(CMD)
}

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
	Tokens map[string]correction `json:"tokens"`
}

type correction struct {
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

func (cor *correction) rank() int {
	for i, c := range cor.Candidates {
		if c.Suggestion == e(cor.GT.GT) {
			return i + 1
		}
	}
	return 0
}

func (cor *correction) lex() bool {
	return len(cor.Candidates) == 1 &&
		cor.Candidates[0].OCRDistance == 0 &&
		cor.Candidates[0].HistDistance == 0
}

func cat(name string) {
	is, err := os.Open(name)
	chk(err)
	defer is.Close()
	var d data
	chk(json.NewDecoder(is).Decode(&d))
	for _, cor := range d.Tokens {
		nosuggs := len(cor.Candidates) == 0
		short := utf8.RuneCountInString(cor.OCR) <= 3
		lex := cor.lex()
		_, err := fmt.Printf("skipped=%t short=%t lex=%t cor=%t rank=%d ocr=%s sug=%s gt=%s\n",
			lex || short || nosuggs, short, lex, cor.Taken,
			cor.rank(), e(cor.OCR), e(cor.Cor), e(cor.GT.GT))
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
