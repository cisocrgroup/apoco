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

func E(str string) string {
	if str == "" {
		return "ε"
	}
	return strings.ToLower(strings.Replace(str, " ", "_", -1))
}
