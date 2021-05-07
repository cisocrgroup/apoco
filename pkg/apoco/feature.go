package apoco

import (
	"fmt"
	"math"
	"unicode/utf8"

	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"git.sr.ht/~flobar/lev"
	"github.com/finkf/gofiler"
)

// registered names for feature functions
var register = map[string]FeatureFunc{
	"AgreeingOCRs":                   AgreeingOCRs,
	"OCRTokenLen":                    OCRTokenLen,
	"OCRUnigramFreq":                 OCRUnigramFreq,
	"OCRTrigramFreq":                 OCRTrigramFreq,
	"OCRMaxTrigramFreq":              OCRMinTrigramFreq,
	"OCRMinTrigramFreq":              OCRMaxTrigramFreq,
	"OCRMaxCharConf":                 OCRMaxCharConf,
	"OCRMinCharConf":                 OCRMinCharConf,
	"OCRLevenshteinDist":             OCRLevenshteinDist,
	"CandidateProfilerWeight":        CandidateProfilerWeight,
	"CandidateUnigramFreq":           CandidateUnigramFreq,
	"CandidateTrigramFreq":           CandidateTrigramFreq,
	"CandidateTrigramFreqLog":        CandidateTrigramFreqLog,
	"CandidateAgreeingOCR":           CandidateAgreeingOCR,
	"CandidateOCRPatternConf":        CandidateOCRPatternConf,
	"CandidateOCRPatternConfLog":     CandidateOCRPatternConfLog,
	"CandidateHistPatternConf":       CandidateHistPatternConf,
	"CandidateHistPatternConfLog":    CandidateHistPatternConfLog,
	"CandidateLevenshteinDist":       CandidateLevenshteinDist,
	"CandidateMaxTrigramFreq":        CandidateMaxTrigramFreq,
	"CandidateMinTrigramFreq":        CandidateMinTrigramFreq,
	"CandidateLen":                   CandidateLen,
	"CandidateMatchesOCR":            CandidateMatchesOCR,
	"RankingConf":                    RankingConf,
	"RankingConfDiffToNext":          RankingConfDiffToNext,
	"RankingCandidateConfDiffToNext": RankingCandidateConfDiffToNext,
	"DocumentLexicality":             DocumentLexicality,
	"SplitOtherOCR":                  splitOtherOCR,
	"SplitNumShortTokens":            splitNumShortTokens,
	"SplitUnigramTokenConf":          splitUnigramTokenConf,
	"SplitIsLexiconEntry":            isLexiconEntry,
}

// FeatureFunc defines the function a feature needs to implement.  A
// feature func gets a token and a configuration (the current
// OCR-index i and the total number of parallel OCRs n).  The function
// then should return the feature value for the given token and whether
// this feature applies for the given configuration (i and n).
type FeatureFunc func(t T, i, n int) (float64, bool)

// FeatureSet is just a list of feature funcs.
type FeatureSet []FeatureFunc

