package internal

import (
	"fmt"
	"strings"
)

const stokFormat = "id=%s skipped=%t short=%t lex=%t cor=%t conf=%g rank=%d ocr=%s sug=%s gt=%s"

// Stok represents a stats token. Stat tokens explain correction
// decisions of apoco.
type Stok struct {
	OCR, Sug, GT, ID         string
	Conf                     float64
	Rank                     int
	Skipped, Short, Lex, Cor bool
}

// MakeStok creates a new stats token from a according formatted line.
func MakeStok(line string) (Stok, error) {
	var s Stok
	_, err := fmt.Sscanf(line, stokFormat,
		&s.ID, &s.Skipped, &s.Short, &s.Lex, &s.Cor,
		&s.Conf, &s.Rank, &s.OCR, &s.Sug, &s.GT)
	if err != nil {
		return Stok{}, fmt.Errorf("bad stats line %s: %v", line, err)
	}
	return s, nil
}

func (s Stok) String() string {
	return fmt.Sprintf(stokFormat,
		E(s.ID), s.Skipped, s.Short, s.Lex, s.Cor,
		s.Conf, s.Rank, E(s.OCR), E(s.Sug), E(s.GT))
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
			return SuspiciousReplacedCorrect
		}
		return SuspiciousReplacedCorrectErr
	}
	if s.Cor && s.GT != s.OCR {
		if s.Sug == s.GT {
			return SuspiciousReplacedNotCorrect
		}
		return SuspiciousReplacedNotCorrectErr
	}
	if !s.Cor && s.GT == s.OCR {
		if s.Sug == s.GT {
			return SuspiciousNotReplacedCorrect
		}
		return SuspiciousNotReplacedCorrectErr
	}
	if !s.Cor && s.GT != s.OCR {
		if s.Sug == s.GT {
			return SuspiciousNotReplacedNotCorrect
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
		return MissingCorrection
	case limit > 0 && limit < s.Rank:
		return BadLimit
	default:
		return BadRank
	}
}

// StokType gives the type of stoks.
type StokType int

const (
	SkippedShort                       StokType = iota // Skipped short token.
	SkippedShortErr                                    // Error in short token.
	SkippedNoCand                                      // Skipped no canidate token.
	SkippedNoCandErr                                   // Error in skipped no candidate token.
	SkippedLex                                         // Skipped lexical token.
	SkippedLexErr                                      // Error in skipped lexical token (false friend).
	SuspiciousReplacedCorrect                          // Redundant correction.
	SuspiciousReplacedCorrectErr                       // Infelicitous correction.
	SuspiciousReplacedNotCorrect                       // Successful correction.
	SuspiciousReplacedNotCorrectErr                    // Do not care correction.
	SuspiciousNotReplacedCorrect                       // Accept OCR.
	SuspiciousNotReplacedCorrectErr                    // Dogded bullet.
	SuspiciousNotReplacedNotCorrect                    // Missed opportunity.
	SuspiciousNotReplacedNotCorrectErr                 // Skipped do not care.
)

// IsSkipped returns true if the stok type marks a skipped tokens.
func (s StokType) Skipped() bool {
	return s <= SkippedLexErr
}

// Err returns true if the stok type marks an Error.
func (s StokType) Err() bool {
	return s%2 == 1 // Odd stok values are errors.
}

// StokCause gives the cause of errors.
type StokCause int

const (
	BadRank           StokCause = iota // Bad correction because of a bad rank.
	BadLimit                           // Bad correction because of a bad limit for the correction candidates.
	MissingCorrection                  // Bad correction because of a missing correct correction candidate.
)

func E(str string) string {
	if str == "" {
		return "ε"
	}
	return strings.ToLower(strings.Replace(str, " ", "_", -1))
}
