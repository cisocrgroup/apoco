package stats

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"strings"

	"git.sr.ht/~flobar/apoco/pkg/apoco/node"
	"git.sr.ht/~flobar/apoco/pkg/apoco/pagexml"
	"github.com/antchfx/xmlquery"
	"github.com/spf13/cobra"
)

var flags = struct {
	ifgs          []string
	mets          string
	limit         int
	verbose, json bool
}{}

// CMD runs the apoco stats command.
var CMD = &cobra.Command{
	Use:   "stats [DIRS...]",
	Short: "Extract correction stats",
	Run:   run,
}

func init() {
	CMD.Flags().StringVarP(&flags.mets, "mets", "m", "mets.xml", "set path to the mets file")
	CMD.Flags().StringSliceVarP(&flags.ifgs, "input-file-grp", "I", nil, "set input file groups")
	CMD.Flags().IntVarP(&flags.limit, "limit", "l", 0, "set limit for the profiler's candidate set")
	CMD.Flags().BoolVarP(&flags.verbose, "verbose", "v", false, "verbose output of stats")
	CMD.Flags().BoolVarP(&flags.json, "json", "j", false, "output as gnuplot dat format [ignored]")
}

func run(_ *cobra.Command, args []string) {
	if len(flags.ifgs) == 0 {
		handleSimple()
	} else {
		handleIFGs(flags.ifgs)
	}
}

func handleSimple() {
	scanner := bufio.NewScanner(os.Stdin)
	var s stats
	var filename string
	for scanner.Scan() {
		dtd := scanner.Text()
		if dtd != "" && dtd[0] == '#' {
			var tmp string
			if _, err := fmt.Sscanf(dtd, "#filename=%s", &tmp); err != nil {
				continue
			}
			if filename != "" {
				s.write(filename)
			}
			filename = tmp
			s = stats{}
			continue
		}
		chk(s.stat(dtd))
	}
	s.write(filename)
}

func handleIFGs(ifgs []string) {
	for _, ifg := range ifgs {
		var s stats
		chk(eachWord(flags.mets, ifg, s.stat))
		s.write(ifg)
	}
}

func eachWord(mets, inputFileGrp string, f func(string) error) error {
	files, err := pagexml.FilePathsForFileGrp(mets, inputFileGrp)
	if err != nil {
		return fmt.Errorf("eachWord: %v", err)
	}
	for _, file := range files {
		if err := eachTokenInFile(file, f); err != nil {
			return fmt.Errorf("eachWord %s: %v", file, err)
		}
	}
	return nil
}

func eachTokenInFile(path string, f func(string) error) error {
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
		// Simply skip this word if id does not contain any actionable
		// data.
		us := pagexml.FindUnicodesInRegionSorted(word)
		if len(us) == 0 { // skip
			return nil
		}
		te := us[0].Parent
		dtd, found := node.LookupAttr(te, xml.Name{Local: "dataTypeDetails"})
		if dtd == "" || !found { // skip
			return nil
		}
		if err := f(dtd); err != nil {
			return err
		}
	}
	return nil
}

type stats struct {
	lastGT                                    string
	skipped, short, nocands, lex              int
	skippedMerges, skippedSplits              int
	merges, splits                            int
	shorterr, nocandserr, lexerr              int
	replaced, ocrcorrect, ocrincorrect        int
	suspicious, ocraccept, disimprovement     int
	disimprovementMC, disimprovementBL        int
	successfulcorrection, donotcare           int
	donotcareMC, donotcareBL                  int
	notreplaced, ocrcorrectNR, ocrincorrectNR int
	ocracceptNR, disimprovementNR             int
	disimprovementNRMC, disimprovementNRBL    int
	disimprovementBR, disimprovementNRBR      int
	missedopportunity, donotcareNR            int
	donotcareNRMC, donotcareNRBL              int
	donotcareBR, donotcareNRBR                int
	totalerrbefore, totalerrafter, total      int
}

