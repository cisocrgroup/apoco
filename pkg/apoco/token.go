package apoco

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/finkf/gofiler"
)

// T represents aligned OCR-tokens.
type T struct {
	Document *Document   // Document this token belongs to.
	Payload  interface{} // Token payload; either *gofiler.Candidate, []Ranking, Correction or Split
	Cor      string      // Correction for the token
	File     string      // The file of the token
	ID       string      // ID of the token in its file
	Chars    Chars       // Master OCR chars including their confidences
	Tokens   []string    // Master and support OCRs and gt
	EOL, SOL bool        // End of line and start of line marker.
	IsSplit  bool        // Marks possible split tokens between the primary and secondary OCR.
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
	if len(interp.Candidates) != 1 {
		return false
	}
	return CandidateIsLexiconEntry(interp.Candidates[0])
}

// ContainsLexiconEntry returns true if any of the suggestions of the
// token are a lexicon entry.
func (t T) ContainsLexiconEntry() bool {
	if t.Document == nil {
		return false
	}
	interp, ok := t.Document.Profile[t.Tokens[0]]
	if !ok {
		return false
	}
	return CandidatesContainsLexiconEntry(interp.Candidates)
}

func (t T) GT() string {
	return t.Tokens[len(t.Tokens)-1]
}

func (t T) String() string {
	return strings.Join(t.Tokens, "|")
}

// CandidateIsLexiconEntry returns true if the given candidate
// represents a lexicon entry, i.e. it contains no OCR- and/or
// historic patterns.
func CandidateIsLexiconEntry(cand gofiler.Candidate) bool {
	return len(cand.OCRPatterns) == 0 && len(cand.HistPatterns) == 0
}

// CandidatesContainsLexiconEntry returns true if any of the given
// candidates contains a candidate that represents a lexicon entry.
func CandidatesContainsLexiconEntry(cands []gofiler.Candidate) bool {
	if len(cands) == 0 {
		return false
	}
	for _, cand := range cands {
		if CandidateIsLexiconEntry(cand) {
			return true
		}
	}
	return false
}

// Chars represents the master OCR chars with the respective
// confidences.
type Chars []Char

// Chars converts a char array to a string containing the chars.
func (chars Chars) Chars() string {
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

func (chars Chars) String() string {
	var sb strings.Builder
	for i := range chars {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%s", chars[i])
	}
	return sb.String()
}

// Char represents an OCR char with its confidence.
type Char struct {
	Conf float64 // confidence of the rune
	Char rune    // rune
}

func (char Char) String() string {
	return fmt.Sprintf("%c:%g", char.Char, char.Conf)
}

// Ranking maps correction candidates of tokens to their predicted
// probabilities.
type Ranking struct {
	Candidate *gofiler.Candidate
	Prob      float64
}

type Split struct {
	Candidates []gofiler.Candidate
	Tokens     []T
	Valid      bool
}

// Correction represents a correction decision for tokens.
type Correction struct {
	Candidate *gofiler.Candidate
	Conf      float64
}

// ApplyOCRToCorrection applies the casing of the master OCR string to
// the correction's candidate suggestion and prepends and appends any
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
