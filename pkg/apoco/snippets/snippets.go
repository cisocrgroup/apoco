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

	"example.com/apoco/pkg/apoco"
	"golang.org/x/sync/errgroup"
)

// Tokenize tokenizes tokens from line snippets TSV files (identyfied
// by the given file extensions) and alignes them accordingly.  If a extension
// ends with `.txt`, one line is read from the text file (no confidences).  Otherwise
// the file is read as a TSV file expecting on char and its confidence on each line.
func Tokenize(fileExts []string, dirs ...string) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, _ <-chan apoco.Token) <-chan apoco.Token {
		out := make(chan apoco.Token)
		g.Go(func() error {
			defer close(out)
			// Iterate over the directories and read the tokens from each dir.
			for _, dir := range dirs {
				if err := readTokensFromDir(ctx, out, dir, fileExts); err != nil {
					return fmt.Errorf("tokenize: %v", err)
				}
			}
			return nil
		})
		return out
	}
}

func readTokensFromDir(ctx context.Context, out chan<- apoco.Token, dir string, fileExts []string) error {
	if len(fileExts) == 0 {
		return fmt.Errorf("readTokensFromDir %s: empty file extensions", dir)
	}
	// Use a dir path stack to iterate over all dirs in the tree.
	dirs := []string{dir}
	for len(dirs) != 0 {
		dir := dirs[len(dirs)-1]
		dirs = dirs[0 : len(dirs)-1]
		// Read all file info entries from the dir.
		is, err := os.Open(dir)
		if err != nil {
			return fmt.Errorf("readTokensFromDir %s: %v", dir, err)
		}
		fis, err := is.Readdir(-1)
		is.Close() // Unconditionally close the dir.
		if err != nil {
			return fmt.Errorf("readTokensFromDir %s: %v", dir, err)
		}
		// Either append new dirs to the stack or handle files with
		// the master file extension at index 0. Skip all other files.
		for _, fi := range fis {
			if fi.IsDir() {
				dirs = append(dirs, filepath.Join(dir, fi.Name()))
				continue
			}
			if !strings.HasSuffix(fi.Name(), fileExts[0]) {
				continue
			}
			if err := readTokensFromSnippets(ctx, out, filepath.Join(dir, fi.Name()), fileExts); err != nil {
				return fmt.Errorf("readTokensFromDir %s: %v", dir, err)
			}
		}
	}
	return nil
}

func readTokensFromSnippets(ctx context.Context, out chan<- apoco.Token, file string, fileExts []string) error {
	var lines []apoco.Chars
	pairs, err := readFile(file)
	if err != nil {
		return fmt.Errorf("readTokensFromSnippets: %v", err)
	}
	lines = append(lines, pairs)
	for i := 1; i < len(fileExts); i++ {
		path := file[0:len(file)-len(fileExts[0])] + fileExts[i]
		pairs, err := readFile(path)
		if err != nil {
			return fmt.Errorf("readTokensFromSnippets: %v", err)
		}
		lines = append(lines, pairs)
	}
	if err := sendTokens(ctx, out, file, lines); err != nil {
		return fmt.Errorf("readTokensFromSnippets: %v", err)
	}
	return nil
}

func sendTokens(ctx context.Context, out chan<- apoco.Token, file string, lines []apoco.Chars) error {
	alignments := align(lines...)
	for i := range alignments {
		var t apoco.Token
		t.File = file
		t.ID = strconv.Itoa(i + 1)
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
		ret = append(ret, pos{b: b + 1, e: e})
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

type pos struct {
	b, e int
}

func readFile(path string) (apoco.Chars, error) {
	is, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("readFile %s: %v", path, err)
	}
	defer is.Close()
	var pairs apoco.Chars
	if strings.HasSuffix(path, ".txt") {
		pairs, err = readTXT(is)
	} else {
		pairs, err = readTSV(is)
	}
	if err != nil {
		return nil, fmt.Errorf("readFile: %v", err)
	}
	return pairs, nil
}

func readTXT(is io.Reader) (apoco.Chars, error) {
	var chars apoco.Chars
	s := bufio.NewScanner(is)
	for s.Scan() {
		for _, r := range s.Text() {
			chars = append(chars, apoco.Char{Char: r})
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
		if _, err := fmt.Sscanf("%c\t%f", s.Text(), &c.Char, &c.Char); err != nil {
			return nil, fmt.Errorf("readTSV: %v", err)
		}
		chars = append(chars, c)
	}
	if s.Err() != nil {
		return nil, fmt.Errorf("readTSV: %v", s.Err())
	}
	return trim(chars), nil
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
