package print

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"unicode/utf8"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/lev"
	"github.com/spf13/cobra"
)

var statsFlags = struct {
	name      string
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
	statsCMD.Flags().StringVarP(&statsFlags.name, "name", "n", "", "set name")
	statsCMD.Flags().IntVarP(&statsFlags.limit, "limit", "L", 0, "set limit for the profiler's candidate set")
	statsCMD.Flags().BoolVarP(&statsFlags.skipShort, "skip-short", "s", false,
		"exclude short tokens (len<3) from the evaluation")
	statsCMD.Flags().BoolVarP(&statsFlags.verbose, "verbose", "v", false,
		"enable more verbose error and correction output")
}

func runStats(_ *cobra.Command, args []string) {
	scanner := bufio.NewScanner(os.Stdin)
	var s stats
	filename := statsFlags.name
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
	switch {
	case flags.json:
		s.json(filename)
	default:
		s.write(filename, statsFlags.verbose)
	}
}

type stats struct {
	types                                     typeMap
	causes                                    causeMap
	before                                    internal.Stok
	mat                                       lev.Mat
	skippedMerges, skippedSplits              int
	merges, splits                            int
	tokenErrBefore, tokenErrAfter, tokenTotal int
	charErrBefore, charErrAfter, charTotal    int
	suspErrBefore, suspErrAfter, suspTotal    int
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
	typ := t.Type()
	s.types.put(typ)
	if typ.Err() && !typ.Skipped() {
		s.causes.put(typ, t.Cause(statsFlags.limit))
	}
	if t.Skipped && t.Merge() {
		s.skippedMerges++
	}
	if !t.Skipped && t.Merge() {
		s.merges++
	}
	if t.Skipped && t.Split(s.before) {
		s.skippedSplits++
	}
	if !t.Skipped && t.Split(s.before) {
		s.splits++
	}
	// Gather token errors.
	s.tokenTotal++
	if t.ErrBefore() {
		s.tokenErrBefore++
	}
	if t.ErrAfter() {
		s.tokenErrAfter++
	}
	// Gather character errors.
	s.charTotal += utf8.RuneCountInString(t.GT)
	s.charErrBefore += s.mat.Distance(t.OCR, t.GT)
	switch {
	case t.Cor:
		s.charErrAfter += s.mat.Distance(t.Sug, t.GT)
	default:
		s.charErrAfter += s.mat.Distance(t.OCR, t.GT)
	}
	// Gather errors on suspicious tokens
	if !t.Skipped {
		s.suspTotal++
		if t.ErrBefore() {
			s.suspErrBefore++
		}
		if t.ErrAfter() {
			s.suspErrAfter++
		}
	}

	s.before = t
	return nil
}

