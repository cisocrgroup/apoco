package apoco

import (
	"fmt"

	"example.com/apoco/pkg/apoco/lev"
	"github.com/finkf/gofiler"
)

// registered names for feature functions
var register = map[string]FeatureFunc{
	"AgreeingOCRs":             AgreeingOCRs,
	"OCRTokenConf":             OCRTokenConf,
	"OCRTokenLen":              OCRTokenLen,
	"OCRUnigramFreq":           OCRUnigramFreq,
	"OCRTrigramFreq":           OCRTrigramFreq,
	"CandidateProfilerWeight":  CandidateProfilerWeight,
	"CandidateUnigramFreq":     CandidateUnigramFreq,
	"CandidateTrigramFreq":     CandidateTrigramFreq,
	"CandidateAgreeingOCR":     CandidateAgreeingOCR,
	"CandidateOCRPatternConf":  CandidateOCRPatternConf,
	"CandidateLevenshteinDist": CandidateLevenshteinDist,
	"RankingConf":              RankingConf,
	"RankingDiffToNext":        RankingDiffToNext,
}

// FeatureFunc defines the function a feature needs to implement.  A
// feature func gets a token and a configuration (the current
// OCR-index i and the total number of OCRs n).  The function then
// should return the feature value for the given token and wether if
// this feature applies for the given configuration (i and n).
type FeatureFunc func(t Token, i, n int) (float64, bool)

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
// functions for the given token and the given number of OCRs.  Any
// given feature function that does not apply to the given
// configuration (and returns false as it second return parameter for
// the configuration) is omitted from the resulting feature vector.
func (fs FeatureSet) Calculate(t Token, n int) []float64 {
	ret := make([]float64, 0, n*len(fs))
	for _, f := range fs {
		for i := 0; i < n; i++ {
			if val, ok := f(t, i, n); ok {
				ret = append(ret, val)
			}
		}
	}
	return ret
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
	var ret int
	suggestion := t.Payload.(*gofiler.Candidate).Suggestion
	for j := 0; j < n; j++ {
		if t.Tokens[j] == suggestion {
			ret++
		}
	}
	return float64(ret), true
}

// CandidateOCRPatternConf returns the average confidence of the
// master OCR characters for the assumed OCR error pattern of the
// connected candidate.
func CandidateOCRPatternConf(t Token, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	candidate := t.Payload.(*gofiler.Candidate)
	if len(candidate.OCRPatterns) == 0 {
		return 0, true
	}
	var sum float64
	for _, p := range candidate.OCRPatterns {
		sum += t.Chars.PatternConf(p)
	}
	return sum / float64(len(candidate.OCRPatterns)), true
}

// CandidateLevenshteinDist returns the levenshtein distance between
// the OCR token and the token's connected profiler candidate.  For
// the master OCR the according Distance from the profiler candidate
// is used, whereas for support OCRs the levenshtein distance is
// calculated.
func CandidateLevenshteinDist(t Token, i, n int) (float64, bool) {
	candidate := t.Payload.(*gofiler.Candidate)
	if i == 0 {
		return float64(candidate.Distance), true
	}
	return float64(lev.Distance(t.Tokens[i], candidate.Suggestion)), true
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

// RankingDiffToNext returns the difference of the best ranked
// correction candidate's confidence to the next.  If only one
// correction candidate is available, the next ranking's confidence is
// assumed to be 0.
func RankingDiffToNext(t Token, i, n int) (float64, bool) {
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