func (s *stats) stat(dtd string) error {
	var skipped, short, lex, cor bool
	var rank int
	var ocr, sug, gt string
	if err := parseDTD(dtd, &skipped, &short, &lex, &cor, &rank, &ocr, &sug, &gt); err != nil {
		return fmt.Errorf("stat: %v", err)
	}
	if flags.verbose {
		verbose(skipped, short, lex, cor, rank, ocr, sug, gt)
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
	if skipped && strings.Index(gt, "_") != -1 {
		s.skippedMerges++
	}
	if !skipped && strings.Index(gt, "_") == 0 {
		s.merges++
	}
	if skipped && gt != ocr && gt == s.lastGT {
		s.skippedSplits++
	}
	if !skipped && gt != ocr && gt == s.lastGT {
		s.splits++
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
		if 0 < flags.limit {
			if rank == 0 {
				s.disimprovementMC++
			} else if flags.limit < rank {
				s.disimprovementBL++
			} else {
				s.disimprovementBR++
			}
		} else {
			if rank == 0 {
				s.disimprovementMC++
			} else if 1 < rank {
				s.disimprovementBR++
			}
		}
	}
	if !skipped && cor && gt != ocr {
		s.ocrincorrect++
	}
	if !skipped && cor && gt != ocr && sug == gt {
		s.successfulcorrection++
	}
	if !skipped && cor && gt != ocr && sug != gt {
		s.donotcare++
		if 0 < flags.limit {
			if rank == 0 {
				s.donotcareMC++
			} else if flags.limit < rank {
				s.donotcareBL++
			} else {
				s.donotcareBR++
			}
		} else {
			if rank == 0 {
				s.donotcareMC++
			} else if 1 < rank {
				s.donotcareBR++
			}
		}
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
		if 0 < flags.limit {
			if rank == 0 {
				s.disimprovementNRMC++
			} else if flags.limit < rank {
				s.disimprovementNRBL++
			} else {
				s.disimprovementNRBR++
			}
		} else {
			if rank == 0 {
				s.disimprovementNRMC++
			} else if 1 < rank {
				s.disimprovementNRBR++
			}
		}
	}
	if !skipped && !cor && ocr != gt {
		s.ocrincorrectNR++
	}
	if !skipped && !cor && ocr != gt && sug == gt {
		s.missedopportunity++
	}
	if !skipped && !cor && ocr != gt && sug != gt {
		s.donotcareNR++
		if 0 < flags.limit {
			if rank == 0 {
				s.donotcareNRMC++
			} else if flags.limit < rank {
				s.donotcareNRBL++
			} else {
				s.donotcareNRBR++
			}
		} else {
			if rank == 0 {
				s.donotcareNRMC++
			} else if 1 < rank {
				s.donotcareNRBR++
			}
		}
	}
	if ocr != gt {
		s.totalerrbefore++
	}
	if (skipped && ocr != gt) || // errors in skipped tokens
		(!skipped && cor && sug != gt) || // infelicitous correction
		(!skipped && !cor && ocr != gt) { // not corrected and false
		s.totalerrafter++
	}
	s.lastGT = gt
	return nil
}

func (s *stats) write(name string) {
	errb := float64(s.totalerrbefore) / float64(s.total)
	erra := float64(s.totalerrafter) / float64(s.total)
	impr := float64(s.totalerrbefore-s.totalerrafter) / float64(s.totalerrafter) * 100
	fmt.Printf("name                                = %s\n", name)
	fmt.Printf("improvement (percent)               = %f\n", impr)
	fmt.Printf("error rate (before/after)           = %f/%f\n", errb, erra)
	fmt.Printf("accuracy (before/after)             = %f/%f\n", 1.0-errb, 1.0-erra)
	fmt.Printf("total errors (before/after)         = %d/%d\n", s.totalerrbefore, s.totalerrafter)
	fmt.Printf("correct (before/after)              = %d/%d\n", s.total-s.totalerrbefore, s.total-s.totalerrafter)
	fmt.Printf("missing correction                  = %d\n",
		s.disimprovementNRMC+s.disimprovementMC+s.donotcareMC+s.donotcareNRMC)
	fmt.Printf("bad rank                            = %d\n",
		s.disimprovementNRBR+s.disimprovementBR+s.donotcareBR+s.donotcareNRBR)
	fmt.Printf("bad limit                           = %d\n",
		s.disimprovementNRBL+s.disimprovementBL+s.donotcareBL+s.donotcareNRBL)
	fmt.Printf("merges                              = %d\n", s.skippedMerges+s.merges)
	fmt.Printf("splits                              = %d\n", s.skippedSplits+s.splits)
	fmt.Printf("total tokens                        = %d\n", s.total)
	fmt.Printf("├─ skipped                          = %d\n", s.skipped)
	fmt.Printf("│  ├─ short                         = %d\n", s.short)
	fmt.Printf("│  │  └─ errors                     = %d\n", s.shorterr)
	fmt.Printf("│  ├─ no candidate                  = %d\n", s.nocands)
	fmt.Printf("│  │  └─ errors                     = %d\n", s.nocandserr)
	fmt.Printf("│  └─ lexicon entries               = %d\n", s.lex)
	fmt.Printf("│     └─ false friends              = %d\n", s.lexerr)
	fmt.Printf("└─ suspicious                       = %d\n", s.suspicious)
	fmt.Printf("   ├─ replaced                      = %d\n", s.replaced)
	fmt.Printf("   │  ├─ ocr correct                = %d\n", s.ocrcorrect)
	fmt.Printf("   │  │  ├─ redundant correction    = %d\n", s.ocraccept)
	fmt.Printf("   │  │  └─ infelicitous correction = %d\n", s.disimprovement)
	fmt.Printf("   │  │     ├─ bad rank             = %d\n", s.disimprovementBR)
	fmt.Printf("   │  │     ├─ bad limit            = %d\n", s.disimprovementBL)
	fmt.Printf("   │  │     └─ missing correction   = %d\n", s.disimprovementMC)
	fmt.Printf("   │  └─ ocr not correct            = %d\n", s.ocrincorrect)
	fmt.Printf("   │     ├─ successful correction   = %d\n", s.successfulcorrection)
	fmt.Printf("   │     └─ do not care             = %d\n", s.donotcare)
	fmt.Printf("   │        ├─ bad rank             = %d\n", s.donotcareBR)
	fmt.Printf("   │        ├─ bad limit            = %d\n", s.donotcareBL)
	fmt.Printf("   │        └─ missing correction   = %d\n", s.donotcareMC)
	fmt.Printf("   └─ not replaced                  = %d\n", s.notreplaced)
	fmt.Printf("      ├─ ocr correct                = %d\n", s.ocrcorrectNR)
	fmt.Printf("      │  ├─ ocr accept              = %d\n", s.ocracceptNR)
	fmt.Printf("      │  └─ dodged bullets          = %d\n", s.disimprovementNR)
	fmt.Printf("      │     ├─ bad rank             = %d\n", s.disimprovementNRBR)
	fmt.Printf("      │     ├─ bad limit            = %d\n", s.disimprovementNRBL)
	fmt.Printf("      │     └─ missing correction   = %d\n", s.disimprovementNRMC)
	fmt.Printf("      └─ ocr not correct            = %d\n", s.ocrincorrectNR)
	fmt.Printf("         ├─ missed opportunity      = %d\n", s.missedopportunity)
	fmt.Printf("         └─ skipped do not care     = %d\n", s.donotcareNR)
	fmt.Printf("            ├─ bad rank             = %d\n", s.donotcareNRBR)
	fmt.Printf("            ├─ bad limit            = %d\n", s.donotcareNRBL)
	fmt.Printf("            └─ missing correction   = %d\n", s.donotcareNRMC)
}

const dtdFormat = "skipped=%t short=%t lex=%t cor=%t rank=%d ocr=%s sug=%s gt=%s"

func verbose(skipped, short, lex, cor bool, rank int, ocr, sug, gt string) {
	write := func(pre string) {
		fmt.Printf(pre+dtdFormat+"\n", skipped, short, lex, cor, rank, ocr, sug, gt)
	}
	if !skipped && rank > 1 {
		write("bad rank:                ")
	}
	if !skipped && rank == 0 {
		write("missing correction:      ")
	}
	if !skipped && cor && gt == ocr && sug != gt {
		write("infelicitous correction: ")
	}
	if !skipped && !cor && ocr != gt && sug == gt {
		write("missed opportunity:      ")
	}
	if !skipped && cor && gt != ocr && sug == gt {
		write("successful correction:   ")
	}
}

func parseDTD(dtd string, skip, short, lex, cor *bool, rank *int, ocr, sug, gt *string) error {
	_, err := fmt.Sscanf(dtd, dtdFormat, skip, short, lex, cor, rank, ocr, sug, gt)
	if err != nil {
		return fmt.Errorf("parseDTD: cannot parse %q: %v", dtd, err)
	}
	return nil
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