func (s *stats) write(name string, verbose bool) {
	tokenErrRateBefore, tokenErrRateAfter := errorRates(s.tokenErrBefore, s.tokenErrAfter, s.tokenTotal)
	charErrRateBefore, charErrRateAfter := errorRates(s.charErrBefore, s.charErrAfter, s.charTotal)
	suspErrRateBefore, suspErrRateAfter := errorRates(s.suspErrBefore, s.suspErrAfter, s.suspTotal)
	accBefore, accAfter := 1.0-tokenErrRateBefore, 1.0-tokenErrRateAfter
	suspAccBefore, suspAccAfter := 1.0-suspErrRateBefore, 1.0-suspErrRateAfter
	corbefore, corafter := s.tokenTotal-s.tokenErrBefore, s.tokenTotal-s.tokenErrAfter
	improvement := s.improvement()
	charImprovement := ((charErrRateAfter - charErrRateBefore) / charErrRateBefore) * 100
	fmt.Printf("Name                            = %s\n", name)
	fmt.Printf("Improvement (chars, percent)    = %g\n", -charImprovement)
	fmt.Printf("Char error rate (before/after)  = %g/%g\n", charErrRateBefore, charErrRateAfter)
	fmt.Printf("Char errors (before/after)      = %d/%d\n", s.charErrBefore, s.charErrAfter)
	fmt.Printf("Total chars                     = %d\n", s.charTotal)
	fmt.Printf("Susp. error rate (before/after) = %g/%g\n", suspErrRateBefore, suspErrRateAfter)
	fmt.Printf("Susp. accuracy (before/after)   = %g/%g\n", suspAccBefore, suspAccAfter)
	fmt.Printf("Suspicious tokens               = %d\n", s.suspTotal)
	fmt.Printf("Improvement (tokens, percent)   = %g\n", improvement)
	fmt.Printf("Error rate (before/after)       = %g/%g\n", tokenErrRateBefore, tokenErrRateAfter)
	fmt.Printf("Accuracy (before/after)         = %g/%g\n", accBefore, accAfter)
	fmt.Printf("Total errors (before/after)     = %d/%d\n", s.tokenErrBefore, s.tokenErrAfter)
	fmt.Printf("Correct (before/after)          = %d/%d\n", corbefore, corafter)
	fmt.Printf("Total tokens                    = %d\n", s.tokenTotal)
	if !verbose {
		fmt.Printf("Successful corrections          = %d\n", s.types[internal.SuccessfulCorrection])
		fmt.Printf("Missed opportunities            = %d\n", s.types[internal.MissedOpportunity])
		fmt.Printf("Infelicitous corrections        = %d\n", s.types[internal.InfelicitousCorrection])
		fmt.Printf("False friends                   = %d\n", s.types[internal.FalseFriend])
		fmt.Printf("Short errors                    = %d\n", s.types[internal.SkippedShortErr])
		fmt.Printf("Merges                          = %d\n", s.skippedMerges+s.merges)
		fmt.Printf("Splits                          = %d\n", s.skippedSplits+s.splits)
		return
	}
	totalSkippedShort := s.types[internal.SkippedShort] + s.types[internal.SkippedShortErr]
	totalSkippedLex := s.types[internal.SkippedLex] + s.types[internal.FalseFriend]
	totalSkippedNoCand := s.types[internal.SkippedNoCand] + s.types[internal.SkippedNoCandErr]
	totalSkipped := totalSkippedShort + totalSkippedLex + totalSkippedNoCand
	fmt.Printf("├─ skipped                      = %d\n", totalSkipped)
	fmt.Printf("│  ├─ short                     = %d\n", totalSkippedShort)
	fmt.Printf("│  │  └─ errors                 = %d\n", s.types[internal.SkippedShortErr])
	fmt.Printf("│  ├─ no candidate              = %d\n", totalSkippedNoCand)
	fmt.Printf("│  │  └─ errors                 = %d\n", s.types[internal.SkippedNoCandErr])
	fmt.Printf("│  └─ lexicon entries           = %d\n", totalSkippedLex)
	fmt.Printf("│     └─ false friends          = %d\n", s.types[internal.FalseFriend])
	totalSusp := s.tokenTotal - totalSkipped
	totalSuspReplCor := s.types[internal.SuspiciousReplacedCorrect] + s.types[internal.InfelicitousCorrection]
	totalSuspReplNotCor := s.types[internal.SuccessfulCorrection] + s.types[internal.DoNotCareCorrection]
	totalSuspRepl := totalSuspReplCor + totalSuspReplNotCor
	fmt.Printf("└─ suspicious                   = %d\n", totalSusp)
	fmt.Printf("   ├─ replaced                  = %d\n", totalSuspRepl)
	fmt.Printf("   │  ├─ ocr correct            = %d\n", totalSuspReplCor)
	fmt.Printf("   │  │  ├─ redundant corr      = %d\n", s.types[internal.SuspiciousReplacedCorrect])
	fmt.Printf("   │  │  └─ infelicitous corr   = %d\n", s.types[internal.InfelicitousCorrection])
	fmt.Printf("   │  │     ├─ bad rank         = %d\n", s.causes[internal.InfelicitousCorrection][internal.BadRank])
	fmt.Printf("   │  │     ├─ bad limit        = %d\n", s.causes[internal.InfelicitousCorrection][internal.BadLimit])
	fmt.Printf("   │  │     └─ missing corr     = %d\n", s.causes[internal.InfelicitousCorrection][internal.MissingCandidate])
	fmt.Printf("   │  └─ ocr not correct        = %d\n", totalSuspReplNotCor)
	fmt.Printf("   │     ├─ successful corr     = %d\n", s.types[internal.SuccessfulCorrection])
	fmt.Printf("   │     └─ do not care         = %d\n", s.types[internal.DoNotCareCorrection])
	fmt.Printf("   │        ├─ bad rank         = %d\n", s.causes[internal.DoNotCareCorrection][internal.BadRank])
	fmt.Printf("   │        ├─ bad limit        = %d\n", s.causes[internal.DoNotCareCorrection][internal.BadLimit])
	fmt.Printf("   │        └─ missing corr     = %d\n", s.causes[internal.DoNotCareCorrection][internal.MissingCandidate])
	totalSuspNotReplCor := s.types[internal.SuspiciousNotReplacedCorrect] + s.types[internal.DodgedBullet]
	totalSuspNotReplNotCor := s.types[internal.MissedOpportunity] + s.types[internal.SuspiciousNotReplacedNotCorrectErr]
	totalSuspNotRepl := totalSuspNotReplCor + totalSuspNotReplNotCor
	fmt.Printf("   └─ not replaced              = %d\n", totalSuspNotRepl)
	fmt.Printf("      ├─ ocr correct            = %d\n", totalSuspNotReplCor)
	fmt.Printf("      │  ├─ ocr accept          = %d\n", s.types[internal.SuspiciousNotReplacedCorrect])
	fmt.Printf("      │  └─ dodged bullets      = %d\n", s.types[internal.DodgedBullet])
	fmt.Printf("      │     ├─ bad rank         = %d\n", s.causes[internal.DodgedBullet][internal.BadRank])
	fmt.Printf("      │     ├─ bad limit        = %d\n", s.causes[internal.DodgedBullet][internal.BadLimit])
	fmt.Printf("      │     └─ missing corr     = %d\n", s.causes[internal.DodgedBullet][internal.MissingCandidate])
	fmt.Printf("      └─ ocr not correct        = %d\n", totalSuspNotReplNotCor)
	fmt.Printf("         ├─ missed opportunity  = %d\n", s.types[internal.MissedOpportunity])
	fmt.Printf("         └─ skipped do not care = %d\n", s.types[internal.SuspiciousNotReplacedNotCorrectErr])
	fmt.Printf("            ├─ bad rank         = %d\n", s.causes[internal.SuspiciousNotReplacedNotCorrectErr][internal.BadRank])
	fmt.Printf("            ├─ bad limit        = %d\n", s.causes[internal.SuspiciousNotReplacedNotCorrectErr][internal.BadLimit])
	fmt.Printf("            └─ missing corr     = %d\n", s.causes[internal.SuspiciousNotReplacedNotCorrectErr][internal.MissingCandidate])
}

