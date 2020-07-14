package apoco

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"example.com/apoco/pkg/apoco/lev"
	"example.com/apoco/pkg/apoco/ml"
	"github.com/finkf/gofiler"
)

// registered names for feature functions
var register = map[string]FeatureFunc{
	"AgreeingOCRs":                   AgreeingOCRs,
	"OCRTokenConf":                   OCRTokenConf,
	"OCRTokenLen":                    OCRTokenLen,
	"OCRUnigramFreq":                 OCRUnigramFreq,
	"OCRTrigramFreq":                 OCRTrigramFreq,
	"OCRMaxTrigramFreq":              OCRMinTrigramFreq,
	"OCRMinTrigramFreq":              OCRMaxTrigramFreq,
	"OCRFirstInLine":                 OCRFirstInLine,
	"OCRLastInLine":                  OCRLastInLine,
	"OCRMaxCharConf":                 OCRMaxCharConf,
	"OCRMinCharConf":                 OCRMinCharConf,
	"CandidateProfilerWeight":        CandidateProfilerWeight,
	"CandidateUnigramFreq":           CandidateUnigramFreq,
	"CandidateTrigramFreq":           CandidateTrigramFreq,
	"CandidateAgreeingOCR":           CandidateAgreeingOCR,
	"CandidateOCRPatternConf":        CandidateOCRPatternConf,
	"CandidateHistPatternConf":       CandidateHistPatternConf,
	"CandidateLevenshteinDist":       CandidateLevenshteinDist,
	"CandidateMaxTrigramFreq":        CandidateMaxTrigramFreq,
	"CandidateMinTrigramFreq":        CandidateMinTrigramFreq,
	"CandidateLen":                   CandidateLen,
	"CandidateMatchesOCR":            CandidateMatchesOCR,
	"RankingConf":                    RankingConf,
	"RankingConfDiffToNext":          RankingConfDiffToNext,
	"RankingCandidateConf":           RankingCandidateConf,
	"RankingCandidateConfDiffToNext": RankingCandidateConfDiffToNext,
	"RankingCandidateUnigramFreq":    RankingCandidateUnigramFreq,
	"RankingCandidateTrigramFreq":    RankingCandidateTrigramFreq,
	"GoodOCRPatterns":                GoodOCRPatterns,
	"GoodHistPatterns":               GoodHistPatterns,
}

// FeatureFunc defines the function a feature needs to implement.  A
// feature func gets a token and a configuration (the current
// OCR-index i and the total number of parallel OCRs n).  The function
// then should return the feature value for the given token and wether
// this feature applies for the given configuration (i and n).
type FeatureFunc func(t Token, i, n int) (float64, bool)

// FeatureSet is just a list of feature funcs.
type FeatureSet []FeatureFunc

// NewFeatureSet creates a new feature set from the list of feature
// function names.
func NewFeatureSet(names ...string) (FeatureSet, error) {
	funcs := make([]FeatureFunc, len(names))
	for i, name := range names {
		if strings.HasPrefix(name, "HasOCRPattern_") {
			funcs[i] = HasOCRPattern(name[14:])
			continue
		}
		if strings.HasPrefix(name, "HasHistPattern_") {
			funcs[i] = HasHistPattern(name[15:])
			continue
		}
		f, ok := register[name]
		if !ok {
			return nil, fmt.Errorf("newFeatureSet %s: no such feature function", name)
		}
		funcs[i] = f
	}
	return funcs, nil
}

// Calculate calculates the feature vector for the given feature
// functions for the given token and the given number of OCRs and
// appends it to the given vector.  Any given feature function that
// does not apply to the given configuration (and returns false as it
// second return parameter for the configuration) is omitted and not
// appended to the resulting feature vector.
func (fs FeatureSet) Calculate(t Token, n int, xs []float64) []float64 {
	// ret := make([]float64, 0, n*len(fs))
	for _, f := range fs {
		for i := 0; i < n; i++ {
			if val, ok := f(t, i, n); ok {
				xs = append(xs, val)
				// ret = append(ret, val)
			}
		}
	}
	return xs
	// return ret
}

// OCRTokenLen returns the length of the OCR token.  It operates on
// any configuration.
func OCRTokenLen(t Token, i, n int) (float64, bool) {
	return float64(len(t.Tokens[0])), true
}

// OCRTokenConf return the OCR-confidence for the the given
// configuration.
func OCRTokenConf(t Token, i, n int) (float64, bool) {
	return t.Confs[i], true
}

