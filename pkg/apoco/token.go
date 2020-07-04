package apoco

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/finkf/gofiler"
)

// Token represent aligned OCR-tokens.
type Token struct {
	LM                          *LanguageModel // language model for this token
	Payload                     interface{}    // token payload; *gofiler.Candidate, []Ranking or Correction
	File                        string         // the file of the token
	ID                          string         // id of the token in this file
	FileGroup                   string         // file group of the token
	Chars                       Chars          // master OCR tokens with confidences
	Confs                       []float64      // master and support OCR confidences
	Tokens                      []string       // master and support OCRs and gt
	Lines                       []string       // lines of the tokens
	IsFirstInLine, IsLastInLine bool           // wether the token is the first or last in its line
}

// IsEmpty returns true iff the token is empty.  Empty tokens are
// used as sentries between documents in the streams.
func (t Token) IsEmpty() bool {
	return t.LM == nil &&
		t.File == "" &&
		t.ID == "" &&
		t.FileGroup == "" &&
		t.Chars == nil &&
		t.Confs == nil &&
		t.Tokens == nil &&
		t.Lines == nil
}

// IsLexiconEntry returns true if this token is a normal lexicon entry
// for its connected language model.
func (t Token) IsLexiconEntry() bool {
	if t.LM == nil {
		return false
	}
	interp, ok := t.LM.Profile[t.Tokens[0]]
	if !ok {
		return false
	}
	return len(interp.Candidates) == 1 &&
		len(interp.Candidates[0].OCRPatterns) == 0 &&
		len(interp.Candidates[0].HistPatterns) == 0
}

func (t Token) String() string {
	return strings.Join(t.Tokens, ",")
}

// Chars represents the master OCR chars with the respective confidences.
type Chars []Char

// PatternConf calculates the product of the pattern confidences for the
// matching right side of the given pattern.
func (chars Chars) PatternConf(p gofiler.Pattern) float64 {
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
	for _, r := range p.Right {
		_ = r
		if p.Pos+n >= len(chars) {
			break
		}
		sum += chars[p.Pos+n].Conf
		n++
	}
	return sum / float64(n)
}

func (chars Chars) String() string {
	strs := make([]string, len(chars))
	for i := range chars {
		strs[i] = chars[i].String()
	}
	return strings.Join(strs, ",")
}

// Char represents an OCR char with its confidence.
type Char struct {
	Conf float64 // confidence of the rune
	Char rune    // rune
}

func (char Char) String() string {
	return fmt.Sprintf("%c,%f", char.Char, char.Conf)
}

// Ranking maps correction candidates of tokens to their predicted
// probabilities.
type Ranking struct {
	Candidate *gofiler.Candidate
	Prob      float64
}

// Correction represents a correction decision for tokens.
type Correction struct {
	Candidate *gofiler.Candidate
	Conf      float64
}

// Correction applies the casing of the master OCR string to the
// correction's candidate suggestion and prepends and appends any
// punctuation of the master OCR to the suggestion.
func (cor Correction) Correction(masterOCR string) string {
	suggestion := []rune(cor.Candidate.Suggestion)
	pre, inf, suf := split(masterOCR)
	for i := 0; i < len(suggestion) && i < len(inf); i++ {
		if unicode.IsUpper(inf[i]) {
			suggestion[i] = unicode.ToUpper(suggestion[i])
		}
	}
	return string(pre) + string(suggestion) + string(suf)
}

func split(masterOCR string) (prefix, infix, suffix []rune) {
	word := []rune(masterOCR)
	var i, j int
	for i = 0; i < len(word); i++ {
		if !unicode.IsPunct(word[i]) {
			break
		}
	}
	for j = i; j < len(word); j++ {
		if unicode.IsPunct(word[j]) {
			break
		}
	}
	return word[0:i], word[i:j], word[j:]
}
