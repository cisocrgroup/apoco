package align

import (
	"unicode"

	"git.sr.ht/~flobar/lev"
)

// Pos represents the start and end position of an alignment.
type Pos struct {
	B, E int    // Start end end positions of the alignment slice.
	str  []rune // Reference string of the alignment.
}

// mkpos creates a new Pos instance with leading and subsequent
// whitespace removed.
func mkpos(b, e int, str []rune) Pos {
	b, e = strip(b, e, str)
	if e < b {
		e = b
	}
	return Pos{B: b, E: e, str: str}
}

// Slice returns the slice of base for the position.
func (p Pos) Slice() []rune {
	return p.str[p.B:p.E]
}

func (p Pos) String() string {
	return string(p.Slice())
}

// Do aligns the words in master pairwise with the words in other.
func Do(master []rune, other ...[]rune) [][]Pos {
	var spaces []int
	var words [][]Pos
	b := -1
	for i, j := strip(0, len(master), master); i < j; i++ {
		if unicode.IsSpace(master[i]) {
			spaces = append(spaces, i)
			words = append(words, []Pos{mkpos(b+1, i, master)})
			// Skip subsequent whitespace.
			for i+1 < len(master) && unicode.IsSpace(master[i+1]) {
				i++
			}
			b = i
		}
	}
	words = append(words, []Pos{mkpos(b+1, len(master), master)})
	for i := 0; i < len(other); i++ {
		b, e := strip(0, len(other[i]), other[i])
		alignments := alignAt(spaces, other[i][b:e])
		for j := range words {
			words[j] = append(words[j], alignments[j])
		}
	}
	return words
}

func alignAt(spaces []int, str []rune) []Pos {
	// If str is empty, each alignment is the empty string.  We
	// still need to return a slice with the right length.
	if len(str) == 0 {
		return make([]Pos, len(spaces)+1)
	}
	ret := make([]Pos, 0, len(spaces)+1)
	b := -1
	for _, s := range spaces {
		// log.Printf("space = %d", s)
		e := alignmentPos(str, s)
		// log.Printf("e = %d", e)
		// Var b points to the last found space.
		// Skip to the next non space token after b.
		b = skipSpace(str, b+1)
		// log.Printf("e <= b, %d <= %d", e, b)
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
	// log.Printf("alignmentPos(%s, %d)", string(str), pos)
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

func Lev(m *lev.Mat, primary []rune, rest ...[]rune) [][]Pos {
	primary = stripR(primary)
	var tokens [][]Pos
	b := -1
	for i, j := strip(0, len(primary), primary); i < j; i++ {
		if unicode.IsSpace(primary[i]) {
			tokens = append(tokens, []Pos{mkpos(b+1, i, primary)})
			// Skip subsequent whitespace.
			for i+1 < len(primary) && unicode.IsSpace(primary[i+1]) {
				i++
			}
			b = i
		}
	}
	tokens = append(tokens, []Pos{mkpos(b+1, len(primary), primary)})
	for _, r := range rest {
		r = stripR(r)
		as := alignPair(m, primary, r)
		if len(as) < len(tokens) {
			as = append(as, make([]Pos, len(tokens)-len(as))...)
		}
		for i := range tokens {
			tokens[i] = append(tokens[i], as[i])
		}
	}
	return tokens
}

func alignPair(m *lev.Mat, p, s []rune) []Pos {
	// log.Printf("alignPair(%s, %s)", string(p), string(s))
	m.DistanceR(p, s)
	trace := m.TraceR(p, s)

	var pos []Pos
	var pi, si /*, pb*/, sb int
	// x, zz := lev.AlignTrace(string(p), string(s), trace)
	// log.Printf("align: %s", string(x))
	// log.Printf("align: %s", string(trace))
	// log.Printf("align: %s", string(zz))
	for i := range trace {
		if pi >= len(p) || si >= len(s) {
			break
		}
		if unicode.IsSpace(p[pi]) {
			end := y(s, si)
			// log.Printf("%d:%d %d:%d", pb, pi, sb, end)
			// log.Printf("%s %s", string(p[pb:pi]), string(s[sb:end]))
			pos = append(pos, mkpos(sb, end, s))
			// pb = z(p, pi)
			// pb = pi + 1
			if end <= pi {
				sb = z(s, end)
			}
		}
		switch trace[i] {
		case '#':
			pi++
			si++
		case '+':
			si++
		case '-':
			pi++
		}
	}
	// end := y(s, si)
	pos = append(pos, mkpos(sb, len(s), s))
	// log.Printf("%s %s", string(p[pb:pi]), string(s[sb:]))
	return pos
}

func y(str []rune, p int) int {
	for i := p; i < len(str); i++ {
		if unicode.IsSpace(str[i]) {
			return i
		}
	}
	return len(str)
}

func z(str []rune, p int) int {
	for ; p < len(str) && unicode.IsSpace(str[p]); p++ {
	}
	return p
}

func stripR(str []rune) []rune {
	b, e := strip(0, len(str), str)
	return str[b:e]
}
