package align

import (
	"unicode"
)

// Pos represents the start and end position of an alignment.
type Pos struct {
	B, E int
	str  []rune
}

// mkpos creates a new Pos instance with leading and subsequent
// whitespace removed.
func mkpos(b, e int, str []rune) Pos {
	b, e = strip(b, e, str)
	return Pos{B: b, E: e, str: str}
}

// Slice returns the slice of base for the position.
func (p Pos) Slice() []rune {
	return p.str[p.B:p.E]
}

// Do aligns the words in master pairwise with the words in other.
func Do(master []rune, other ...[]rune) [][]Pos {
	var spaces []int
	var words [][]Pos
	b := -1
	for i, j := strip(0, len(master), master); i < j; i++ {
		if unicode.IsSpace(master[i]) {
			spaces = append(spaces, i)
			words = append(words, []Pos{mkpos(b+1, i, master)}) //{B: b + 1, E: i, str: master}})
			// Skip subsequent whitespace.
			for i+1 < len(master) && unicode.IsSpace(master[i+1]) {
				i++
			}
			b = i
		}
	}
	words = append(words, []Pos{mkpos(b+1, len(master), master)}) //{B: b + 1, E: len(master), str: master}})
	for i := 0; i < len(other); i++ {
		alignments := alignAt(spaces, other[i])
		for j := range words {
			words[j] = append(words[j], alignments[j])
		}
	}
	return words
}

func alignAt(spaces []int, str []rune) []Pos {
	// If str is empty, each alignment is the empty string.  We
	// still need to return a slice with the right lenght.
	if len(str) == 0 {
		return make([]Pos, len(spaces)+1)
	}
	ret := make([]Pos, 0, len(spaces)+1)
	b := -1
	for _, s := range spaces {
		e := alignmentPos(str, s)
		// Var b points to the last found space.
		// Skip to the next non space token after b.
		b = skipSpace(str, b+1)
		if e <= b { // (e <= b) -> (b>=0) -> len(ret) > 0
			b = ret[len(ret)-1].B
		}
		ret = append(ret, mkpos(b, e, str))
		b = e
	}
	if len(str) <= b { // see above
		ret = append(ret, mkpos(ret[len(ret)-1].B, len(str), str))
	} else {
		ret = append(ret, mkpos(b+1, len(str), str))
	}
	return ret
}

func alignmentPos(str []rune, pos int) int {
	if pos >= len(str) {
		return len(str)
	}
	if str[pos] == ' ' {
		return pos
	}
	for i := 1; ; i++ {
		if pos+i >= len(str) && i >= pos {
			return len(str)
		}
		if pos+i < len(str) && str[pos+i] == ' ' {
			return pos + i
		}
		if i <= pos && str[pos-i] == ' ' {
			return pos - i
		}
	}
}

func skipSpace(str []rune, pos int) int {
	for pos < len(str) && unicode.IsSpace(str[pos]) {
		pos++
	}
	return pos
}

func strip(b, e int, str []rune) (int, int) {
	for b < len(str) && unicode.IsSpace(str[b]) {
		b++
	}
	for e > b && unicode.IsSpace(str[e-1]) {
		e--
	}
	return b, e
}
