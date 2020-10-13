package stats

import (
	"bufio"
	"encoding/json"
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
	ifgs       []string
	mets       string
	limit      int
	info, json bool
	skipShort  bool
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
	CMD.Flags().IntVarP(&flags.limit, "limit", "L", 0, "set limit for the profiler's candidate set")
	CMD.Flags().BoolVarP(&flags.info, "skip-short", "s", false,
		"exclude short tokens (len<3) from the evaluation")
	CMD.Flags().BoolVarP(&flags.info, "info", "i", false, "print out correction information")
	CMD.Flags().BoolVarP(&flags.json, "json", "j", false, "output as json")
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
			if filename != "" && !flags.info {
				s.write(filename)
			}
			filename = tmp
			s = stats{}
			continue
		}
		chk(s.stat(dtd))
	}
	if !flags.info {
		s.write(filename)
	}
}

func handleIFGs(ifgs []string) {
	for _, ifg := range ifgs {
		var s stats
		chk(eachWord(flags.mets, ifg, s.stat))
		if !flags.info {
			s.write(ifg)
		}
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
	lastGT                                          string
	Skipped, Short, NoCands, Lex                    int
	SkippedMerges, SkippedSplits                    int
	Merges, Splits                                  int
	ShortErr, NoCandsErr, LexErr                    int
	Replaced, OCRCorrect, OCRIncorrect              int
	Suspicious, RedundandCorrection, Disimprovement int
	DisimprovementMC, DisimprovementBL              int
	SuccessfulCorrection, DoNotCare                 int
	DoNotCareMC, DoNotCareBL                        int
	NotReplaced, OCRCorrectNR, OCRIncorrectNR       int
	OCRAccept, DodgedBullets                        int
	DodgedBulletsMC, DodgedBulletsBL                int
	DisimprovementBR, DodgedBulletsBR               int
	MissedOpportunity, SkippedDoNotCare             int
	SkippedDoNotCareMC, SkippedDoNotCareBL          int
	DoNotCareBR, SkippedDoNotCareBR                 int
	TotalErrBefore, TotalErrAfter, Total            int
	Improvement, ErrorRateBefore, ErrorRateAfter    float64
	AccuracyBefore, AccuracyAfter                   float64
}

func (s *stats) stat(dtd string) error {
	var skipped, short, lex, cor bool
	var rank int
	var ocr, sug, gt string
	if err := parseDTD(dtd, &skipped, &short, &lex, &cor, &rank, &ocr, &sug, &gt); err != nil {
		return fmt.Errorf("stat: %v", err)
	}
	if flags.info {
		printErrors(skipped, short, lex, cor, rank, ocr, sug, gt)
		return nil
	}
	// Exclude short tokens from the complete evaluation if
	// the flags.skipShort option is set.
	if flags.skipShort && skipped && short {
		return nil
	}
	// Update counts.
	s.Total++
	if skipped {
		s.Skipped++
	}
	if skipped && short {
		s.Short++
	}
	if skipped && short && ocr != gt {
		s.ShortErr++
	}
	if skipped && !short && !lex {
		s.NoCands++
	}
	if skipped && !short && !lex && ocr != gt {
		s.NoCandsErr++
	}
	if skipped && !short && lex {
		s.Lex++
	}
	if skipped && !short && lex && ocr != gt {
		s.LexErr++
	}
	if skipped && strings.Index(gt, "_") != -1 {
		s.SkippedMerges++
	}
	if !skipped && strings.Index(gt, "_") == 0 {
		s.Merges++
	}
	if skipped && gt != ocr && gt == s.lastGT {
		s.SkippedSplits++
	}
	if !skipped && gt != ocr && gt == s.lastGT {
		s.Splits++
	}
	if !skipped {
		s.Suspicious++
	}
	if !skipped && cor {
		s.Replaced++
	}
	if !skipped && cor && gt == ocr {
		s.OCRCorrect++
	}
	if !skipped && cor && gt == ocr && sug == gt {
		s.RedundandCorrection++
	}
	if !skipped && cor && gt == ocr && sug != gt {
		s.Disimprovement++
		if 0 < flags.limit {
			if rank == 0 {
				s.DisimprovementMC++
			} else if flags.limit < rank {
				s.DisimprovementBL++
			} else {
				s.DisimprovementBR++
			}
		} else {
			if rank == 0 {
				s.DisimprovementMC++
			} else if 1 < rank {
				s.DisimprovementBR++
			}
		}
	}
	if !skipped && cor && gt != ocr {
		s.OCRIncorrect++
	}
	if !skipped && cor && gt != ocr && sug == gt {
		s.SuccessfulCorrection++
	}
	if !skipped && cor && gt != ocr && sug != gt {
		s.DoNotCare++
		if 0 < flags.limit {
			if rank == 0 {
				s.DoNotCareMC++
			} else if flags.limit < rank {
				s.DoNotCareBL++
			} else {
				s.DoNotCareBR++
			}
		} else {
			if rank == 0 {
				s.DoNotCareMC++
			} else if 1 < rank {
				s.DoNotCareBR++
			}
		}
	}
	if !skipped && !cor {
		s.NotReplaced++
	}
	if !skipped && !cor && ocr == gt {
		s.OCRCorrectNR++
	}
	if !skipped && !cor && ocr == gt && sug == gt {
		s.OCRAccept++
	}
	if !skipped && !cor && ocr == gt && sug != gt {
		s.DodgedBullets++
		if 0 < flags.limit {
			if rank == 0 {
				s.DodgedBulletsMC++
			} else if flags.limit < rank {
				s.DodgedBulletsBL++
			} else {
				s.DodgedBulletsBR++
			}
		} else {
			if rank == 0 {
				s.DodgedBulletsMC++
			} else if 1 < rank {
				s.DodgedBulletsBR++
			}
		}
	}
	if !skipped && !cor && ocr != gt {
		s.OCRIncorrectNR++
	}
	if !skipped && !cor && ocr != gt && sug == gt {
		s.MissedOpportunity++
	}
	if !skipped && !cor && ocr != gt && sug != gt {
		s.SkippedDoNotCare++
		if 0 < flags.limit {
			if rank == 0 {
				s.SkippedDoNotCareMC++
			} else if flags.limit < rank {
				s.SkippedDoNotCareBL++
			} else {
				s.SkippedDoNotCareBR++
			}
		} else {
			if rank == 0 {
				s.SkippedDoNotCareMC++
			} else if 1 < rank {
				s.SkippedDoNotCareBR++
			}
		}
	}
	if ocr != gt {
		s.TotalErrBefore++
	}
	if (skipped && ocr != gt) || // errors in skipped tokens
		(!skipped && cor && sug != gt) || // infelicitous correction
		(!skipped && !cor && ocr != gt) { // not corrected and false
		s.TotalErrAfter++
	}
	s.lastGT = gt
	return nil
}

func (s *stats) write(name string) {
	s.ErrorRateBefore = float64(s.TotalErrBefore) / float64(s.Total)
	s.ErrorRateAfter = float64(s.TotalErrAfter) / float64(s.Total)
	s.Improvement = float64(s.TotalErrBefore-s.TotalErrAfter) / float64(s.TotalErrAfter) * 100
	s.AccuracyBefore = 1.0 - s.ErrorRateBefore
	s.AccuracyAfter = 1.0 - s.ErrorRateAfter
	if flags.json {
		chk(json.NewEncoder(os.Stdout).Encode(s))
		return
	}
	fmt.Printf("name                              = %s\n", name)
	fmt.Printf("improvement (percent)             = %f\n", s.Improvement)
	fmt.Printf("error rate (before/after)         = %f/%f\n", s.ErrorRateBefore, s.ErrorRateAfter)
	fmt.Printf("accuracy (before/after)           = %f/%f\n", s.AccuracyBefore, s.AccuracyAfter)
	fmt.Printf("Total errors (before/after)       = %d/%d\n", s.TotalErrBefore, s.TotalErrAfter)
	fmt.Printf("correct (before/after)            = %d/%d\n",
		s.Total-s.TotalErrBefore, s.Total-s.TotalErrAfter)
	fmt.Printf("missing correction                = %d\n",
		s.DodgedBulletsMC+s.DisimprovementMC+s.DoNotCareMC+s.SkippedDoNotCareMC)
	fmt.Printf("bad rank                          = %d\n",
		s.DodgedBulletsBR+s.DisimprovementBR+s.DoNotCareBR+s.SkippedDoNotCareBR)
	fmt.Printf("bad limit                         = %d\n",
		s.DodgedBulletsBL+s.DisimprovementBL+s.DoNotCareBL+s.SkippedDoNotCareBL)
	fmt.Printf("merges                            = %d\n", s.SkippedMerges+s.Merges)
	fmt.Printf("splits                            = %d\n", s.SkippedSplits+s.Splits)
	fmt.Printf("Total tokens                      = %d\n", s.Total)
	fmt.Printf("├─ skipped                        = %d\n", s.Skipped)
	fmt.Printf("│  ├─ short                       = %d\n", s.Short)
	fmt.Printf("│  │  └─ errors                   = %d\n", s.ShortErr)
	fmt.Printf("│  ├─ no candidate                = %d\n", s.NoCands)
	fmt.Printf("│  │  └─ errors                   = %d\n", s.NoCandsErr)
	fmt.Printf("│  └─ lexicon entries             = %d\n", s.Lex)
	fmt.Printf("│     └─ false friends            = %d\n", s.LexErr)
	fmt.Printf("└─ suspicious                     = %d\n", s.Suspicious)
	fmt.Printf("   ├─ replaced                    = %d\n", s.Replaced)
	fmt.Printf("   │  ├─ ocr correct              = %d\n", s.OCRCorrect)
	fmt.Printf("   │  │  ├─ redundant correction  = %d\n", s.RedundandCorrection)
	fmt.Printf("   │  │  └─ disimprovement        = %d\n", s.Disimprovement)
	fmt.Printf("   │  │     ├─ bad rank           = %d\n", s.DisimprovementBR)
	fmt.Printf("   │  │     ├─ bad limit          = %d\n", s.DisimprovementBL)
	fmt.Printf("   │  │     └─ missing correction = %d\n", s.DisimprovementMC)
	fmt.Printf("   │  └─ ocr not correct          = %d\n", s.OCRIncorrect)
	fmt.Printf("   │     ├─ successful correction = %d\n", s.SuccessfulCorrection)
	fmt.Printf("   │     └─ do not care           = %d\n", s.DoNotCare)
	fmt.Printf("   │        ├─ bad rank           = %d\n", s.DoNotCareBR)
	fmt.Printf("   │        ├─ bad limit          = %d\n", s.DoNotCareBL)
	fmt.Printf("   │        └─ missing correction = %d\n", s.DoNotCareMC)
	fmt.Printf("   └─ not replaced                = %d\n", s.NotReplaced)
	fmt.Printf("      ├─ ocr correct              = %d\n", s.OCRCorrectNR)
	fmt.Printf("      │  ├─ ocr accept            = %d\n", s.OCRAccept)
	fmt.Printf("      │  └─ dodged bullets        = %d\n", s.DodgedBullets)
	fmt.Printf("      │     ├─ bad rank           = %d\n", s.DodgedBulletsBR)
	fmt.Printf("      │     ├─ bad limit          = %d\n", s.DodgedBulletsBL)
	fmt.Printf("      │     └─ missing correction = %d\n", s.DodgedBulletsMC)
	fmt.Printf("      └─ ocr not correct          = %d\n", s.OCRIncorrectNR)
	fmt.Printf("         ├─ missed opportunity    = %d\n", s.MissedOpportunity)
	fmt.Printf("         └─ skipped do not care   = %d\n", s.SkippedDoNotCare)
	fmt.Printf("            ├─ bad rank           = %d\n", s.SkippedDoNotCareBR)
	fmt.Printf("            ├─ bad limit          = %d\n", s.SkippedDoNotCareBL)
	fmt.Printf("            └─ missing correction = %d\n", s.SkippedDoNotCareMC)
}

const dtdFormat = "skipped=%t short=%t lex=%t cor=%t rank=%d ocr=%s sug=%s gt=%s"

func printErrors(skipped, short, lex, cor bool, rank int, ocr, sug, gt string) {
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
