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
	corrected, skipped, total         int
	unknown, lex, short               int
	falsef, missedops, disimps, succc int
	badrank, missingcor               int
	totalerrs, skippederrs, corerrs   int
	errsafter                         int
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
	if !skipped {
		s.corrected++
	}
	if skipped && short {
		s.short++
	}
	if skipped && lex {
		s.lex++
	}
	if skipped && !(lex || short) {
		s.unknown++
	}
	if skipped && lex && ocr != gt {
		s.falsef++
	}
	if !skipped && !cor && ocr != gt && sug == gt {
		s.missedops++
	}
	if !skipped && cor && ocr == gt && sug != gt {
		s.disimps++
	}
	if !skipped && cor && ocr != gt && sug == gt {
		s.succc++
	}
	if ocr != gt {
		s.totalerrs++
	}
	if skipped && ocr != gt {
		s.skippederrs++
	}
	if !skipped && ocr != gt {
		s.corerrs++
	}
	if (cor && sug != gt) || (!cor && ocr != gt) || (skipped && ocr != gt) {
		s.errsafter++
	}
	if !skipped && ocr != gt && sug == gt {
		s.errsafter++
	}
	if !skipped && rank == 0 {
		s.missingcor++
	}
	if !skipped && rank > 1 {
		s.badrank++
	}
	return nil
}

func (s *stats) write() {
	fmt.Printf("total tokens           = %d\n", s.total)
	fmt.Printf("skipped tokens         = %d\n", s.skipped)
	fmt.Printf("short tokens           = %d\n", s.short)
	fmt.Printf("uninterpretable tokens = %d\n", s.unknown)
	fmt.Printf("lexical tokens         = %d\n", s.lex)
	fmt.Printf("false friends          = %d\n", s.falsef)
	fmt.Printf("missed oportunities    = %d\n", s.missedops)
	fmt.Printf("missing correction     = %d\n", s.missingcor)
	fmt.Printf("bad rank               = %d\n", s.badrank)
	fmt.Printf("disimprovements        = %d\n", s.disimps)
	fmt.Printf("succesfull corrections = %d\n", s.succc)
	fmt.Printf("uncorrectable errors   = %d\n", s.skippederrs)
	fmt.Printf("correctable errors     = %d\n", s.corerrs)
	fmt.Printf("total errors (before)  = %d\n", s.totalerrs)
	fmt.Printf("total errors (after)   = %d\n", s.errsafter)
}

func parseDTD(n *xmlquery.Node, skipped, short, lex, cor *bool, rank *int, ocr, sug, gt *string) error {
	dtd, _ := node.LookupAttr(n, xml.Name{Local: "dataTypeDetails"})
	const format = "skipped=%t short=%t lex=%t cor=%t rank=%d ocr=%s sug=%s gt=%s"
	_, err := fmt.Sscanf(dtd, format, skipped, short, lex, cor, rank, ocr, sug, gt)
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
