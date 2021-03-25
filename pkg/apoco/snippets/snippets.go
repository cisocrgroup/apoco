package snippets

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/align"
)

// Extensions is used to tokenize snippets in directories using the
// list of file extensions.
type Extensions []string

// Tokenize is a helper function that combines ReadLines and
// TokenizeLines into one function.  It is the same as calling
// `apoco.Pipe(ReadLines, TokenizeLines,...)`.
func (e Extensions) Tokenize(ctx context.Context, dirs ...string) apoco.StreamFunc {
	return apoco.Combine(ctx, e.ReadLines(dirs...), e.TokenizeLines)
}

// ReadLines returns a stream function that reads snippet files
// (identyfied by the given file extensions) and returns a stream of
// line tokens.
//
// If a extension ends with `.txt`, one line is read from the text
// file (no confidences); if the file ends with `.json`, calamari's
// extended data format is assumed. Otherwise the file is read as a
// TSV file expecting a char (or a sequence thereof) and its
// confidence on each line.
func (e Extensions) ReadLines(dirs ...string) apoco.StreamFunc {
	return func(ctx context.Context, _ <-chan apoco.T, out chan<- apoco.T) error {
		for _, dir := range dirs {
			if err := e.readLinesFromDir(ctx, out, dir); err != nil {
				return fmt.Errorf("read lines %s: %v", dir, err)
			}
		}
		return nil
	}
}

func (e Extensions) readLinesFromDir(ctx context.Context, out chan<- apoco.T, base string) error {
	// Use a dir path stack to iterate over all dirs in the tree.
	stack := []string{base}
	for len(stack) != 0 {
		dir := stack[len(stack)-1]
		stack = stack[0 : len(stack)-1]
		// Read all file info entries from the dir.
		dirs, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("read lines from dir %s: %v", dir, err)
		}
		// Either append new dirs to the stack or handle files with
		// the master file extension at index 0. Skip all other files.
		for i := range dirs {
			if dirs[i].IsDir() {
				stack = append(stack, filepath.Join(dir, dirs[i].Name()))
				continue
			}
			if !strings.HasSuffix(dirs[i].Name(), e[0]) {
				continue
			}
			file := filepath.Join(dir, dirs[i].Name())
			if err := e.readLinesFromSnippets(ctx, out, base, file); err != nil {
				return fmt.Errorf("read lines from dir %s: %v", dir, err)
			}
		}
	}
	return nil
}

func (e Extensions) readLinesFromSnippets(ctx context.Context, out chan<- apoco.T, base, file string) error {
	var lines []apoco.Chars
	pairs, err := readSnippetFile(file)
	if err != nil {
		return fmt.Errorf("read lines from snippets %s: %v", file, err)
	}
	lines = append(lines, pairs)
	for i := 1; i < len(e); i++ {
		path := file[0:len(file)-len(e[0])] + e[i]
		pairs, err := readSnippetFile(path)
		if err != nil {
			return fmt.Errorf("read lines from snippets %s: %v", file, err)
		}
		lines = append(lines, pairs)
	}
	err = apoco.SendTokens(ctx, out, apoco.T{
		Chars:  lines[0],
		Confs:  lines[0].Confs(),
		Group:  filepath.Base(base),
		File:   file,
		ID:     filepath.Base(file),
		Tokens: makeTokensFromPairs(lines),
	})
	if err != nil {
		return fmt.Errorf("read lines from snippets %s: %v", file, err)
	}
	return nil
}

func makeTokensFromPairs(lines []apoco.Chars) []string {
	ret := make([]string, len(lines))
	for i := range lines {
		ret[i] = lines[i].String()
	}
	return ret
}

// TokenizeLines is a stream function that tokenizes and aligns line
// tokens.
func (e Extensions) TokenizeLines(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
	return apoco.EachToken(ctx, in, func(line apoco.T) error {
		alignments := alignLines(line.Tokens...)
		for i := range alignments {
			t := apoco.T{
				File:  line.File,
				Group: line.Group,
				ID:    line.ID + ":" + strconv.Itoa(i+1),
			}
			for j, p := range alignments[i] {
				if j == 0 {
					t.Chars = line.Chars[p.B:p.E]
					t.Confs = line.Chars[p.B:p.E].Confs()
				}
				t.Tokens = append(t.Tokens, string(p.Slice()))
			}
			if err := apoco.SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("tokenize lines: %v", err)
			}
		}
		return nil
	})
}

func alignLines(lines ...string) [][]align.Pos {
	rs := make([][]rune, len(lines))
	for i := range lines {
		rs[i] = []rune(lines[i])
	}
	return align.Do(rs[0], rs[1:]...)
}

func readSnippetFile(path string) (apoco.Chars, error) {
	is, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("readFile %s: %v", path, err)
	}
	defer is.Close()
	var line apoco.Chars
	switch filepath.Ext(path) {
	case ".txt":
		line, err = readTXT(is)
	case ".json":
		line, err = readJSON(is)
	default:
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
		_, err := fmt.Sscanf(s.Text(), "%c\t%f", &c.Char, &c.Conf)
		if err == nil {
			chars = appendChar(chars, c)
			continue
		}
		// The TSV files contain also some artifacts of the
		// form "%c%c\t%f"
		var str string
		var conf float64
		_, err = fmt.Sscanf(s.Text(), "%s\t%f", &str, &conf)
		if err == nil {
			for _, c := range str {
				chars = appendChar(chars, apoco.Char{Conf: conf, Char: c})
			}
			continue
		}
		// The TSV files contain artifacts of the form "\t%f".
		// We add whitespaces in these cases. It would be
		// better to treat these entries as empty strings and
		// skip them, but in order to be compatible with the
		// old java-version of the automatic post-correction,
		// we use whitespace.
		_, err = fmt.Sscanf(s.Text(), "\t%f", &conf)
		if err != nil {
			return nil, fmt.Errorf("read tsv: bad line %s: %v", s.Text(), err)
		}
		chars = appendChar(chars, apoco.Char{Conf: conf, Char: ' '})
	}
	if s.Err() != nil {
		return nil, fmt.Errorf("read tsv: %v", s.Err())
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

func readJSON(in io.Reader) (apoco.Chars, error) {
	var data calamariPredictions
	if err := json.NewDecoder(in).Decode(&data); err != nil {
		return nil, fmt.Errorf("cannot read json: %v", err)
	}
	var ret apoco.Chars
	for _, p := range data.Predictions {
		if p.ID != "voted" {
			continue
		}
		for _, pos := range p.Positions {
			if len(pos.Chars) == 0 {
				continue
			}
			for _, r := range pos.Chars[0].Char {
				ret = append(ret, apoco.Char{
					Char: r,
					Conf: pos.Chars[0].Prob,
				})
			}
		}
	}
	return ret, nil
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

type calamariChar struct {
	Char string  `json:"char"`
	Prob float64 `json:"probability"`
}

type calamariPositions struct {
	Chars []calamariChar `json:"chars"`
}

type calamariPrediction struct {
	ID        string              `json:"id"`
	Positions []calamariPositions `json:"positions"`
}

type calamariPredictions struct {
	Predictions []calamariPrediction `json:"predictions"`
}