// NewFeatureSet creates a new feature set from the list of feature
// function names.
func NewFeatureSet(names ...string) (FeatureSet, error) {
	funcs := make([]FeatureFunc, len(names))
	for i, name := range names {
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
func (fs FeatureSet) Calculate(xs []float64, t T, n int) []float64 {
	for _, f := range fs {
		for i := 0; i < n; i++ {
			if val, ok := f(t, i, n); ok {
				xs = append(xs, val)
			}
		}
	}
	return xs
}

// Names returns the names of the features including the features for
// different values of OCR's.  This function panics if the length of the
// feature set differs from the length of the given feature names.
func (fs FeatureSet) Names(names []string, typ string, nocr int) []string {
	if len(names) != len(fs) {
		panic("bad names")
	}
	var ret []string
	// Create dummy tokens to test if the features activate.
	t := T{
		Tokens:   make([]string, nocr+1),
		Document: new(Document),
	}
	switch typ {
	case "dm":
		t.Payload = []Ranking{{Candidate: new(gofiler.Candidate)}}
	case "rr":
		t.Payload = new(gofiler.Candidate)
	case "mrg":
		t.Payload = Split{
			Tokens:     []T{{Tokens: make([]string, nocr+1)}},
			Candidates: []gofiler.Candidate{{}},
		}
	default:
		panic("bad type: " + typ)
	}
	// Iterate over the features, check if a feature is active
	// for a given configuration using the dummy token and append
	// the feature name to the results.
	for fi, f := range fs {
		for i := 0; i < nocr; i++ {
			if _, ok := f(t, i, nocr); !ok {
				continue
			}
			ret = append(ret, fmt.Sprintf("%s/%d", names[fi], i+1))
		}
	}
	return ret
}

// OCRTokenLen returns the length of the OCR token.  It operates on
// any configuration.
func OCRTokenLen(t T, i, n int) (float64, bool) {
	return float64(len(t.Tokens[0])), true
}

// AgreeingOCRs returns the number of OCRs that aggree with the master
// OCR token.
func AgreeingOCRs(t T, i, n int) (float64, bool) {
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
func OCRUnigramFreq(t T, i, n int) (float64, bool) {
	return t.Document.Unigram(t.Tokens[i]), true
}

// OCRTrigramFreq returns the product of the OCR token's trigrams.
func OCRTrigramFreq(t T, i, n int) (float64, bool) {
	return t.Document.Trigram(t.Tokens[i]), true
}

// OCRMaxCharConf returns the maximal character confidence of the
// master OCR token.
func OCRMaxCharConf(t T, i, n int) (float64, bool) {
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
func OCRMinCharConf(t T, i, n int) (float64, bool) {
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
func OCRMaxTrigramFreq(t T, i, n int) (float64, bool) {
	max := 0.0
	t.Document.EachTrigram(t.Tokens[i], func(conf float64) {
		if max < conf {
			max = conf
		}
	})
	return max, true
}

// OCRMinTrigramFreq returns the minimal trigram relative frequenzy
// confidence of the tokens.
func OCRMinTrigramFreq(t T, i, n int) (float64, bool) {
	min := 1.0
	t.Document.EachTrigram(t.Tokens[i], func(conf float64) {
		if min > conf {
			min = conf
		}
	})
	return min, true
}

// CandidateProfilerWeight returns the profiler confidence value for
// tokens candidate.
func CandidateProfilerWeight(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := t.Payload.(*gofiler.Candidate)
	return float64(candidate.Weight), true
}

// CandidateUnigramFreq returns the relative frequency of the token's
// candidate.
func CandidateUnigramFreq(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := mustGetCandidate(t)
	return t.Document.Unigram(candidate.Suggestion), true
}

// CandidateTrigramFreq returns the product of the candidate's
// trigrams.
func CandidateTrigramFreq(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := mustGetCandidate(t)
	return t.Document.Trigram(candidate.Suggestion), true
}

// CandidateTrigramFreqLog returns the product of the candidate's
// trigrams.
func CandidateTrigramFreqLog(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := mustGetCandidate(t)
	return t.Document.TrigramLog(candidate.Suggestion), true
}

// CandidateAgreeingOCR returns the number of OCR tokens that agree
// with the specific profiler candidate of the token.
func CandidateAgreeingOCR(t T, i, n int) (float64, bool) {
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
func CandidateHistPatternConf(t T, i, n int) (float64, bool) {
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

// CandidateHistPatternConfLog returns the sum of the logrithm of the
// confidences of the primary OCR characters for the assumed
// historical rewrite pattern of the connected candidate.
func CandidateHistPatternConfLog(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := mustGetCandidate(t)
	if len(candidate.HistPatterns) == 0 {
		return 0, true
	}
	var sum float64
	for _, p := range candidate.HistPatterns {
		sum += math.Log(averagePosPatternConf(t.Chars, p))
	}
	return sum, true
}

// CandidateOCRPatternConf returns the product of the confidences of
// the primary OCR characters for the assumed OCR error pattern of the
// connected candidate.
// TODO: rename to CandiateErrPatternConf
func CandidateOCRPatternConf(t T, i, n int) (float64, bool) {
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

// CandidateOCRPatternConfLog returns the sum of the logarithm of the
// confidences of the primary OCR characters for the assumed OCR error
// pattern of the connected candidate.
func CandidateOCRPatternConfLog(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := mustGetCandidate(t)
	if len(candidate.OCRPatterns) == 0 {
		return 0, true
	}
	var sum float64
	for _, p := range candidate.OCRPatterns {
		sum += math.Log(averagePosPatternConf(t.Chars, p))
	}
	return sum, true
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
func CandidateMatchesOCR(t T, i, n int) (float64, bool) {
	candidate := mustGetCandidate(t)
	return ml.Bool(candidate.Suggestion == t.Tokens[i]), true
}

// OCRLevenshteinDist returns the levenshtein distance between the
// secondary OCRs with the primary OCR.
func OCRLevenshteinDist(t T, i, n int) (float64, bool) {
	if i == 0 {
		return 0, false
	}
	return float64(lev.Distance(t.Tokens[i], t.Tokens[0])), true
}

// CandidateLevenshteinDist returns the levenshtein distance between
// the OCR token and the token's connected profiler candidate.  For
// the master OCR the according Distance from the profiler candidate
// is used, whereas for support OCRs the levenshtein distance is
// calculated.
func CandidateLevenshteinDist(t T, i, n int) (float64, bool) {
	candidate := mustGetCandidate(t)
	if i == 0 {
		return float64(candidate.Distance), true
	}
	return float64(lev.Distance(t.Tokens[i], candidate.Suggestion)), true
}

// CandidateLen returns the length of the connected profiler
// candidate.
func CandidateLen(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := mustGetCandidate(t)
	len := utf8.RuneCountInString(candidate.Suggestion)
	return float64(len), true
}

// CandidateMaxTrigramFreq returns the maximal trigram frequenzy for
// the connected candidate.
func CandidateMaxTrigramFreq(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := mustGetCandidate(t)
	max := 0.0
	t.Document.EachTrigram(candidate.Suggestion, func(conf float64) {
		if max < conf {
			max = conf
		}
	})
	return max, true
}

// CandidateMinTrigramFreq returns the minimal trigram frequezy for
// the connected candidate.
func CandidateMinTrigramFreq(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := mustGetCandidate(t)
	min := 1.0
	t.Document.EachTrigram(candidate.Suggestion, func(conf float64) {
		if min > conf {
			min = conf
		}
	})
	return min, true
}

func mustGetCandidate(t T) *gofiler.Candidate {
	switch tx := t.Payload.(type) {
	case *gofiler.Candidate:
		return tx
	case []Ranking:
		return tx[0].Candidate
	case Split:
		return &tx.Candidates[0]
	default:
		panic(fmt.Sprintf("mustGetCandidate: bad type: %T", tx))
	}
}

// RankingConf returns the confidence of the best ranked correction
// candidate for the given token.
func RankingConf(t T, i, n int) (float64, bool) {
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
func RankingConfDiffToNext(t T, i, n int) (float64, bool) {
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

// RankingCandidateConfDiffToNext returns the top ranked candidate's
// weight minus the the weight of the next (or 0).
func RankingCandidateConfDiffToNext(t T, i, n int) (float64, bool) {
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

// DocumentLexicality returns the (global) lexicality of the given
// token's document.  Using this feature only makes sense if the
// training contains at least more than one training document.
func DocumentLexicality(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	return t.Document.Lexicality, true
}

func splitOtherOCR(t T, i, n int) (float64, bool) {
	if i == 0 {
		return 0, false
	}
	ts := t.Payload.(Split).Tokens
	for j := 1; j < len(ts); j++ {
		if ts[j-1].Tokens[i] != ts[j].Tokens[i] {
			return ml.False, true
		}
	}
	return ml.True, true
}

func splitNumShortTokens(t T, i, n int) (float64, bool) {
	ts := t.Payload.(Split).Tokens
	var sum int
	for j := range ts {
		if utf8.RuneCountInString(ts[j].Tokens[i]) < 4 {
			sum++
		}
	}
	return float64(sum), true
}

func splitUnigramTokenConf(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	ts := t.Payload.(Split).Tokens
	var sum float64
	for j := range ts {
		sum += math.Log(t.Document.Unigram(ts[j].Tokens[i]))
	}
	return sum, true
}

func isLexiconEntry(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	cands := t.Payload.(Split).Candidates
	if len(cands) == 1 && len(cands[0].OCRPatterns) == 0 && len(cands[0].HistPatterns) == 0 {
		return ml.True, true
	}
	return ml.False, true
}
