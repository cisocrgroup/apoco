package stats

import (
	"encoding/xml"
	"fmt"
	"log"
	"os"

	"example.com/apoco/pkg/apoco/node"
	"example.com/apoco/pkg/apoco/pagexml"
	"github.com/antchfx/xmlquery"
	"github.com/spf13/cobra"
)

func init() {
	CMD.Flags().StringVarP(&flags.mets, "mets", "m", "mets.xml", "set mets file")
	CMD.Flags().StringVarP(&flags.inputFileGrp, "input-file-grp", "I", "", "set input file group")
	CMD.Flags().BoolVarP(&flags.csv, "csv", "c", false, "output csv data")
}

var flags = struct {
	mets, inputFileGrp string
	parameters         string
	csv                bool
}{}

// CMD runs the apoco stats command.
var CMD = &cobra.Command{
	Use:   "stats",
	Short: "Extract correction stats",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	if flags.csv {
		printCSVHeader()
		noerr(eachToken(flags.mets, flags.inputFileGrp, printCSV))
	} else {
		var s stats
		noerr(eachToken(flags.mets, flags.inputFileGrp, s.stat))
		s.write()
	}
}

func eachToken(mets, inputFileGrp string, f func(doc *xmlquery.Node) error) error {
	files, err := pagexml.FilePathsForFileGrp(mets, inputFileGrp)
	if err != nil {
		return fmt.Errorf("eachToken: %v", err)
	}
	for _, file := range files {
		if err := eachTokenInFile(file, f); err != nil {
			return fmt.Errorf("eachToken %s: %v", file, err)
		}
	}
	return nil
}

func eachTokenInFile(path string, f func(doc *xmlquery.Node) error) error {
	is, err := os.Open(path)
	if err != nil {
		return err
	}
	defer is.Close()
	doc, err := xmlquery.Parse(is)
	if err != nil {
		return err
	}
	for _, word := range xmlquery.Find(doc, "//*[local-name()='Word']") {
		if err := f(word); err != nil {
			return err
		}
	}
	return nil
}

func printCSVHeader() {
	fmt.Printf("# skipped,short,lex,cor,rank,ocr,sug,gt\n")
}

func printCSV(word *xmlquery.Node) error {
	te := pagexml.FindUnicodesFromRegionSorted(word)[0].Parent
	var skipped, short, lex, cor bool
	var rank int
	var ocr, sug, gt string
	if err := parseDTD(te, &skipped, &short, &lex, &cor, &rank, &ocr, &sug, &gt); err != nil {
		return fmt.Errorf("printCSV: %v", err)
	}
	fmt.Printf("%t,%t,%t,%t,%d,%s,%s,%s\n", skipped, short, lex, cor, rank, ocr, sug, gt)
	return nil
}

type stats struct {
	skipped, short, nocands, lex              int
	shorterr, nocandserr, lexerr              int
	replaced, ocrcorrect, ocrincorrect        int
	suspicious, ocraccept, disimprovement     int
	successfulcorrection, donotcare           int
	notreplaced, ocrcorrectNR, ocrincorrectNR int
	ocracceptNR, disimprovementNR             int
	missedopportunity, donotcareNR            int
	total, badrank, missingcorrection         int
	totalerrbefore, totalerrafter             int
}

