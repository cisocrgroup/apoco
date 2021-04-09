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

var statsFlags = struct {
	limit     int
	skipShort bool
	verbose   bool
}{}

// statsCMD runs the apoco stats command.
var statsCMD = &cobra.Command{
	Use:   "stats [DIRS...]",
	Short: "Extract correction stats",
	Run:   runStats,
}

func init() {
	statsCMD.Flags().IntVarP(&statsFlags.limit, "limit", "L", 0, "set limit for the profiler's candidate set")
	statsCMD.Flags().BoolVarP(&statsFlags.skipShort, "skip-short", "s", false,
		"exclude short tokens (len<3) from the evaluation")
	statsCMD.Flags().BoolVarP(&statsFlags.verbose, "verbose", "v", false,
		"enable more verbose error and correction output")
}

func runStats(_ *cobra.Command, args []string) {
	scanner := bufio.NewScanner(os.Stdin)
	var s stats
	var filename string
	for scanner.Scan() {
		dtd := scanner.Text()
		if dtd != "" && dtd[0] == '#' {
			var tmp string
			if _, err := fmt.Sscanf(dtd, "#name=%s", &tmp); err != nil {
				continue // Treat lines starting with # as comments.
			}
			filename = tmp
			s = stats{}
			continue
		}
		chk(s.stat(dtd))
	}
	s.write(filename)
}

type stats struct {
	lastGT                                       string
	Skipped, Short, NoCands, Lex                 int
	SkippedMerges, SkippedSplits                 int
	Merges, Splits                               int
	ShortErr, NoCandsErr, LexErr                 int
	Replaced, OCRCorrect, OCRIncorrect           int
	Suspicious, RedundandCorr, Disimprovement    int
	DisimprovementMC, DisimprovementBL           int
	SuccessfulCorr, DoNotCare                    int
	DoNotCareMC, DoNotCareBL                     int
	NotReplaced, OCRCorrectNR, OCRIncorrectNR    int
	OCRAccept, DodgedBullets                     int
	DodgedBulletsMC, DodgedBulletsBL             int
	DisimprovementBR, DodgedBulletsBR            int
	MissedOpportunity, SkippedDoNotCare          int
	SkippedDoNotCareMC, SkippedDoNotCareBL       int
	DoNotCareBR, SkippedDoNotCareBR              int
	TotalErrBefore, TotalErrAfter, Total         int
	Improvement, ErrorRateBefore, ErrorRateAfter float64
	AccuracyBefore, AccuracyAfter                float64
}

