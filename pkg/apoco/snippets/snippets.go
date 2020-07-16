package snippets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"golang.org/x/sync/errgroup"
)

// Extensions is used to tokenize snippets in directories using the
// list of file extensions.
type Extensions []string

// Tokenize tokenizes tokens from line snippets TSV files (identyfied
// by the given file extensions) and alignes them accordingly.  If a
// extension ends with `.txt`, one line is read from the text file (no
// confidences).  Otherwise the file is read as a TSV file expecting
// on char and its confidence on each line.
func (e Extensions) Tokenize(dirs ...string) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, _ <-chan apoco.Token) <-chan apoco.Token {
		out := make(chan apoco.Token)
		g.Go(func() error {
			defer close(out)
			// Iterate over the directories and read the tokens from each dir.
			for _, dir := range dirs {
				if err := e.readTokensFromDir(ctx, out, dir); err != nil {
					return fmt.Errorf("tokenize: %v", err)
				}
			}
			return nil
		})
		return out
	}
}

func (e Extensions) readTokensFromDir(ctx context.Context, out chan<- apoco.Token, bdir string) error {
	if len(e) == 0 {
		return fmt.Errorf("readTokensFromDir %s: empty file extensions", bdir)
	}
	// Use a dir path stack to iterate over all dirs in the tree.
	dirs := []string{bdir}
	for len(dirs) != 0 {
		dir := dirs[len(dirs)-1]
		dirs = dirs[0 : len(dirs)-1]
		// Read all file info entries from the dir.
		is, err := os.Open(dir)
		if err != nil {
			return fmt.Errorf("readTokensFromDir %s: %v", bdir, err)
		}
		fis, err := is.Readdir(-1)
		is.Close() // Unconditionally close the dir.
		if err != nil {
			return fmt.Errorf("readTokensFromDir %s: %v", bdir, err)
		}
		// Either append new dirs to the stack or handle files with
		// the master file extension at index 0. Skip all other files.
		for _, fi := range fis {
			if fi.IsDir() {
				dirs = append(dirs, filepath.Join(dir, fi.Name()))
				continue
			}
			if !strings.HasSuffix(fi.Name(), e[0]) {
				continue
			}
			file := filepath.Join(dir, fi.Name())
			if err := e.readTokensFromSnippets(ctx, out, bdir, file); err != nil {
				return fmt.Errorf("readTokensFromDir %s: %v", bdir, err)
			}
		}
	}
	return nil
}

func (e Extensions) readTokensFromSnippets(ctx context.Context, out chan<- apoco.Token, bdir, file string) error {
	var lines []apoco.Chars
	pairs, err := readFile(file)
	if err != nil {
		return fmt.Errorf("readTokensFromSnippets: %v", err)
	}
	lines = append(lines, pairs)
	for i := 1; i < len(e); i++ {
		path := file[0:len(file)-len(e[0])] + e[i]
		pairs, err := readFile(path)
		if err != nil {
			return fmt.Errorf("readTokensFromSnippets: %v", err)
		}
		lines = append(lines, pairs)
	}
	if err := sendTokens(ctx, out, bdir, file, lines); err != nil {
		return fmt.Errorf("readTokensFromSnippets: %v", err)
	}
	return nil
}

func sendTokens(ctx context.Context, out chan<- apoco.Token, bdir, file string, lines []apoco.Chars) error {
	alignments := align(lines...)
	for i := range alignments {
		t := apoco.Token{
			File:  file,
			Group: bdir,
			ID:    strconv.Itoa(i + 1),
		}
		for j, p := range alignments[i] {
			if j == 0 {
				t.Chars = lines[j][p.b:p.e]
			}
			t.Tokens = append(t.Tokens, string(runes(lines[j][p.b:p.e])))
		}
		if err := apoco.SendTokens(ctx, out, t); err != nil {
			return fmt.Errorf("sendTokens: %v", err)
		}
	}
	return nil
}

func align(lines ...apoco.Chars) [][]pos {
	var spaces []int
	var words [][]pos
	b := -1
	for i := range lines[0] {
		if unicode.IsSpace(lines[0][i].Char) {
			spaces = append(spaces, i)
			words = append(words, []pos{{b: b + 1, e: i}})
			b = i
		}
	}
	words = append(words, []pos{{b: b + 1, e: len(lines[0])}})
	for i := 1; i < len(lines); i++ {
		alignments := alignAt(spaces, runes(lines[i]))
		for j := range words {
			words[j] = append(words[j], alignments[j])
		}
	}
	return words
}

func alignAt(spaces []int, str []rune) []pos {
	ret := make([]pos, 0, len(spaces)+1)
	b := -1
	for _, s := range spaces {
		e := alignmentPos(str, s)
		// Var b points to the last found space.
		// Skip to the next non space token after b.
		b = skipSpace(str, b+1)
		if e <= b { // (e <= b) -> (b>=0) -> len(ret) > 0
			b = ret[len(ret)-1].b
		}
		ret = append(ret, pos{b: b, e: e})
		if e != len(str) {
			b = e
		}
	}
	ret = append(ret, pos{b: b + 1, e: len(str)})
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

type pos struct {
	b, e int
}

func readFile(path string) (apoco.Chars, error) {
	is, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("readFile %s: %v", path, err)
	}
	defer is.Close()
	var line apoco.Chars
	if strings.HasSuffix(path, ".txt") {
		line, err = readTXT(is)
	} else {
		line, err = readTSV(is)
	}
	if err != nil {
		return nil, fmt.Errorf("readFile %s: %v", path, err)
	}
	return line, nil
}

func readTXT(is io.Reader) (apoco.Chars, error) {
	var chars apoco.Chars
	s := bufio.NewScanner(is)
	for s.Scan() {
		for _, r := range s.Text() {
			chars = appendChar(chars, apoco.Char{Char: r})
		}
		break
	}
	if s.Err() != nil {
		return nil, fmt.Errorf("readTXT: %v", s.Err())
	}
	return trim(chars), nil
}

func readTSV(is io.Reader) (apoco.Chars, error) {
	var chars apoco.Chars
	s := bufio.NewScanner(is)
	for s.Scan() {
		var c apoco.Char
		if _, err := fmt.Sscanf(s.Text(), "%c\t%f", &c.Char, &c.Conf); err != nil {
			// The TSV files contain artifacts of the form "\t%f".
			// Skip these.
			var tmp float64
			if _, err := fmt.Sscanf(s.Text(), "\t%f", &tmp); err != nil {
				return nil, fmt.Errorf("readTSV: %v", err)
			}
			continue
		}
		chars = appendChar(chars, c)
	}
	if s.Err() != nil {
		return nil, fmt.Errorf("readTSV: %v", s.Err())
	}
	return trim(chars), nil
}

func appendChar(chars apoco.Chars, c apoco.Char) apoco.Chars {
	if len(chars) == 0 {
		return append(chars, c)
	}
	// We do not want to append multiple whitespaces.
	if unicode.IsSpace(chars[len(chars)-1].Char) && unicode.IsSpace(c.Char) {
		return chars
	}
	return append(chars, c)
}

func trim(chars apoco.Chars) apoco.Chars {
	var i, j int
	for i = 0; i < len(chars); i++ {
		if !unicode.IsSpace(chars[i].Char) {
			break
		}
	}
	for j = len(chars); j > i; j-- {
		if !unicode.IsSpace(chars[j-1].Char) {
			break
		}
	}
	return chars[i:j]
}

func runes(chars apoco.Chars) []rune {
	runes := make([]rune, len(chars))
	for i := range chars {
		runes[i] = chars[i].Char
	}
	return runes
}