func (s *stats) stat(word *xmlquery.Node) error {
	te := pagexml.FindUnicodesFromRegionSorted(word)[0].Parent
	var skipped, short, lex, cor bool
	var rank int
	var ocr, sug, gt string
	if err := parseDTD(te, &skipped, &short, &lex, &cor, &rank, &ocr, &sug, &gt); err != nil {
		return fmt.Errorf("stat: %v", err)
	}
	s.total++
	if skipped {
		s.skipped++
	}
	if skipped && short {
		s.short++
	}
	if skipped && short && ocr != gt {
		s.shorterr++
	}
	if skipped && !short && !lex {
		s.nocands++
	}
	if skipped && !short && !lex && ocr != gt {
		s.nocandserr++
	}
	if skipped && !short && lex {
		s.lex++
	}
	if skipped && !short && lex && ocr != gt {
		s.lexerr++
	}
	if !skipped {
		s.suspicious++
	}
	if !skipped && cor {
		s.replaced++
	}
	if !skipped && cor && gt == ocr {
		s.ocrcorrect++
	}
	if !skipped && cor && gt == ocr && sug == gt {
		s.ocraccept++
	}
	if !skipped && cor && gt == ocr && sug != gt {
		s.disimprovement++
	}
	if !skipped && cor && gt != ocr {
		s.ocrincorrect++
	}
	if !skipped && cor && gt != ocr && sug == gt {
		s.successfulcorrection++
	}
	if !skipped && cor && gt != ocr && sug != gt {
		s.donotcare++
	}
	if !skipped && !cor {
		s.notreplaced++
	}
	if !skipped && !cor && ocr == gt {
		s.ocrcorrectNR++
	}
	if !skipped && !cor && ocr == gt && sug == gt {
		s.ocracceptNR++
	}
	if !skipped && !cor && ocr == gt && sug != gt {
		s.disimprovementNR++
	}
	if !skipped && !cor && ocr != gt {
		s.ocrincorrectNR++
	}
	if !skipped && !cor && ocr != gt && sug == gt {
		s.missedopportunity++
	}
	if !skipped && !cor && ocr != gt && sug != gt {
		s.donotcareNR++
	}
	if !skipped && rank == 0 {
		s.missingcorrection++
	}
	if !skipped && rank > 1 {
		s.badrank++
	}
	if ocr != gt {
		s.totalerrbefore++
	}
	if (skipped && ocr != gt) || // errors in skipped tokens
		(!skipped && cor && sug != gt) || // infelicious correction
		(!skipped && !cor && ocr != gt) { // not corrected and false
		s.totalerrafter++
	}
	return nil
}

// skipped, short, nocands, lex, falsef int
func (s *stats) write() {
	fmt.Printf("missing correction candidate    = %d\n", s.missingcorrection)
	fmt.Printf("bad rank                        = %d\n", s.badrank)
	fmt.Printf("total errors (before)           = %d\n", s.totalerrbefore)
	fmt.Printf("total errors (after)            = %d\n", s.totalerrafter)
	fmt.Printf("total tokens                    = %d\n", s.total)
	fmt.Printf("├ skipped                       = %d\n", s.skipped)
	fmt.Printf("│ ├ short                       = %d\n", s.short)
	fmt.Printf("│ │ └ errors                    = %d\n", s.shorterr)
	fmt.Printf("│ ├ no candidate                = %d\n", s.nocands)
	fmt.Printf("│ │ └ errors                    = %d\n", s.nocandserr)
	fmt.Printf("│ └ lexicon entries             = %d\n", s.lex)
	fmt.Printf("│   └ false friends             = %d\n", s.lexerr)
	fmt.Printf("└ suspicious                    = %d\n", s.suspicious)
	fmt.Printf("  ├ replaced                    = %d\n", s.replaced)
	fmt.Printf("  │ ├ ocr correct               = %d\n", s.ocrcorrect)
	fmt.Printf("  │ │ ├ ocr accept              = %d\n", s.ocraccept)
	fmt.Printf("  │ │ └ infelicitous correction = %d\n", s.disimprovement)
	fmt.Printf("  │ └ ocr not correct           = %d\n", s.ocrincorrect)
	fmt.Printf("  │   ├ successful correction   = %d\n", s.successfulcorrection)
	fmt.Printf("  │   └ do not care             = %d\n", s.donotcare)
	fmt.Printf("  └ not replaced                = %d\n", s.notreplaced)
	fmt.Printf("    ├ ocr correct               = %d\n", s.ocrcorrectNR)
	fmt.Printf("    │ ├ candiate correct        = %d\n", s.ocracceptNR)
	fmt.Printf("    │ └ candiate not correct    = %d\n", s.disimprovementNR)
	fmt.Printf("    └ ocr no correct            = %d\n", s.ocrincorrectNR)
	fmt.Printf("      ├ missed opportunity      = %d\n", s.missedopportunity)
	fmt.Printf("      └ ocr incorrect           = %d\n", s.donotcareNR)
}

func parseDTD(n *xmlquery.Node, skip, short, lex, cor *bool, rank *int, ocr, sug, gt *string) error {
	dtd, _ := node.LookupAttr(n, xml.Name{Local: "dataTypeDetails"})
	const format = "skipped=%t short=%t lex=%t cor=%t rank=%d ocr=%s sug=%s gt=%s"
	_, err := fmt.Sscanf(dtd, format, skip, short, lex, cor, rank, ocr, sug, gt)
	if err != nil {
		return fmt.Errorf("parseDTD: cannot parse %q: %v", dtd, err)
	}
	return nil
}

func noerr(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