func errorRates(before, after, total int) (float64, float64) {
	return float64(before) / float64(total), float64(after) / float64(total)
}

func (s *stats) improvement() float64 {
	corbefore, corafter := s.tokenTotal-s.tokenErrBefore, s.tokenTotal-s.tokenErrAfter
	return (float64(corafter-corbefore) / float64(corbefore)) * 100.0
}

func (s *stats) json(name string) {
	data := s.data(name)
	chk(json.NewEncoder(os.Stdout).Encode(data))
}

func (s *stats) data(name string) map[string]interface{} {
	data := make(map[string]interface{})
	errRateBefore, errRateAfter := errorRates(s.tokenErrBefore, s.tokenErrAfter, s.tokenTotal)
	charErrRateBefore, charErrRateAfter := errorRates(s.charErrBefore, s.charErrAfter, s.charTotal)
	accBefore, accAfter := 1.0-errRateBefore, 1.0-errRateAfter
	corbefore, corafter := s.tokenTotal-s.tokenErrBefore, s.tokenTotal-s.tokenErrAfter
	improvement := s.improvement()
	data["Name"] = name
	data["AccuracyBefore"] = accBefore
	data["AccuracyAfter"] = accAfter
	data["ErrorRateBefore"] = errRateBefore
	data["ErrorRateAfter"] = errRateAfter
	data["CharErrorRateBefore"] = charErrRateBefore
	data["CharErrorRateAfter"] = charErrRateAfter
	data["CorrectBefore"] = corbefore
	data["CorrectAfter"] = corafter
	data["ErrorsBefore"] = s.tokenErrBefore
	data["ErrorsAfter"] = s.tokenErrAfter
	data["Improvement"] = improvement
	data["Total"] = s.tokenTotal
	for typ, count := range s.types {
		switch {
		case typ.Skipped():
			data[typ.String()] = count
		case typ.Err():
			data[typ.String()+internal.BadRank.String()] = s.causes[typ][internal.BadRank]
			data[typ.String()+internal.BadLimit.String()] = s.causes[typ][internal.BadLimit]
			data[typ.String()+internal.MissingCandidate.String()] = s.causes[typ][internal.MissingCandidate]
		default:
			data[typ.String()] = count
		}
	}
	return data
}

type typeMap map[internal.StokType]int

func (m *typeMap) put(typ internal.StokType) {
	if *m == nil {
		*m = make(typeMap)
	}
	(*m)[typ]++
}

type causeMap map[internal.StokType]map[internal.StokCause]int

func (m *causeMap) put(typ internal.StokType, cause internal.StokCause) {
	if *m == nil {
		*m = make(causeMap)
	}
	if (*m)[typ] == nil {
		(*m)[typ] = make(map[internal.StokCause]int)
	}
	(*m)[typ][cause]++
}