func (s *stats) stat(dtd string) error {
	t, err := internal.MakeStok(dtd)
	if err != nil {
		return err
	}
	// Exclude short tokens from the complete evaluation if
	// the statsFlags.skipShort option is set.
	if statsFlags.skipShort && t.Skipped && t.Short {
		return nil
	}
	// Update counts.
	s.Total++
	if t.Skipped {
		s.Skipped++
	}
	if t.Skipped && t.Short {
		s.Short++
	}
	if t.Skipped && t.Short && t.OCR != t.GT {
		s.ShortErr++
	}
	if t.Skipped && !t.Short && !t.Lex {
		s.NoCands++
	}
	if t.Skipped && !t.Short && !t.Lex && t.OCR != t.GT {
		s.NoCandsErr++
	}
	if t.Skipped && !t.Short && t.Lex {
		s.Lex++
	}
	if t.Skipped && !t.Short && t.Lex && t.OCR != t.GT {
		s.LexErr++
	}
	if t.Skipped && strings.Contains(t.GT, "_") {
		s.SkippedMerges++
	}
	if !t.Skipped && strings.Index(t.GT, "_") == 0 {
		s.Merges++
	}
	if t.Skipped && t.GT != t.OCR && t.GT == s.lastGT {
		s.SkippedSplits++
	}
	if !t.Skipped && t.GT != t.OCR && t.GT == s.lastGT {
		s.Splits++
	}
	if !t.Skipped {
		s.Suspicious++
	}
	if !t.Skipped && t.Cor {
		s.Replaced++
	}
	if !t.Skipped && t.Cor && t.GT == t.OCR {
		s.OCRCorrect++
	}
	if !t.Skipped && t.Cor && t.GT == t.OCR && t.Sug == t.GT {
		s.RedundandCorr++
	}
	if !t.Skipped && t.Cor && t.GT == t.OCR && t.Sug != t.GT {
		s.Disimprovement++
		updateSubErrors(
			statsFlags.limit,
			t.Rank,
			&s.DisimprovementMC,
			&s.DisimprovementBR,
			&s.DisimprovementBL,
		)
	}
	if !t.Skipped && t.Cor && t.GT != t.OCR {
		s.OCRIncorrect++
	}
	if !t.Skipped && t.Cor && t.GT != t.OCR && t.Sug == t.GT {
		s.SuccessfulCorr++
	}
	if !t.Skipped && t.Cor && t.GT != t.OCR && t.Sug != t.GT {
		s.DoNotCare++
		updateSubErrors(
			statsFlags.limit,
			t.Rank,
			&s.DoNotCareMC,
			&s.DoNotCareBR,
			&s.DoNotCareBL,
		)
	}
	if !t.Skipped && !t.Cor {
		s.NotReplaced++
	}
	if !t.Skipped && !t.Cor && t.OCR == t.GT {
		s.OCRCorrectNR++
	}
	if !t.Skipped && !t.Cor && t.OCR == t.GT && t.Sug == t.GT {
		s.OCRAccept++
	}
	if !t.Skipped && !t.Cor && t.OCR == t.GT && t.Sug != t.GT {
		s.DodgedBullets++
		updateSubErrors(
			statsFlags.limit,
			t.Rank,
			&s.DodgedBulletsMC,
			&s.DodgedBulletsBR,
			&s.DodgedBulletsBL,
		)
	}
	if !t.Skipped && !t.Cor && t.OCR != t.GT {
		s.OCRIncorrectNR++
	}
	if !t.Skipped && !t.Cor && t.OCR != t.GT && t.Sug == t.GT {
		s.MissedOpportunity++
	}
	if !t.Skipped && !t.Cor && t.OCR != t.GT && t.Sug != t.GT {
		s.SkippedDoNotCare++
		updateSubErrors(
			statsFlags.limit,
			t.Rank,
			&s.SkippedDoNotCareMC,
			&s.SkippedDoNotCareBR,
			&s.SkippedDoNotCareBL,
		)
	}
	if t.OCR != t.GT {
		s.TotalErrBefore++
	}
	if (t.Skipped && t.OCR != t.GT) || // errors in skipped tokens
		(!t.Skipped && t.Cor && t.Sug != t.GT) || // infelicitous correction
		(!t.Skipped && !t.Cor && t.OCR != t.GT) { // not corrected and false
		s.TotalErrAfter++
	}
	s.lastGT = t.GT
	return nil
}

