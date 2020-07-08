package apoco

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"github.com/finkf/gofiler"
)

// FreqList is a simple frequenzy map.
type FreqList struct {
	FreqList map[string]int `json:"freqList"`
	Total    int            `json:"total"`
}

func (f *FreqList) init() {
	if f.FreqList != nil {
		return
	}
	f.FreqList = make(map[string]int)
}

func (f *FreqList) add(strs ...string) {
	f.init()
	for _, str := range strs {
		f.Total++
		f.FreqList[str]++
	}
}

func (f *FreqList) total() int {
	return f.Total
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
	if abs == 0 {
		return 1.0 / float64(f.Total)
	}
	return float64(abs) / float64(f.Total)
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

// LanguageModel consists of holds the language model for tokens.
type LanguageModel struct {
	ngrams   FreqList
	unigrams FreqList
	Profile  gofiler.Profile
}

// AddUnigram adds the token to the language model's unigram map.
func (lm *LanguageModel) AddUnigram(token string) {
	lm.unigrams.add(token)
}

// Unigram looks up the given token in the unigram list (or 0 if the
// unigram is not present).
func (lm *LanguageModel) Unigram(str string) float64 {
	return lm.unigrams.relative(str)
}

// Trigram looks up the trigrams of the given token and returns the
// product of the token's trigrams.
func (lm *LanguageModel) Trigram(str string) float64 {
	tmp := []rune("$" + str + "$")
	begin, end := 0, 3
	if end > len(tmp) {
		end = len(tmp)
	}
	ret := 1.0
	for i, j := begin, end; j <= len(tmp); i, j = i+1, j+1 {
		ret *= lm.ngrams.relative(string(tmp[i:j]))
	}
	return ret
}

// EachTrigram looks up the trigrams of the given token and returns the
// product of the token's trigrams.
func (lm *LanguageModel) EachTrigram(str string, f func(float64)) {
	tmp := []rune("$" + str + "$")
	begin, end := 0, 3
	if end > len(tmp) {
		end = len(tmp)
	}
	for i, j := begin, end; j <= len(tmp); i, j = i+1, j+1 {
		f(lm.ngrams.relative(string(tmp[i:j])))
	}
}

// LoadGzippedNGram loads the (gzipped) ngram model file.  The expected format
// for each line is `%d,%s`.
func (lm *LanguageModel) LoadGzippedNGram(path string) error {
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
	if err := lm.ngrams.loadCSV(gz); err != nil {
		return fmt.Errorf("load ngrams: %s: %v", path, err)
	}
	return nil
}

// LoadProfile loads the profile for the master OCR tokens.
func (lm *LanguageModel) LoadProfile(ctx context.Context, exe, config string, nocache bool, tokens ...Token) error {
	if len(tokens) == 0 {
		return nil
	}
	if !nocache {
		if profile, ok := readCachedProfile(tokens[0].Group); ok {
			lm.Profile = profile
			return nil
		}
	}
	var profilerTokens []gofiler.Token
	for _, token := range tokens {
		profilerTokens = append(profilerTokens, gofiler.Token{OCR: token.Tokens[0]})
	}
	profiler := gofiler.Profiler{Exe: exe, Types: true, Log: logger{}}
	profile, err := profiler.Run(ctx, config, profilerTokens)
	if err != nil {
		return fmt.Errorf("load profile: %v", err)
	}
	lm.Profile = profile
	if nocache {
		return nil
	}
	go func() { cacheProfile(tokens[0].Group, profile) }()
	return nil
}

func cachePath(fg string) (string, bool) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", false
	}
	dir := url.PathEscape(fg)
	return filepath.Join(cacheDir, "apoco", dir, "profile.json"), true
}

func readCachedProfile(fg string) (gofiler.Profile, bool) {
	path, ok := cachePath(fg)
	if !ok {
		return nil, false
	}
	is, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer is.Close()
	gz, err := gzip.NewReader(is)
	if err != nil {
		return nil, false
	}
	defer gz.Close()
	var profile gofiler.Profile
	if err := json.NewDecoder(gz).Decode(&profile); err != nil {
		return nil, false
	}
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
	out, err := os.Create(path)
	if err != nil {
		return
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	defer gz.Close()
	_ = json.NewEncoder(gz).Encode(profile)
}

type logger struct {
}

func (logger) Log(str string) {
	log.Print(str)
}
