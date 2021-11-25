package apoco

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"git.sr.ht/~flobar/lev"
	"github.com/finkf/gofiler"
)

func _ff(f FeatureFunc) func([]string) (FeatureFunc, error) {
	return func(args []string) (FeatureFunc, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("no argument allowed for feature")
		}
		return f, nil
	}
}

// registered names for feature functions
var register = map[string]func([]string) (FeatureFunc, error){
	"AgreeingOCRs":                   _ff(AgreeingOCRs),
	"OCRTokenLen":                    _ff(OCRTokenLen),
	"OCRUnigramFreq":                 mkOCRUnigramFreq,
	"OCRTrigramFreq":                 mkOCRTrigramFreq,
	"OCRMaxTrigramFreq":              mkOCRMaxTrigramFreq,
	"OCRMinTrigramFreq":              mkOCRMinTrigramFreq,
	"OCRMaxCharConf":                 _ff(OCRMaxCharConf),
	"OCRMinCharConf":                 _ff(OCRMinCharConf),
	"OCRLevenshteinDist":             _ff(OCRLevDist),
	"OCRLevDist":                     _ff(OCRLevDist),
	"OCRWLevDist":                    _ff(OCRWLevDist),
	"OCRLibFreq":                     _ff(ocrLibFreq),
	"CandidateProfilerWeight":        _ff(CandidateProfilerWeight),
	"CandidateUnigramFreq":           mkCandidateUnigramFreq,
	"CandidateTrigramFreq":           mkCandidateTrigramFreq,
	"CandidateTrigramFreqLog":        mkCandidateTrigramFreqLog,
	"CandidateAgreeingOCR":           _ff(CandidateAgreeingOCR),
	"CandidateOCRPatternConf":        _ff(CandidateOCRPatternConf),
	"CandidateOCRPatternConfLog":     _ff(CandidateOCRPatternConfLog),
	"CandidateHistPatternConf":       _ff(CandidateHistPatternConf),
	"CandidateHistPatternConfLog":    _ff(CandidateHistPatternConfLog),
	"CandidateLevenshteinDist":       _ff(CandidateLevDist),
	"CandidateLevDist":               _ff(CandidateLevDist),
	"CandidateWLevDist":              _ff(CandidateWLevDist),
	"CandidateMaxTrigramFreq":        mkCandidateMaxTrigramFreq,
	"CandidateMinTrigramFreq":        mkCandidateMinTrigramFreq,
	"CandidateLen":                   _ff(CandidateLen),
	"CandidateMatchesOCR":            _ff(CandidateMatchesOCR),
	"RankingConf":                    _ff(RankingConf),
	"RankingConfDiffToNext":          _ff(RankingConfDiffToNext),
	"RankingCandidateConfDiffToNext": _ff(RankingCandidateConfDiffToNext),
	"DocumentLexicality":             _ff(DocumentLexicality),
	"SplitOtherOCR":                  _ff(splitOtherOCR),
	"SplitNumShortTokens":            _ff(splitNumShortTokens),
	"SplitUnigramTokenConf":          _ff(splitUnigramTokenConf),
	"SplitNumberOfLexiconEntries":    _ff(countLexiconEntriesInMergedSplits),
	"SplitIsLexiconEntry":            _ff(isLexiconEntry),
	"SplitLen":                       _ff(splitLen),
	"IsStartOfLine":                  _ff(isSOL),
	"IsEndOfLine":                    _ff(isEOL),
	"FFNumberOfCandidates":           _ff(ffNumberOfCandidates),
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
//
// Feature function names can have optional arguments. The arguments
// of a feature function must be given in a comman-separated list
// enclosed in `()`. For example `feature`, `feature()`,
// `feature(arg1,arg2)` are all valid feature function names.
func NewFeatureSet(names ...string) (FeatureSet, error) {
	fail := func(err error) (FeatureSet, error) {
		return nil, fmt.Errorf("new feature set: %v", err)
	}
	funcs := make([]FeatureFunc, len(names))
	for i, name := range names {
		fname, args, err := parseff(name)
		if err != nil {
			return fail(err)
		}
		f, ok := register[fname]
		if !ok {
			return fail(fmt.Errorf("no such feature function %s", fname))
		}
		ff, err := f(args)
		if err != nil {
			return fail(err)
		}
		funcs[i] = ff
	}
	return funcs, nil
}

var ffre = regexp.MustCompile(`^(.*)\((.*)\)$`)

func parseff(name string) (string, []string, error) {
	m := ffre.FindStringSubmatch(name)
	if len(m) == 0 {
		return name, nil, nil
	}
	args := strings.Split(m[2], ",")
	for i := range args {
		args[i] = strings.Trim(args[i], " \t\n")
	}
	return m[1], args, nil
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
		Tokens: make([]string, nocr+1),
		Document: &Document{
			LM: map[string]*FreqList{"3grams": {}},
		},
	}
	switch typ {
	case "dm", "ff":
		t.Payload = []Ranking{{Candidate: new(gofiler.Candidate)}}
	case "rr":
		t.Payload = new(gofiler.Candidate)
	case "ms":
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

// mkOCRUnigramFreq returns a function that returns the relative
// frequency of the ocr tokens in an external frequency list. If no
// arguments are given, the ocr-document frequencies are used.
func mkOCRUnigramFreq(args []string) (FeatureFunc, error) {
	if len(args) == 0 {
		return func(t T, i, n int) (float64, bool) {
			return t.Document.Unigram(t.Tokens[i]), true
		}, nil
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("ocr unigram freq: bad arguments %v", args)
	}
	return func(t T, i, n int) (float64, bool) {
		return t.Document.LM[args[0]].relative(t.Tokens[i]), true
	}, nil
}

func lmFail(name string, err error) (FeatureFunc, error) {
	return nil, fmt.Errorf("%s: %v", name, err)
}

func mkOCRTrigramFreq(args []string) (FeatureFunc, error) {
	if len(args) != 1 {
		return lmFail("ocr trigram freq", fmt.Errorf("bad arguments: %v", args))
	}
	return func(t T, i, n int) (float64, bool) {
		return ocrTrigramFreq(args[0], t, i, n)
	}, nil
}

// ocrTrigramFreq returns the product of the OCR token's trigrams.
func ocrTrigramFreq(lm string, t T, i, n int) (float64, bool) {
	prod := 1.0
	t.Document.LM[lm].EachTrigram(t.Tokens[i], func(val float64) {
		prod *= val
	})
	return prod, true
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

// mkOCRMaxTrigramFreq returns a feature function that calculates the
// maximal trigram relative frequenzy of the tokens.
func mkOCRMaxTrigramFreq(args []string) (FeatureFunc, error) {
	if len(args) != 1 {
		return lmFail("ocr max trigram freq", fmt.Errorf("bad arguments: %v", args))
	}
	lm := args[0]
	return func(t T, i, n int) (float64, bool) {
		max := 0.0
		t.Document.LM[lm].EachTrigram(t.Tokens[i], func(conf float64) {
			if max < conf {
				max = conf
			}
		})
		return max, true
	}, nil
}

// mkOCRMinTrigramFreq returns a feature function that calculates the
// minimal trigram relative frequenzy of the tokens.
func mkOCRMinTrigramFreq(args []string) (FeatureFunc, error) {
	if len(args) != 1 {
		return lmFail("ocr min trigram freq", fmt.Errorf("bad arguments: %v", args))
	}
	lm := args[0]
	return func(t T, i, n int) (float64, bool) {
		min := 0.0
		t.Document.LM[lm].EachTrigram(t.Tokens[i], func(conf float64) {
			if min > conf {
				min = conf
			}
		})
		return min, true
	}, nil
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

// mkCandidateUnigramFreq returns a function that returns the relative
// frequency of the candidate in a frequency list. If no arguments are
// given, the master ocr document is used instead of an external
// frequency list.
func mkCandidateUnigramFreq(args []string) (FeatureFunc, error) {
	if len(args) == 0 {
		return func(t T, i, n int) (float64, bool) {
			if i != 0 {
				return 0, false
			}
			candidate := mustGetCandidate(t)
			return t.Document.Unigram(candidate.Suggestion), true
		}, nil
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("candidate unigram freq: bad arguments %v", args)
	}
	return func(t T, i, n int) (float64, bool) {
		if i != 0 {
			return 0, false
		}
		candidate := mustGetCandidate(t)
		return t.Document.LM[args[0]].relative(candidate.Suggestion), true
	}, nil
}

// mkCandidateTrigramFreq returns a feature function that calculates
// the product of the candidate's trigrams.
func mkCandidateTrigramFreq(args []string) (FeatureFunc, error) {
	if len(args) != 1 {
		return lmFail("ocr trigram freq", fmt.Errorf("bad arguments: %v", args))
	}
	lm := args[0]
	return func(t T, i, n int) (float64, bool) {
		if i != 0 {
			return 0, false
		}
		candidate := mustGetCandidate(t)
		prod := 1.0
		t.Document.LM[lm].EachTrigram(candidate.Suggestion, func(val float64) {
			prod *= val
		})
		return prod, true
	}, nil
}

// mkCandidateTrigramFreqLog returns a feature function that calculates
// the sum of the logaritm of the candidate's trigrams.
func mkCandidateTrigramFreqLog(args []string) (FeatureFunc, error) {
	if len(args) != 1 {
		return lmFail("ocr trigram freq log", fmt.Errorf("bad arguments: %v", args))
	}
	lm := args[0]
	return func(t T, i, n int) (float64, bool) {
		if i != 0 {
			return 0, false
		}
		candidate := mustGetCandidate(t)
		sum := 0.0
		t.Document.LM[lm].EachTrigram(candidate.Suggestion, func(val float64) {
			sum += math.Log(val)
		})
		return sum, true
	}, nil
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

// OCRLevDist returns the levenshtein distance between the
// secondary OCRs with the primary OCR.
func OCRLevDist(t T, i, n int) (float64, bool) {
	if i == 0 {
		return 0, false
	}
	return float64(lev.Distance(t.Tokens[i], t.Tokens[0])), true
}

// OCRWLevDist returns the weighted levenshtein distance
// between the secondary OCRs with the primary OCR.
func OCRWLevDist(t T, i, n int) (float64, bool) {
	if i == 0 {
		return 0, false
	}
	m := lev.NewWMat(t.Document.LookupOCRPattern)
	return m.Distance(t.Tokens[i], t.Tokens[0]), true
}

func ocrLibFreq(t T, i, n int) (float64, bool) {
	return t.Document.LM["lib"].relative(t.Tokens[i]), true
}

// CandidateLevDist returns the levenshtein distance between
// the OCR token and the token's connected profiler candidate.  For
// the master OCR the according Distance from the profiler candidate
// is used, whereas for support OCRs the levenshtein distance is
// calculated.
func CandidateLevDist(t T, i, n int) (float64, bool) {
	candidate := mustGetCandidate(t)
	if i == 0 {
		return float64(candidate.Distance), true
	}
	return float64(lev.Distance(t.Tokens[i], candidate.Suggestion)), true
}

// CandidateLevDist returns the weighted levenshtein distance between
// the OCR token and the token's connected profiler candidate.  For
// the master OCR the according Distance from the profiler candidate
// is used, whereas for support OCRs the levenshtein distance is
// calculated.
func CandidateWLevDist(t T, i, n int) (float64, bool) {
	m := lev.NewWMat(t.Document.LookupOCRPattern)
	candidate := mustGetCandidate(t)
	return m.Distance(t.Tokens[i], candidate.Suggestion), true
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

// mkCandidateMaxTrigramFreq returns a feature function that calculates
// the maximal trigram frequency for the connected candidate.
func mkCandidateMaxTrigramFreq(args []string) (FeatureFunc, error) {
	if len(args) != 1 {
		return lmFail("candidate max trigram freq", fmt.Errorf("bad arguments: %v", args))
	}
	lm := args[0]
	return func(t T, i, n int) (float64, bool) {
		if i != 0 {
			return 0, false
		}
		candidate := mustGetCandidate(t)
		max := 0.0
		t.Document.LM[lm].EachTrigram(candidate.Suggestion, func(conf float64) {
			if max < conf {
				max = conf
			}
		})
		return max, true
	}, nil
}

// mkCandidateMinTrigramFreq returns a feature function that calculates
// the minimal trigram frequency for the connected candidate.
func mkCandidateMinTrigramFreq(args []string) (FeatureFunc, error) {
	if len(args) != 1 {
		return lmFail("candidate min trigram freq", fmt.Errorf("bad arguments: %v", args))
	}
	lm := args[0]
	return func(t T, i, n int) (float64, bool) {
		if i != 0 {
			return 0, false
		}
		candidate := mustGetCandidate(t)
		min := 1.0
		t.Document.LM[lm].EachTrigram(candidate.Suggestion, func(conf float64) {
			if min > conf {
				min = conf
			}
		})
		return min, true
	}, nil
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
	if i != 0 {
		return 0, false
	}
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
	ret := CandidatesContainsLexiconEntry(cands)
	return ml.Bool(ret), true
}

func splitLen(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	ts := t.Payload.(Split).Tokens
	return float64(len(ts)), true
}

func countLexiconEntriesInMergedSplits(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	ts := t.Payload.(Split).Tokens
	number := 0.0
	for _, t := range ts {
		if t.ContainsLexiconEntry() {
			number++
		}
	}
	return number, true
}

func isSOL(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	return ml.Bool(t.SOL), true
}

func isEOL(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	return ml.Bool(t.EOL), true
}

func ffNumberOfCandidates(t T, i, n int) (float64, bool) {
	if i != 0 {
		return 0, false
	}
	cs := t.Document.Profile[t.Tokens[0]]
	return float64(len(cs.Candidates)), true
}
