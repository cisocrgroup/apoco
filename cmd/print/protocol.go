package print

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"unicode/utf8"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco/mets"
	"git.sr.ht/~flobar/apoco/pkg/apoco/node"
	"git.sr.ht/~flobar/apoco/pkg/apoco/pagexml"
	"github.com/antchfx/xmlquery"
	"github.com/spf13/cobra"
)

var protocolFlags = struct {
	mets string
	ifgs []string
}{}

var protocolCMD = &cobra.Command{
	Use:   "protocol [INPUT...]",
	Short: "Output stats from a-i-pocoto protocol or from page XML files",
	Run:   runProtocol,
}

func init() {
	protocolCMD.Flags().StringVarP(&protocolFlags.mets, "mets", "m", "mets.xml", "set path to the mets file")
	protocolCMD.Flags().StringSliceVarP(&protocolFlags.ifgs, "input-file-grp", "I", nil, "set input file groups")
}

func runProtocol(_ *cobra.Command, args []string) {
	switch {
	case len(protocolFlags.ifgs) == 0:
		aipocoto(args)
	default:
		ifgs(protocolFlags.mets, protocolFlags.ifgs)
	}
}

func ifgs(METS string, ifgs []string) {
	m, err := mets.Open(METS)
	chk(err)
	for _, ifg := range ifgs {
		names, err := m.FilePathsForFileGrp(ifg)
		chk(err)
		for _, name := range names {
			printStoksInPageXML(name)
		}
	}
}

func printStoksInPageXML(name string) {
	is, err := os.Open(name)
	chk(err)
	defer is.Close()
	doc, err := xmlquery.Parse(is)
	chk(err)
	var stoks []internal.Stok
	for _, word := range xmlquery.Find(doc, "//*[local-name()='Word']") {
		// Simply skip this word if id does not contain any actionable
		// data.
		us := pagexml.FindUnicodesInRegionSorted(word)
		if len(us) == 0 { // skip
			continue
		}
		te := us[0].Parent
		dtd, found := node.LookupAttr(te, xml.Name{Local: "dataTypeDetails"})
		if dtd == "" || !found { // skip
			continue
		}
		switch {
		case flags.json:
			stok, err := internal.MakeStok(dtd)
			chk(err)
			stoks = append(stoks, stok)
		default:
			_, err := fmt.Println(dtd)
			chk(err)
		}
	}
	if flags.json {
		chk(json.NewEncoder(os.Stdout).Encode(stoks))
	}
}

func aipocoto(args []string) {
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
	var stoks []internal.Stok
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
		switch {
		case flags.json:
			stoks = append(stoks, stok)
		default:
			_, err := fmt.Println(stok.String())
			chk(err)
		}
	}
	if flags.json {
		chk(json.NewEncoder(os.Stdout).Encode(stoks))
	}
}

func e(str string) string {
	return internal.E(str)
}
