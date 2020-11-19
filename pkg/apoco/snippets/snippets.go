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
	"golang.org/x/sync/errgroup"
)

// Extensions is used to tokenize snippets in directories using the
// list of file extensions.
type Extensions []string

// Tokenize tokenizes tokens from line snippets TSV files (identyfied
// by the given file extensions) and alignes them accordingly.
//
// If a extension ends with `.txt`, one line is read from the text
// file (no confidences); if the file ends with `.json`, calamari's
// extended data format is assumed. Otherwise the file is read as a
// TSV file expecting on char and its confidence on each line.
func (e Extensions) Tokenize(dirs ...string) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, _ <-chan apoco.Token) <-chan apoco.Token {
		out := make(chan apoco.Token)
		g.Go(func() error {
			defer close(out)
			// Iterate over the directories and read the tokens from each dir.
			for _, dir := range dirs {
				if err := e.sendTokensFromDir(ctx, out, dir); err != nil {
					return fmt.Errorf("tokenize: %v", err)
				}
			}
			return nil
		})
		return out
	}
}

func (e Extensions) sendTokensFromDir(ctx context.Context, out chan<- apoco.Token, bdir string) error {
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
			if err := e.sendTokensFromSnippets(ctx, out, bdir, file); err != nil {
				return fmt.Errorf("readTokensFromDir %s: %v", bdir, err)
			}
		}
	}
	return nil
}

func (e Extensions) sendTokensFromSnippets(ctx context.Context, out chan<- apoco.Token, bdir, file string) error {
	var lines []apoco.Chars
	pairs, err := readFile(file)
	if err != nil {
		return fmt.Errorf("sendTokensFromSnippets: %v", err)
	}
	lines = append(lines, pairs)
	for i := 1; i < len(e); i++ {
		path := file[0:len(file)-len(e[0])] + e[i]
		pairs, err := readFile(path)
		if err != nil {
			return fmt.Errorf("sendTokensFromSnippets %s: %v", file, err)
		}
		lines = append(lines, pairs)
	}
	if err := sendTokens(ctx, out, bdir, file, lines); err != nil {
		return fmt.Errorf("sendTokensFromSnippets: %v", err)
	}
	return nil
}

func sendTokens(ctx context.Context, out chan<- apoco.Token, bdir, file string, lines []apoco.Chars) error {
	alignments := alignLines(lines...)
	for i := range alignments {
		t := apoco.Token{
			File:  file,
			Group: filepath.Base(bdir),
			ID:    strconv.Itoa(i + 1),
		}
		for j, p := range alignments[i] {
			if j == 0 {
				t.Chars = lines[j][p.B:p.E]
			}
			t.Tokens = append(t.Tokens, string(p.Slice())) //lines[j][p.b:p.e])))
		}
		if err := apoco.SendTokens(ctx, out, t); err != nil {
			return fmt.Errorf("sendTokens: %v", err)
		}
	}
	return nil
}

func alignLines(lines ...apoco.Chars) [][]align.Pos {
	var rs [][]rune
	for _, line := range lines {
		rs = append(rs, runes(line))
	}
	return align.Do(rs[0], rs[1:]...)
}

func readFile(path string) (apoco.Chars, error) {
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
