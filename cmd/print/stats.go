package print

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
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

// statsCmd runs the apoco stats command.
var statsCmd = &cobra.Command{
	Use:   "stats [FILE...]",
	Short: "Extract correction stats",
	Run:   runStats,
}

func init() {
	statsCmd.Flags().StringVarP(&statsFlags.name, "name", "n", "", "set name")
	statsCmd.Flags().IntVarP(&statsFlags.limit, "limit", "L", 0, "set limit for the profiler's candidate set")
	statsCmd.Flags().BoolVarP(&statsFlags.skipShort, "noshort", "s", false,
		"exclude short tokens (len<4) from the evaluation")
	statsCmd.Flags().BoolVarP(&statsFlags.verbose, "verbose", "v", false,
		"enable more verbose error and correction output")
}

func runStats(_ *cobra.Command, args []string) {
	if len(args) > 0 {
		for _, arg := range args {
			chk(statsEachStokInFile(arg))
		}
		return
	}
	var fname string
	var s stats
	chk(internal.EachStok(os.Stdin, func(name string, stok internal.Stok) error {
		if fname == "" {
			fname = name
			return s.stat(stok)
		}
		if fname != name {
			if err := s.output(fname, flags.json, statsFlags.verbose); err != nil {
				return err
			}
			fname = name
			s = stats{}
		}
		return s.stat(stok)
	}))
	chk(s.output(fname, flags.json, statsFlags.verbose))
}