// AgreeingOCRs returns the number of OCRs that aggree with the master
// OCR token.
func AgreeingOCRs(t Token, i, n int) (float64, bool) {
	if i != 0 || n == 1 {
		return 0, false
	}
	var ret int
	for j := 1; j < n; j++ {
		if t.Tokens[j] == t.Tokens[0] {
			ret++
		}
	}
	return float64(ret), true
}

// OCRUnigramFreq returns the relative frequency of the OCR token in
// the unigram language model.
func OCRUnigramFreq(t Token, i, n int) (float64, bool) {
	return t.LM.Unigram(t.Tokens[i]), true
}

// OCRTrigramFreq returns the product of the OCR token's trigrams.
func OCRTrigramFreq(t Token, i, n int) (float64, bool) {
	return t.LM.Trigram(t.Tokens[i]), true
}

// OCRFirstInLine checks if the given token is the first in a line.
func OCRFirstInLine(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	return ml.Bool(t.HasTrait(0, FirstInLine)), true
}

// OCRLastInLine checks if the given token is the first in a line.
func OCRLastInLine(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	return ml.Bool(t.HasTrait(0, LastInLine)), true
}

// OCRMaxCharConf returns the maximal character confidence of the
// master OCR token.
func OCRMaxCharConf(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	max := 0.0
	for _, c := range t.Chars {
		if max < c.Conf {
			max = c.Conf
		}
	}
	return max, true
}

// OCRMinCharConf returns the minimal character confidence of the
// master OCR token.
func OCRMinCharConf(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	min := 1.0
	for _, c := range t.Chars {
		if min > c.Conf {
			min = c.Conf
		}
	}
	return min, true
}

// OCRMaxTrigramFreq returns the maximal trigram relative frequenzy
// confidence of the tokens.
func OCRMaxTrigramFreq(t Token, i, n int) (float64, bool) {
	max := 0.0
	t.LM.EachTrigram(t.Tokens[i], func(conf float64) {
		if max < conf {
			max = conf
		}
	})
	return max, true
}

// OCRMinTrigramFreq returns the minimal trigram relative frequenzy
// confidence of the tokens.
func OCRMinTrigramFreq(t Token, i, n int) (float64, bool) {
	min := 1.0
	t.LM.EachTrigram(t.Tokens[i], func(conf float64) {
		if min > conf {
			min = conf
		}
	})
	return min, true
}

// CandidateProfilerWeight returns the profiler confidence value for
// tokens candidate.
func CandidateProfilerWeight(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := t.Payload.(*gofiler.Candidate)
	return float64(candidate.Weight), true
}

// CandidateUnigramFreq returns the relative frequency of the token's
// candidate.
func CandidateUnigramFreq(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := t.Payload.(*gofiler.Candidate)
	return t.LM.Unigram(candidate.Suggestion), true
}

// CandidateTrigramFreq returns the product of the candidate's
// trigrams.
func CandidateTrigramFreq(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := t.Payload.(*gofiler.Candidate)
	return t.LM.Trigram(candidate.Suggestion), true
}

// CandidateAgreeingOCR returns the number of OCR tokens that agree
// with the specific profiler candidate of the token.
func CandidateAgreeingOCR(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := mustGetCandidate(t)
	var ret int
	for j := 0; j < n; j++ {
		if t.Tokens[j] == candidate.Suggestion {
			ret++
		}
	}
	return float64(ret), true
}

// CandidateHistPatternConf returns the product of the confidences of
// the primary OCR characters for the assumed historical rewrite
// pattern of the connected candidate.
func CandidateHistPatternConf(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := mustGetCandidate(t)
	if len(candidate.HistPatterns) == 0 {
		return 0, true
	}
	prod := 1.0
	for _, p := range candidate.HistPatterns {
		prod *= averagePosPatternConf(t.Chars, p)
	}
	return prod, true
}

// CandidateOCRPatternConf returns the product of the confidences of
// the primary OCR characters for the assumed OCR error pattern of the
// connected candidate.
func CandidateOCRPatternConf(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := mustGetCandidate(t)
	if len(candidate.OCRPatterns) == 0 {
		return 0, true
	}
	prod := 1.0
	for _, p := range candidate.OCRPatterns {
		prod *= averagePosPatternConf(t.Chars, p)
	}
	return prod, true
}

