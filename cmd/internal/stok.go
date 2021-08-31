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
	OCRConfs                 []float64
	Conf                     float64
	Rank                     int
	Skipped, Short, Lex, Cor bool
}

func MakeStokFromT(t apoco.T, gt bool) Stok {
	ret := Stok{
		ID:       t.ID,
		OCR:      t.Tokens[0],
		Short:    utf8.RuneCountInString(t.Tokens[0]) < 4,
		OCRConfs: t.Chars.Confs(),
	}
	if gt {
		ret.GT = t.Tokens[len(t.Tokens)-1]
	}
	return ret
}

// MakeStok creates a new stats token from a according formatted line.
func MakeStok(line string) (Stok, error) {
	var stok Stok
	toks := strings.Split(line, " ")
	for _, tok := range toks {
		switch {
		case strings.HasPrefix(tok, "id="):
			if _, err := fmt.Sscanf(tok, "id=%s", &stok.ID); err != nil {
				return stok, fmt.Errorf("bad stats line %s: %v", line, err)
			}
		case strings.HasPrefix(tok, "skipped="):
			if _, err := fmt.Sscanf(tok, "skipped=%t", &stok.Skipped); err != nil {
				return stok, fmt.Errorf("bad stats line %s: %v", line, err)
			}
		case strings.HasPrefix(tok, "short="):
			if _, err := fmt.Sscanf(tok, "short=%t", &stok.Short); err != nil {
				return stok, fmt.Errorf("bad stats line %s: %v", line, err)
			}
		case strings.HasPrefix(tok, "lex="):
			if _, err := fmt.Sscanf(tok, "lex=%t", &stok.Lex); err != nil {
				return stok, fmt.Errorf("bad stats line %s: %v", line, err)
			}
		case strings.HasPrefix(tok, "cor="):
			if _, err := fmt.Sscanf(tok, "cor=%t", &stok.Cor); err != nil {
				return stok, fmt.Errorf("bad stats line %s: %v", line, err)
			}
		case strings.HasPrefix(tok, "ocrconfs="):
			var tmp ocrconfs
			if _, err := fmt.Sscanf(tok, "ocrconfs=%s", &tmp); err != nil {
				return stok, fmt.Errorf("bad stats line %s: %v", line, err)
			}
			stok.OCRConfs = tmp
		case strings.HasPrefix(tok, "conf="):
			if _, err := fmt.Sscanf(tok, "conf=%g", &stok.Conf); err != nil {
				return stok, fmt.Errorf("bad stats line %s: %v", line, err)
			}
		case strings.HasPrefix(tok, "rank="):
			if _, err := fmt.Sscanf(tok, "rank=%d", &stok.Rank); err != nil {
				return stok, fmt.Errorf("bad stats line %s: %v", line, err)
			}
		case strings.HasPrefix(tok, "ocr="):
			if _, err := fmt.Sscanf(tok, "ocr=%s", &stok.OCR); err != nil {
				return stok, fmt.Errorf("bad stats line %s: %v", line, err)
			}
		case strings.HasPrefix(tok, "sug="):
			if _, err := fmt.Sscanf(tok, "sug=%s", &stok.Sug); err != nil {
				return stok, fmt.Errorf("bad stats line %s: %v", line, err)
			}
		case strings.HasPrefix(tok, "gt="):
			if _, err := fmt.Sscanf(tok, "gt=%s", &stok.GT); err != nil {
				return stok, fmt.Errorf("bad stats line %s: %v", line, err)
			}
		case strings.HasPrefix(tok, "type="):
			// The command print types adds an additional type=... argument to
			// the stats IO.  Ignore this argument for reading stoks.
		default:
			return stok, fmt.Errorf("bad stats line: %s", line)
		}
	}
	return stok, nil
}

func (s Stok) String() string {
	return fmt.Sprintf("id=%s skipped=%t short=%t lex=%t cor=%t ocrconfs=%s conf=%g rank=%d ocr=%s sug=%s gt=%s",
		s.ID, s.Skipped, s.Short, s.Lex, s.Cor, ocrconfs(s.OCRConfs),
		s.Conf, s.Rank, E(s.OCR), E(s.Sug), E(s.GT))

}

type ocrconfs []float64

func (confs ocrconfs) String() string {
	if len(confs) == 0 {
		return Epsilon
	}
	var sb strings.Builder
	for i, conf := range confs {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%g", conf)
	}
	return sb.String()
}

func (confs *ocrconfs) Scan(state fmt.ScanState, verb rune) error {
	if verb != 's' {
		return fmt.Errorf("bad format verb: %c", verb)
	}
	bs, err := state.Token(false, func(c rune) bool {
		return c != ' '
	})
	if err != nil {
		return err
	}
	str := string(bs)
	if str == Epsilon {
		*confs = nil
		return nil
	}
	conftoks := strings.Split(string(bs), ",")
	for _, tok := range conftoks {
		var conf float64
		if _, err := fmt.Sscanf(tok, "%g", &conf); err != nil {
			return err
		}
		*confs = append(*confs, conf)
	}
	return nil
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

// Epsilon is used to mark empty strings and slices in the IO of stoks.
const Epsilon = "ε"

func E(str string) string {
	if str == "" {
		return Epsilon
	}
	return strings.ToLower(strings.Replace(str, " ", "_", -1))
}

// EachStok calls the given callback function f for each token read
// from r with the according name.  Stokens are read line by line
// from the reader, lines starting with # are skipped.  If a line starting
// with '#name=x' is encountered the name for the callback function is
// updated accordingly.
func EachStok(r io.Reader, f func(string, Stok) error) error {
	const namepref = "#name="
	s := bufio.NewScanner(r)
	var name string
	for s.Scan() {
		line := s.Text()
		if len(line) == 0 {
			continue
		}
		if strings.HasPrefix(line, namepref) {
			name = strings.Trim(line[len(namepref):], " \t\n")
			continue
		}
		if line[0] == '#' {
			continue
		}
		stok, err := MakeStok(line)
		if err != nil {
			return err
		}
		if err := f(name, stok); err != nil {
			return err
		}
		return nil
	}
	return s.Err()
}
