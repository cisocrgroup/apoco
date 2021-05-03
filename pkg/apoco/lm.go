package apoco

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
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
	if n, ok := f.FreqList[str]; ok {
		return n
	}
	return 0
}

func (f *FreqList) relative(str string) float64 {
	if f.Total == 0 {
		return 0
	}
	abs := f.absolute(str)
	return float64(abs+1) / float64(f.Total+len(f.FreqList))
}

func (f *FreqList) loadCSV(in io.Reader) error {
	f.init()
	s := bufio.NewScanner(in)
	for s.Scan() {
		var n int
		var str string
		if _, err := fmt.Sscanf(s.Text(), "%d,%s", &n, &str); err != nil {
			return fmt.Errorf("loadCSV: %v", err)
		}
		f.Total += n
		f.FreqList[str] += n
	}
	if err := s.Err(); err != nil {
		return fmt.Errorf("loadCSV: %v", err)
	}
	return nil
}

// Document represents the token's document.
type Document struct {
	LM         *FreqList       // Global ngram model
	Unigrams   FreqList        // Document-wise unigram model
	Profile    gofiler.Profile // Document-wise profile
	Lexicality float64         // Lexicality score
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

// Trigram looks up the trigrams of the given token and returns the
// product of the token's trigrams.
func (d *Document) Trigram(str string) float64 {
	tmp := []rune("$" + str + "$")
	begin, end := 0, 3
	if end > len(tmp) {
		end = len(tmp)
	}
	ret := 1.0
	for i, j := begin, end; j <= len(tmp); i, j = i+1, j+1 {
		ret *= d.LM.relative(string(tmp[i:j]))
	}
	return ret
}

// TrigramLog looks up the trigrams of the given token and returns the
// sum of the logarithmic relative frequency of the token's trigrams.
func (d *Document) TrigramLog(str string) float64 {
	var sum float64
	d.EachTrigram(str, func(freq float64) {
		sum += math.Log(freq)
	})
	return sum
}

// EachTrigram calls the given callback function for each trigram in
// the given string.
func EachTrigram(str string, f func(string)) {
	runes := []rune("$" + str + "$")
	begin, end := 0, 3
	if end > len(runes) {
		end = len(runes)
	}
	for i, j := begin, end; j <= len(runes); i, j = i+1, j+1 {
		f(string(runes[i:j]))
	}
}

// EachTrigram looks up the trigrams of the given token and returns the
// product of the token's trigrams.
func (d *Document) EachTrigram(str string, f func(float64)) {
	EachTrigram(str, func(trigram string) {
		f(d.LM.relative(trigram))
	})
}

// LoadGzippedNGram loads the (gzipped) ngram model file.  The expected format
// for each line is `%d,%s`.
func (d *Document) LoadGzippedNGram(path string) error {
	is, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("load ngrams %s: %v", path, err)
	}
	defer is.Close()
	gz, err := gzip.NewReader(is)
	if err != nil {
		return fmt.Errorf("load ngrams %s: %v", path, err)
	}
	defer gz.Close()
	if err := d.LM.loadCSV(gz); err != nil {
		return fmt.Errorf("load ngrams: %s: %v", path, err)
	}
	return nil
}

func (d *Document) calculateLexicality(tokens ...T) {
	var total, lexical int
	for _, token := range tokens {
		total++
		interpretation, ok := d.Profile[token.Tokens[0]]
		if !ok || len(interpretation.Candidates) == 0 {
			continue
		}
		if interpretation.Candidates[0].Distance == 0 {
			lexical++
		}
	}
	d.Lexicality = float64(lexical) / float64(total)
}

// ReadProfile loads the profile for the master OCR tokens.
func (d *Document) ReadProfile(ctx context.Context, exe, config string, cache bool, tokens ...T) error {
	if len(tokens) == 0 {
		return nil
	}
	if cache {
		if profile, ok := readCachedProfile(tokens[0].Group); ok {
			d.Profile = profile
			return nil
		}
	}
	profile, err := RunProfiler(ctx, exe, config, tokens...)
	if err != nil {
		return fmt.Errorf("read profile %s %s: %v", exe, config, err)
	}
	d.Profile = profile
	if !cache {
		return nil
	}
	cacheProfile(tokens[0].Group, profile)
	d.calculateLexicality(tokens...)
	return nil
}

// lengthOfWord gives the maximal length of words that the profiler
// accepts (see lengthOfWord in Global.h in the profiler's source).
const lengthOfWord = 64

// RunProfiler runs the profiler over the given tokens (using the
// token entries at index 0) with the given executable and config
// file.  The profiler's output is logged to stderr.
func RunProfiler(ctx context.Context, exe, config string, tokens ...T) (gofiler.Profile, error) {
	var profilerTokens []gofiler.Token
	var adaptive bool
	for _, token := range tokens {
		// Skip words that are too long for the profiler.  They are
		// only skipped for the input for the profiler not from the
		// general token stream.
		if len(token.Tokens[0]) > lengthOfWord {
			continue
		}
		profilerTokens = append(profilerTokens, gofiler.Token{
			OCR: token.Tokens[0],
			COR: token.Cor,
		})
		adaptive = adaptive || token.Cor != ""
	}
	profiler := gofiler.Profiler{Exe: exe, Types: true, Adaptive: adaptive, Log: logger{}}
	profile, err := profiler.Run(ctx, config, profilerTokens)
	if err != nil {
		return nil, fmt.Errorf("run profiler %s %s: %v", exe, config, err)
	}
	return profile, nil
}

func cachePath(dir string) (string, bool) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", false
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	name := strings.ReplaceAll(abs, "/", "-")[1:]
	return filepath.Join(cacheDir, "apoco", name+".json.gz"), true
}

func readCachedProfile(fg string) (gofiler.Profile, bool) {
	path, ok := cachePath(fg)
	if !ok {
		return nil, false
	}
	Log("reading cached profile from %s", path)
	profile, err := ReadProfile(path)
	if err != nil {
		return nil, false
	}
	Log("read %d profile tokens from %s", len(profile), path)
	return profile, true
}

func cacheProfile(fg string, profile gofiler.Profile) {
	path, ok := cachePath(fg)
	if !ok {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return
	}
	if err := WriteProfile(path, profile); err != nil {
		return
	}
	Log("cached %d profile tokens to %s", len(profile), path)
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
	out, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("write profile %s: %v", name, err)
	}
	defer out.Close()
	w := gzip.NewWriter(out)
	defer w.Close()
	if err := json.NewEncoder(w).Encode(profile); err != nil {
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
