package apoco

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/finkf/gofiler"
)

// FreqList is a simple frequenzy map.
type FreqList struct {
	FreqList map[string]int `json:"freqList"`
	Total    int            `json:"total"`
}

func (f *FreqList) init() {
	if f.FreqList == nil {
		f.FreqList = make(map[string]int)
	}
}

func (f *FreqList) add(strs ...string) {
	f.init()
	for _, str := range strs {
		f.Total++
		f.FreqList[str]++
	}
}

func (f *FreqList) absolute(str string) int {
	if f == nil {
		return 0
	}
	if n, ok := f.FreqList[str]; ok {
		return n
	}
	return 0
}

func (f *FreqList) relative(str string) float64 {
	if f == nil {
		return 0
	}
	if f.Total == 0 {
		return 0
	}
	abs := f.absolute(str)
	return float64(abs+1) / float64(f.Total+len(f.FreqList))
}

// EachTrigram calls the given callback function for each trigram in
// the given string.
func (f *FreqList) EachTrigram(str string, fn func(float64)) {
	EachTrigram(str, func(trg string) {
		fn(f.relative(trg))
	})
}

func EachTrigram(str string, fn func(string)) {
	runes := []rune("$" + str + "$")
	begin, end := 0, 3
	if end > len(runes) {
		end = len(runes)
	}
	for i, j := begin, end; j <= len(runes); i, j = i+1, j+1 {
		fn(string(runes[i:j]))
	}
}

// Document represents the token's document.
type Document struct {
	LM         map[string]*FreqList // Global language models.
	Unigrams   FreqList             // Document-wise unigram model.
	Profile    gofiler.Profile      // Document-wise profile.
	OCRPats    map[string]float64   // Error patterns (from the profiler).
	Group      string               // File group or directory of the document.
	Lexicality float64              // Lexicality score.
}

func (d *Document) LookupOCRPattern(a, b []rune) (float64, bool) {
	w, ok := d.OCRPats[string(a)+":"+string(b)]
	return 1 - w, ok
}

// AddUnigram adds the token to the language model's unigram map.
func (d *Document) AddUnigram(token string) {
	d.Unigrams.add(token)
}

// Unigram looks up the given token in the unigram list (or 0 if the
// unigram is not present).
func (d *Document) Unigram(str string) float64 {
	return d.Unigrams.relative(str)
}

// // Trigram looks up the trigrams of the given token and returns the
// // product of the token's trigrams.
// func (d *Document) Trigram(str string) float64 {
// 	tmp := []rune("$" + str + "$")
// 	begin, end := 0, 3
// 	if end > len(tmp) {
// 		end = len(tmp)
// 	}
// 	ret := 1.0
// 	for i, j := begin, end; j <= len(tmp); i, j = i+1, j+1 {
// 		ret *= d.LM["3grams"].relative(string(tmp[i:j]))
// 	}
// 	return ret
// }

// // TrigramLog looks up the trigrams of the given token and returns the
// // sum of the logarithmic relative frequency of the token's trigrams.
// func (d *Document) TrigramLog(str string) float64 {
// 	var sum float64
// 	d.EachTrigram(str, func(freq float64) {
// 		sum += math.Log(freq)
// 	})
// 	return sum
// }

// lengthOfWord gives the maximal length of words that the profiler
// accepts (see lengthOfWord in Global.h in the profiler's source).
const lengthOfWord = 64

// RunProfiler runs the profiler over the given tokens (using the
// token entries at index 0) with the given executable and config
// file.  The profiler's output is logged to stderr.
func RunProfiler(ctx context.Context, exe, config string, ts ...T) (gofiler.Profile, error) {
	var pts []gofiler.Token
	var adaptive bool
	for _, t := range ts {
		// Skip words that are too long for the profiler.  They are
		// only skipped for the input for the profiler not from the
		// general token stream.
		if len(t.Tokens[0]) >= lengthOfWord {
			continue
		}
		pts = append(pts, gofiler.Token{
			OCR: t.Tokens[0],
			COR: t.Cor,
		})
		adaptive = adaptive || t.Cor != ""
	}
	w, err := os.Create("profile.txt")
	if err != nil {
		return nil, err
	}
	defer w.Close()
	for _, t := range pts {
		fmt.Fprintln(w, t.String())
	}
	profiler := gofiler.Profiler{
		Exe:      exe,
		Config:   config,
		Types:    true,
		Adaptive: adaptive,
		Log:      logger{},
	}
	Log("len(pts) = %d", len(pts))
	profile, err := profiler.Run(ctx, pts)
	if err != nil {
		return nil, fmt.Errorf("run profiler %s %s: %v", exe, config, err)
	}
	return profile, nil
}

// ReadProfile reads the profile from a gzipped json formatted file.
func ReadProfile(name string) (gofiler.Profile, error) {
	in, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("read profile %s: %v", name, err)
	}
	defer in.Close()
	r, err := gzip.NewReader(in)
	if err != nil {
		return nil, fmt.Errorf("read profile %s: %v", name, err)
	}
	defer r.Close()
	var profile gofiler.Profile
	if err := json.NewDecoder(r).Decode(&profile); err != nil {
		return nil, fmt.Errorf("read profile %s: %v", name, err)
	}
	return profile, nil
}

// WriteProfile writes the profile as gzipped json formatted file.
func WriteProfile(name string, profile gofiler.Profile) error {
	w, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("write profile %s: %v", name, err)
	}
	defer w.Close()
	zip := gzip.NewWriter(w)
	defer zip.Close()
	if err := json.NewEncoder(zip).Encode(profile); err != nil {
		return fmt.Errorf("write profile %s: %v", name, err)
	}
	return nil
}

type logger struct {
}

func (logger) Log(str string) {
	const prefix = "[profiler] "
	if strings.Contains(str, "additional lexicon entries") {
		Log("%s %s", prefix, str)
	}
	if strings.Contains(str, "iteration") {
		Log("%s %s", prefix, str)
	}
	if strings.Contains(str, "cmd:") {
		Log("%s %s", prefix, str)
	}
}