func averagePosPatternConf(chars Chars, p gofiler.Pattern) float64 {
	if len(chars) == 0 || p.Pos < 0 {
		return 0
	}
	if len(p.Right) == 0 { // deletion
		if p.Pos == 0 {
			return chars[0].Conf
		} else if p.Pos >= len(chars) {
			return chars[len(chars)-1].Conf
		}
		return (chars[p.Pos].Conf + chars[p.Pos-1].Conf) / 2.0
	}
	if p.Pos >= len(chars) {
		return chars[len(chars)-1].Conf
	}
	var sum float64
	var n int
	for range p.Right {
		if p.Pos+n >= len(chars) {
			break
		}
		sum += chars[p.Pos+n].Conf
		n++
	}
	return sum / float64(n)
}

// CandidateMatchesOCR returns true if the according ocr matches the
// connected candidate and false otherwise.
func CandidateMatchesOCR(t Token, i, n int) (float64, bool) {
	candidate := mustGetCandidate(t)
	return ml.Bool(candidate.Suggestion == t.Tokens[i]), true
}

// CandidateLevenshteinDist returns the levenshtein distance between
// the OCR token and the token's connected profiler candidate.  For
// the master OCR the according Distance from the profiler candidate
// is used, whereas for support OCRs the levenshtein distance is
// calculated.
func CandidateLevenshteinDist(t Token, i, n int) (float64, bool) {
	candidate := mustGetCandidate(t)
	if i == 0 {
		return float64(candidate.Distance), true
	}
	return float64(lev.Distance(t.Tokens[i], candidate.Suggestion)), true
}

// CandidateLen returns the length of the connected profiler
// candidate.
func CandidateLen(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := mustGetCandidate(t)
	len := utf8.RuneCountInString(candidate.Suggestion)
	return float64(len), true
}

// CandidateMaxTrigramFreq returns the maximal trigram frequenzy for
// the connected candidate.
func CandidateMaxTrigramFreq(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := mustGetCandidate(t)
	max := 0.0
	t.LM.EachTrigram(candidate.Suggestion, func(conf float64) {
		if max < conf {
			max = conf
		}
	})
	return max, true
}

// CandidateMinTrigramFreq returns the minimal trigram frequezy for
// the connected candidate.
func CandidateMinTrigramFreq(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := mustGetCandidate(t)
	min := 1.0
	t.LM.EachTrigram(candidate.Suggestion, func(conf float64) {
		if min > conf {
			min = conf
		}
	})
	return min, true
}

func mustGetCandidate(t Token) *gofiler.Candidate {
	switch tx := t.Payload.(type) {
	case *gofiler.Candidate:
		return tx
	case []Ranking:
		return tx[0].Candidate
	default:
		panic(fmt.Sprintf("mustGetCandidate: bad type: %T", tx))
	}
}

// RankingConf returns the confidence of the best ranked correction
// candidate for the given token.
func RankingConf(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	ranking := t.Payload.([]Ranking)[0]
	return ranking.Prob, true
}

// RankingConfDiffToNext returns the difference of the best ranked
// correction candidate's confidence to the next.  If only one
// correction candidate is available, the next ranking's confidence is
// assumed to be 0.
func RankingConfDiffToNext(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	rankings := t.Payload.([]Ranking)
	next := 0.0
	if len(rankings) > 1 {
		next = rankings[1].Prob
	}
	return rankings[0].Prob - next, true
}

// RankingCandidateTrigramFreq returns the trigram frequency for the
// profiler candidate of the top ranked correction suggestion.
func RankingCandidateTrigramFreq(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	return t.LM.Trigram(t.Payload.([]Ranking)[0].Candidate.Suggestion), true
}

// RankingCandidateUnigramFreq returns the unigram frequency for the
// profiler candidate of the top ranked correction suggestion.
func RankingCandidateUnigramFreq(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	return t.LM.Unigram(t.Payload.([]Ranking)[0].Candidate.Suggestion), true
}

// RankingCandidateConf returns the top ranked candidate's weight.
func RankingCandidateConf(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	return float64(t.Payload.([]Ranking)[0].Candidate.Weight), true
}

// RankingCandidateConfDiffToNext returns the top ranked candidate's
// weight minus the the weight of the next (or 0).
func RankingCandidateConfDiffToNext(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	rankings := t.Payload.([]Ranking)
	next := 0.0
	if len(rankings) > 1 {
		next = float64(rankings[1].Candidate.Weight)
	}
	return float64(rankings[0].Candidate.Weight) - next, true
}
