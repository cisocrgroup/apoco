package apoco

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/finkf/gofiler"
)

// T represents aligned OCR-tokens.
type T struct {
	Document *Document   // Document of this token
	Payload  interface{} // Token payload; either *gofiler.Candidate or []Ranking or Correction
	Cor      string      // Correction for the token
	File     string      // The file of the token
	ID       string      // ID of the token in its file
	Chars    Chars       // Master OCR chars including their confidences
	Tokens   []string    // Master and support OCRs and gt
}

// IsLexiconEntry returns true if this token is a normal lexicon entry
// for its connected language model.
func (t T) IsLexiconEntry() bool {
	if t.Document == nil {
		return false
	}
	interp, ok := t.Document.Profile[t.Tokens[0]]
	if !ok {
		return false
	}
	return len(interp.Candidates) == 1 &&
		len(interp.Candidates[0].OCRPatterns) == 0 &&
		len(interp.Candidates[0].HistPatterns) == 0
}

func (t T) String() string {
	return strings.Join(t.Tokens, "|")
}

// Chars represents the master OCR chars with the respective confidences.
type Chars []Char

// String converts a char array to a string.
func (chars Chars) String() string {
	var sb strings.Builder
	for i := range chars {
		sb.WriteRune(chars[i].Char)
	}
	return sb.String()
}

// Confs returns the confidences as array.
func (chars Chars) Confs() []float64 {
	confs := make([]float64, len(chars))
	for i := range chars {
		confs[i] = chars[i].Conf
	}
	return confs
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