func (s *stats) write(name string) {
	s.ErrorRateBefore = float64(s.TotalErrBefore) / float64(s.Total)
	s.ErrorRateAfter = float64(s.TotalErrAfter) / float64(s.Total)
	corbefore, corafter := s.Total-s.TotalErrBefore, s.Total-s.TotalErrAfter
	s.Improvement = (float64(corafter-corbefore) / float64(corbefore)) * 100.0
	s.AccuracyBefore = 1.0 - s.ErrorRateBefore
	s.AccuracyAfter = 1.0 - s.ErrorRateAfter
	if flags.json {
		chk(json.NewEncoder(os.Stdout).Encode(s))
		return
	}
	fmt.Printf("Name                            = %s\n", name)
	fmt.Printf("Improvement (percent)           = %f\n", s.Improvement)
	fmt.Printf("Error rate (before/after)       = %f/%f\n", s.ErrorRateBefore, s.ErrorRateAfter)
	fmt.Printf("Accuracy (before/after)         = %f/%f\n", s.AccuracyBefore, s.AccuracyAfter)
	fmt.Printf("Total errors (before/after)     = %d/%d\n", s.TotalErrBefore, s.TotalErrAfter)
	fmt.Printf("Correct (before/after)          = %d/%d\n", corbefore, corafter)
	fmt.Printf("Missing corrections             = %d\n",
		s.DodgedBulletsMC+s.DisimprovementMC+s.DoNotCareMC+s.SkippedDoNotCareMC)
	fmt.Printf("Bad rank                        = %d\n",
		s.DodgedBulletsBR+s.DisimprovementBR+s.DoNotCareBR+s.SkippedDoNotCareBR)
	fmt.Printf("Bad limit                       = %d\n",
		s.DodgedBulletsBL+s.DisimprovementBL+s.DoNotCareBL+s.SkippedDoNotCareBL)
	fmt.Printf("Short errors                    = %d\n", s.ShortErr)
	fmt.Printf("Missing candidate errors        = %d\n", s.NoCandsErr)
	fmt.Printf("False friends                   = %d\n", s.LexErr)
	fmt.Printf("Merges                          = %d\n", s.SkippedMerges+s.Merges)
	fmt.Printf("Splits                          = %d\n", s.SkippedSplits+s.Splits)
	fmt.Printf("Total tokens                    = %d\n", s.Total)
	if !statsFlags.verbose {
		return
	}
	fmt.Printf("├─ skipped                      = %d\n", s.Skipped)
	fmt.Printf("│  ├─ short                     = %d\n", s.Short)
	fmt.Printf("│  │  └─ errors                 = %d\n", s.ShortErr)
	fmt.Printf("│  ├─ no candidate              = %d\n", s.NoCands)
	fmt.Printf("│  │  └─ errors                 = %d\n", s.NoCandsErr)
	fmt.Printf("│  └─ lexicon entries           = %d\n", s.Lex)
	fmt.Printf("│     └─ false friends          = %d\n", s.LexErr)
	fmt.Printf("└─ suspicious                   = %d\n", s.Suspicious)
	fmt.Printf("   ├─ replaced                  = %d\n", s.Replaced)
	fmt.Printf("   │  ├─ ocr correct            = %d\n", s.OCRCorrect)
	fmt.Printf("   │  │  ├─ redundant corr      = %d\n", s.RedundandCorr)
	fmt.Printf("   │  │  └─ disimprovement      = %d\n", s.Disimprovement)
	fmt.Printf("   │  │     ├─ bad rank         = %d\n", s.DisimprovementBR)
	fmt.Printf("   │  │     ├─ bad limit        = %d\n", s.DisimprovementBL)
	fmt.Printf("   │  │     └─ missing corr     = %d\n", s.DisimprovementMC)
	fmt.Printf("   │  └─ ocr not correct        = %d\n", s.OCRIncorrect)
	fmt.Printf("   │     ├─ successful corr     = %d\n", s.SuccessfulCorr)
	fmt.Printf("   │     └─ do not care         = %d\n", s.DoNotCare)
	fmt.Printf("   │        ├─ bad rank         = %d\n", s.DoNotCareBR)
	fmt.Printf("   │        ├─ bad limit        = %d\n", s.DoNotCareBL)
	fmt.Printf("   │        └─ missing corr     = %d\n", s.DoNotCareMC)
	fmt.Printf("   └─ not replaced              = %d\n", s.NotReplaced)
	fmt.Printf("      ├─ ocr correct            = %d\n", s.OCRCorrectNR)
	fmt.Printf("      │  ├─ ocr accept          = %d\n", s.OCRAccept)
	fmt.Printf("      │  └─ dodged bullets      = %d\n", s.DodgedBullets)
	fmt.Printf("      │     ├─ bad rank         = %d\n", s.DodgedBulletsBR)
	fmt.Printf("      │     ├─ bad limit        = %d\n", s.DodgedBulletsBL)
	fmt.Printf("      │     └─ missing corr     = %d\n", s.DodgedBulletsMC)
	fmt.Printf("      └─ ocr not correct        = %d\n", s.OCRIncorrectNR)
	fmt.Printf("         ├─ missed opportunity  = %d\n", s.MissedOpportunity)
	fmt.Printf("         └─ skipped do not care = %d\n", s.SkippedDoNotCare)
	fmt.Printf("            ├─ bad rank         = %d\n", s.SkippedDoNotCareBR)
	fmt.Printf("            ├─ bad limit        = %d\n", s.SkippedDoNotCareBL)
	fmt.Printf("            └─ missing corr     = %d\n", s.SkippedDoNotCareMC)
}

func updateSubErrors(limit, rank int, mc, br, bl *int) {
	if limit > 0 {
		if rank == 0 {
			*mc++
		} else if statsFlags.limit < rank {
			*bl++
		} else {
			*br++
		}
	} else {
		if rank == 0 {
			*mc++
		} else if 1 < rank {
			*br++
		}
	}
}
