package internal

import "fmt"

const stokFormat = "skipped=%t short=%t lex=%t cor=%t rank=%d ocr=%s sug=%s gt=%s"

// Stok represents a stats token. Stat tokens explain correction
// decisions of apoco.
type Stok struct {
	OCR, Sug, GT             string
	Rank                     int
	Skipped, Short, Lex, Cor bool
}

// NewStok creates a new stats token from a according formatted line.
func NewStok(line string) (Stok, error) {
	var s Stok
	_, err := fmt.Sscanf(line, stokFormat,
		&s.Skipped, &s.Short, &s.Lex, &s.Cor,
		&s.Rank, &s.OCR, &s.Sug, &s.GT)
	if err != nil {
		return Stok{}, fmt.Errorf("bad stats line %s: %v", line, err)
	}
	return s, nil
}

func (s Stok) String() string {
	return fmt.Sprintf(stokFormat, s.Skipped, s.Short, s.Lex, s.Cor, s.Rank, s.OCR, s.Sug, s.GT)
}
