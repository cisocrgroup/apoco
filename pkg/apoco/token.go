package apoco

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/finkf/gofiler"
)

// Token represent aligned OCR-tokens.
type Token struct {
	LM      *LanguageModel // language model for this token
	Payload interface{}    // token payload; *gofiler.Candidate, []Ranking or Correction
	File    string         // the file of the token
	Group   string         // file group of the token
	ID      string         // id of the token in this file
	Chars   Chars          // master OCR tokens with confidences
	Confs   []float64      // master and support OCR confidences
	Tokens  []string       // master and support OCRs and gt
	Lines   []string       // lines of the tokens
	traits  TraitType      // token traits
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
	return fmt.Sprintf("%s,%s,%s", t.File, t.ID, strings.Join(t.Tokens, ","))
}

// TraitType is used to define different
// traits for the tokens.
type TraitType int64

// Trait flags
const (
	FirstInLine = 1 << iota
	LastInLine
	LowerCase
	UpperCase
	TitleCase
	MixedCase
)

// SetTrait sets a trait.
func (t *Token) SetTrait(i int, trait TraitType) {
	if trait < LowerCase {
		t.traits |= trait
		return
	}
	t.traits |= trait << (i * 4)
}

// HasTrait returns true if the token has the given trait.
func (t *Token) HasTrait(i int, trait TraitType) bool {
	if trait < LowerCase {
		return (t.traits & trait) > 0
	}
	return (t.traits & (trait << (i * 4))) > 0
}

// Chars represents the master OCR chars with the respective confidences.
type Chars []Char

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

// ApplyOCRToCorrection applies the casing of the master OCR string to the
// correction's candidate suggestion and prepends and appends any
// punctuation of the master OCR to the suggestion.
func ApplyOCRToCorrection(ocr, sug string) string {
	correction := []rune(sug)
	pre, inf, suf := split(ocr)
	for i := 0; i < len(correction) && i < len(inf); i++ {
		if unicode.IsUpper(inf[i]) {
			correction[i] = unicode.ToUpper(correction[i])
		}
	}
	return string(pre) + string(correction) + string(suf)
}

func split(masterOCR string) (prefix, infix, suffix []rune) {
	word := []rune(masterOCR)
	var i, j int
	for i = 0; i < len(word); i++ {
		if !unicode.IsPunct(word[i]) {
			break
		}
	}
	for j = len(word); j > 0; j-- {
		if !unicode.IsPunct(word[j-1]) {
			break
		}
	}
	return word[0:i], word[i:j], word[j:]
}
