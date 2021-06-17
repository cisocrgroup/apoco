package internal

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
)

// Stok represents a stats token. Stat tokens explain
// the correction decisions of apoco and form the basis
// of the correction protocols.
type Stok struct {
	OCR, Sug, GT, ID         string
	Conf, OCRConf            float64
	Rank                     int
	Skipped, Short, Lex, Cor bool
}

func MakeStokFromT(t apoco.T, gt bool) Stok {
	ret := Stok{
		ID:      t.ID,
		OCR:     t.Tokens[0],
		Short:   utf8.RuneCountInString(t.Tokens[0]) < 4,
		OCRConf: avrg(t.Chars),
	}
	if gt {
		ret.GT = t.Tokens[len(t.Tokens)-1]
	}
	return ret
}

func avrg(chars apoco.Chars) float64 {
	if len(chars) == 0 {
		return 0
	}
	sum := 0.0
	for _, char := range chars {
		sum += char.Conf
	}
	return sum / float64(len(chars))
}

const stokFormat = "id=%s skipped=%t short=%t lex=%t cor=%t ocrconf=%g conf=%g rank=%d ocr=%s sug=%s gt=%s"

// MakeStok creates a new stats token from a according formatted line.
func MakeStok(line string) (Stok, error) {
	var s Stok
	_, err := fmt.Sscanf(line, stokFormat,
		&s.ID, &s.Skipped, &s.Short, &s.Lex, &s.Cor,
		&s.OCRConf, &s.Conf, &s.Rank, &s.OCR, &s.Sug, &s.GT)
	if err != nil {
		return Stok{}, fmt.Errorf("bad stats line: %s", line)
	}
	return s, nil
}

func (s Stok) String() string {
	return fmt.Sprintf(stokFormat,
		s.ID, s.Skipped, s.Short, s.Lex, s.Cor,
		s.OCRConf, s.Conf, s.Rank, E(s.OCR), E(s.Sug), E(s.GT))
}

// Type returns the correction type of the stok.
func (s Stok) Type() StokType {
	if s.Skipped {
		if s.Short {
			return SkippedShort + s.skippedErrOffset()
		}
		if s.Lex {
			return SkippedLex + s.skippedErrOffset()
		}
		return SkippedNoCand + s.skippedErrOffset()
	}
	if s.Cor && s.GT == s.OCR {
		if s.Sug == s.GT {
			return RedundantCorrection
		}
		return InfelicitousCorrection
	}
	if s.Cor && s.GT != s.OCR {
		if s.Sug == s.GT {
			return SuccessfulCorrection
		}
		return DoNotCareCorrection
	}
	if !s.Cor && s.GT == s.OCR {
		if s.Sug == s.GT {
			return SuspiciousNotReplacedCorrect
		}
		return DodgedBullet
	}
	if !s.Cor && s.GT != s.OCR {
		if s.Sug == s.GT {
			return MissedOpportunity
		}
		return SuspiciousNotReplacedNotCorrectErr
	}
	panic("invalid stok type")
}

func (s Stok) skippedErrOffset() StokType {
	if !s.Skipped {
		panic("call to skippedErrOffset on not skipped token")
	}
	if s.GT != s.OCR {
		return StokType(1)
	}
	return StokType(0)
}

// Cause returns the cause of a correction error.  There are 3 possibilities.
// Either the correction candidate was missing, the correct correction
// candidate was not selected by the reranker or the correct correction
// canidate would have been available but could not be selected because of the
// imposed limit of the number of correction candidates.  If the limit smaller
// or equal to 0, no limit is imposed.
func (s Stok) Cause(limit int) StokCause {
	switch {
	case s.Rank == 0:
		return MissingCandidate
	case limit > 0 && limit < s.Rank:
		return BadLimit
	default:
		return BadRank
	}
}

// Merge returns true if the token contains merged OCR-tokens.
func (s Stok) Merge() bool {
	return strings.Contains(s.GT, "_")
}

//
func (s Stok) Split(before Stok) bool {
	return s.GT != s.OCR && s.GT == before.GT
}

func (s Stok) ErrBefore() bool {
	return s.OCR != s.GT
}

func (s Stok) ErrAfter() bool {
	if (s.Skipped && s.OCR != s.GT) || // errors in skipped tokens
		(!s.Skipped && s.Cor && s.Sug != s.GT) || // infelicitous correction
		(!s.Skipped && !s.Cor && s.OCR != s.GT) { // not corrected and false
		return true
	}
	return false
}

// StokType gives the type of stoks.
type StokType int

const (
	SkippedShort                       StokType = iota // Skipped short token.
	SkippedShortErr                                    // Error in short token.
	SkippedNoCand                                      // Skipped no canidate token.
	SkippedNoCandErr                                   // Error in skipped no candidate token.
	SkippedLex                                         // Skipped lexical token.
	FalseFriend                                        // Error in skipped lexical token (false friend).
	RedundantCorrection                                // Redundant correction.
	InfelicitousCorrection                             // Infelicitous correction.
	SuccessfulCorrection                               // Successful correction.
	DoNotCareCorrection                                // Do not care correction.
	SuspiciousNotReplacedCorrect                       // Accept OCR.
	DodgedBullet                                       // Dogded bullet.
	MissedOpportunity                                  // Missed opportunity.
	SuspiciousNotReplacedNotCorrectErr                 // Skipped do not care.
)

// IsSkipped returns true if the stok type marks a skipped tokens.
func (s StokType) Skipped() bool {
	return s <= FalseFriend
}

// Err returns true if the stok type marks an Error.
func (s StokType) Err() bool {
	return s%2 == 1 // Odd stok values are errors.
}

// StokCause gives the cause of errors.
type StokCause int

const (
	BadRank          StokCause = iota // Bad correction because of a bad rank.
	BadLimit                          // Bad correction because of a bad limit for the correction candidates.
	MissingCandidate                  // Bad correction because of a missing correct correction candidate.
)

func E(str string) string {
	if str == "" {
		return "ε"
	}
	return strings.ToLower(strings.Replace(str, " ", "_", -1))
}

// EachStok calls the given callback function f for each token read
// from r with the according name.  Stokens are read line by line
// from the reader, lines starting with # are skipped.  If a line starting
// with '#name=x' is encountered the name for the callback function is
// updated accordingly.
func EachStok(r io.Reader, f func(string, Stok)) error {
	s := bufio.NewScanner(r)
	var name string
	for s.Scan() {
		line := s.Text()
		if len(line) == 0 {
			continue
		}
		if strings.HasPrefix(line, "#name=") {
			name = strings.Trim(line[6:], " \t\n")
			continue
		}
		if line[0] == '#' {
			continue
		}
		stok, err := MakeStok(line)
		if err != nil {
			return err
		}
		f(name, stok)
	}
	return s.Err()
}