func statsEachStokInFile(name string) error {
	in, err := os.Open(name)
	if err != nil {
		return err
	}
	defer in.Close()
	var s stats
	internal.EachStok(in, func(_ string, stok internal.Stok) error {
		return s.stat(stok)
	})
	if flags.json {
		s.json(name)
		return nil
	}
	return s.output(name, flags.json, statsFlags.verbose)
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

func (s *stats) stat(t internal.Stok) error {
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

func (s *stats) output(name string, json, verbose bool) error {
	if json {
		s.json(name)
		return nil
	}
	s.write(name, verbose)
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "Name\t%s\n", name)
	fmt.Fprintf(w, "Improvement (chars, percent)\t%g\n", -charImprovement)
	fmt.Fprintf(w, "Char error rate (before/after)\t%g/%g\n", charErrRateBefore, charErrRateAfter)
	fmt.Fprintf(w, "Char errors (before/after)\t%d/%d\n", s.charErrBefore, s.charErrAfter)
	fmt.Fprintf(w, "Total chars\t%d\n", s.charTotal)
	fmt.Fprintf(w, "Susp. error rate (before/after)\t%g/%g\n", suspErrRateBefore, suspErrRateAfter)
	fmt.Fprintf(w, "Susp. accuracy (before/after)\t%g/%g\n", suspAccBefore, suspAccAfter)
	fmt.Fprintf(w, "Susp. tokens\t%d\n", s.suspTotal)
	fmt.Fprintf(w, "Improvement (tokens, percent)\t%g\n", improvement)
	fmt.Fprintf(w, "Error rate (before/after)\t%g/%g\n", tokenErrRateBefore, tokenErrRateAfter)
	fmt.Fprintf(w, "Accuracy (before/after)\t%g/%g\n", accBefore, accAfter)
	fmt.Fprintf(w, "Total errors (before/after)\t%d/%d\n", s.tokenErrBefore, s.tokenErrAfter)
	fmt.Fprintf(w, "Correct (before/after)\t%d/%d\n", corbefore, corafter)
	fmt.Fprintf(w, "Total tokens\t%d\n", s.tokenTotal)
	if !verbose {
		fmt.Fprintf(w, "Successful corrections\t%d\n", s.types[internal.SuccessfulCorrection])
		fmt.Fprintf(w, "Missed opportunities\t%d\n", s.types[internal.MissedOpportunity])
		fmt.Fprintf(w, "Infelicitous corrections\t%d\n", s.types[internal.InfelicitousCorrection])
		fmt.Fprintf(w, "False friends\t%d\n", s.types[internal.FalseFriend])
		fmt.Fprintf(w, "Short errors\t%d\n", s.types[internal.SkippedShortErr])
		fmt.Fprintf(w, "Merges\t%d\n", s.skippedMerges+s.merges)
		fmt.Fprintf(w, "Splits\t%d\n", s.skippedSplits+s.splits)
		return
	}
	totalSkippedShort := s.types[internal.SkippedShort] + s.types[internal.SkippedShortErr]
	totalSkippedLex := s.types[internal.SkippedLex] + s.types[internal.FalseFriend]
	totalSkippedNoCand := s.types[internal.SkippedNoCand] + s.types[internal.SkippedNoCandErr]
	totalSkipped := totalSkippedShort + totalSkippedLex + totalSkippedNoCand
	fmt.Fprintf(w, "├─ skipped\t%d\n", totalSkipped)
	fmt.Fprintf(w, "│  ├─ short\t%d\n", totalSkippedShort)
	fmt.Fprintf(w, "│  │  └─ errors\t%d\n", s.types[internal.SkippedShortErr])
	fmt.Fprintf(w, "│  ├─ no candidate\t%d\n", totalSkippedNoCand)
	fmt.Fprintf(w, "│  │  └─ errors\t%d\n", s.types[internal.SkippedNoCandErr])
	fmt.Fprintf(w, "│  └─ lexicon entries\t%d\n", totalSkippedLex)
	fmt.Fprintf(w, "│     └─ false friends\t%d\n", s.types[internal.FalseFriend])
	totalSusp := s.tokenTotal - totalSkipped
	totalSuspReplCor := s.types[internal.RedundantCorrection] + s.types[internal.InfelicitousCorrection]
	totalSuspReplNotCor := s.types[internal.SuccessfulCorrection] + s.types[internal.DoNotCareCorrection]
	totalSuspRepl := totalSuspReplCor + totalSuspReplNotCor
	fmt.Fprintf(w, "└─ suspicious\t%d\n", totalSusp)
	fmt.Fprintf(w, "   ├─ replaced\t%d\n", totalSuspRepl)
	fmt.Fprintf(w, "   │  ├─ ocr correct\t%d\n", totalSuspReplCor)
	fmt.Fprintf(w, "   │  │  ├─ redundant corr\t%d\n", s.types[internal.RedundantCorrection])
	fmt.Fprintf(w, "   │  │  └─ infelicitous corr\t%d\n", s.types[internal.InfelicitousCorrection])
	fmt.Fprintf(w, "   │  │     ├─ bad rank\t%d\n", s.causes[internal.InfelicitousCorrection][internal.BadRank])
	fmt.Fprintf(w, "   │  │     ├─ bad limit\t%d\n", s.causes[internal.InfelicitousCorrection][internal.BadLimit])
	fmt.Fprintf(w, "   │  │     └─ missing corr\t%d\n", s.causes[internal.InfelicitousCorrection][internal.MissingCandidate])
	fmt.Fprintf(w, "   │  └─ ocr not correct\t%d\n", totalSuspReplNotCor)
	fmt.Fprintf(w, "   │     ├─ successful corr\t%d\n", s.types[internal.SuccessfulCorrection])
	fmt.Fprintf(w, "   │     └─ do not care\t%d\n", s.types[internal.DoNotCareCorrection])
	fmt.Fprintf(w, "   │        ├─ bad rank\t%d\n", s.causes[internal.DoNotCareCorrection][internal.BadRank])
	fmt.Fprintf(w, "   │        ├─ bad limit\t%d\n", s.causes[internal.DoNotCareCorrection][internal.BadLimit])
	fmt.Fprintf(w, "   │        └─ missing corr\t%d\n", s.causes[internal.DoNotCareCorrection][internal.MissingCandidate])
	totalSuspNotReplCor := s.types[internal.SuspiciousNotReplacedCorrect] + s.types[internal.DodgedBullet]
	totalSuspNotReplNotCor := s.types[internal.MissedOpportunity] + s.types[internal.SuspiciousNotReplacedNotCorrectErr]
	totalSuspNotRepl := totalSuspNotReplCor + totalSuspNotReplNotCor
	fmt.Fprintf(w, "   └─ not replaced\t%d\n", totalSuspNotRepl)
	fmt.Fprintf(w, "      ├─ ocr correct\t%d\n", totalSuspNotReplCor)
	fmt.Fprintf(w, "      │  ├─ ocr accept\t%d\n", s.types[internal.SuspiciousNotReplacedCorrect])
	fmt.Fprintf(w, "      │  └─ dodged bullets\t%d\n", s.types[internal.DodgedBullet])
	fmt.Fprintf(w, "      │     ├─ bad rank\t%d\n", s.causes[internal.DodgedBullet][internal.BadRank])
	fmt.Fprintf(w, "      │     ├─ bad limit\t%d\n", s.causes[internal.DodgedBullet][internal.BadLimit])
	fmt.Fprintf(w, "      │     └─ missing corr\t%d\n", s.causes[internal.DodgedBullet][internal.MissingCandidate])
	fmt.Fprintf(w, "      └─ ocr not correct\t%d\n", totalSuspNotReplNotCor)
	fmt.Fprintf(w, "         ├─ missed opportunity\t%d\n", s.types[internal.MissedOpportunity])
	fmt.Fprintf(w, "         └─ skipped do not care\t%d\n", s.types[internal.SuspiciousNotReplacedNotCorrectErr])
	fmt.Fprintf(w, "            ├─ bad rank\t%d\n", s.causes[internal.SuspiciousNotReplacedNotCorrectErr][internal.BadRank])
	fmt.Fprintf(w, "            ├─ bad limit\t%d\n", s.causes[internal.SuspiciousNotReplacedNotCorrectErr][internal.BadLimit])
	fmt.Fprintf(w, "            └─ missing corr\t%d\n", s.causes[internal.SuspiciousNotReplacedNotCorrectErr][internal.MissingCandidate])
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
