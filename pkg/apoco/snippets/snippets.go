package snippets

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/align"
	"git.sr.ht/~flobar/lev"
	"golang.org/x/sync/errgroup"
)

// Extensions is used to tokenize snippets in directories using the
// list of file extensions.
type Extensions []string

// Tokenize is a helper function that combines ReadLines and
// TokenizeLines into one function.  It is the same as calling
// `apoco.Pipe(ReadLines, TokenizeLines,...)`.
func (e Extensions) Tokenize(ctx context.Context, lev bool, dirs ...string) apoco.StreamFunc {
	return apoco.Combine(ctx, e.ReadLines(dirs...), e.TokenizeLines(lev))
}

// ReadLines returns a stream function that reads snippet files in
// the directories (identyfied by the given file extensions) and
// returns a stream of line tokens.  The directories are read in
// parallel by GOMAXPROCS goroutines.
//
// If a extension ends with `.txt`, one line is read from the text
// file (no confidences); if the file ends with `.json`, calamari's
// extended data format is assumed. Otherwise the file is read as a
// TSV file expecting a char (or a sequence thereof) and its
// confidence on each line.
func (e Extensions) ReadLines(dirs ...string) apoco.StreamFunc {
	return func(ctx context.Context, _ <-chan apoco.T, out chan<- apoco.T) error {
		if len(dirs) == 0 {
			return nil
		}
		g, gctx := errgroup.WithContext(ctx)
		linechan := make(chan []apoco.T)
		docchan := make(chan *apoco.Document)
		var wg sync.WaitGroup
		// Feed dirs into the dir channel and close it.
		g.Go(func() error {
			defer close(docchan)
			for _, dir := range dirs {
				select {
				case docchan <- &apoco.Document{Group: dir}:
				case <-gctx.Done():
					return gctx.Err()
				}
			}
			return nil
		})
		// Start GOMAXPROGS goroutines that read
		// the dir contents.
		n := runtime.GOMAXPROCS(0)
		wg.Add(n)
		for i := 0; i < n; i++ {
			g.Go(func() error {
				defer wg.Done()
				for {
					select {
					case doc, ok := <-docchan:
						if !ok {
							return nil
						}
						lines, err := e.readLinesFromDir(doc)
						if err != nil {
							return err
						}
						select {
						case linechan <- lines:
						case <-gctx.Done():
							return gctx.Err()
						}
					case <-gctx.Done():
						return gctx.Err()
					}
				}
			})
		}
		// Wait until all producers are done
		// and close the line channel.
		g.Go(func() error {
			wg.Wait()
			close(linechan)
			return nil
		})
		// Read lines an write their tokens
		// into the output channel.
		g.Go(func() error {
			for {
				select {
				case lines, ok := <-linechan:
					if !ok {
						return nil
					}
					if err := apoco.SendTokens(gctx, out, lines...); err != nil {
						return err
					}
				case <-gctx.Done():
					return gctx.Err()
				}
			}
		})
		return g.Wait()
	}
}

func (e Extensions) readLinesFromDir(doc *apoco.Document) ([]apoco.T, error) {
	// Use a dir path stack to iterate over all dirs in the tree.
	stack := []string{doc.Group}
	var lines []apoco.T
	for len(stack) != 0 {
		dir := stack[len(stack)-1]
		stack = stack[0 : len(stack)-1]
		// Read all file info entries from the dir.
		fis, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("read lines from dir %s: %v", dir, err)
		}
		// Either append new dirs to the stack or handle files with
		// the master file extension at index 0. Skip all other files.
		for i := range fis {
			if fis[i].IsDir() {
				stack = append(stack, filepath.Join(dir, fis[i].Name()))
				continue
			}
			if !strings.HasSuffix(fis[i].Name(), e[0]) {
				continue
			}
			file := filepath.Join(dir, fis[i].Name())
			t, err := e.readLinesFromSnippets(doc, file)
			if err != nil {
				return nil, fmt.Errorf("read lines from dir %s: %v", dir, err)
			}
			lines = append(lines, t)
		}
	}
	return lines, nil
}

func (e Extensions) readLinesFromSnippets(doc *apoco.Document, file string) (apoco.T, error) {
	var lines []apoco.Chars
	pairs, err := readSnippetFile(file)
	if err != nil {
		return apoco.T{}, fmt.Errorf("read lines from snippets %s: %v", file, err)
	}
	lines = append(lines, pairs)
	for i := 1; i < len(e); i++ {
		path := file[0:len(file)-len(e[0])] + e[i]
		pairs, err := readSnippetFile(path)
		if err != nil {
			return apoco.T{}, fmt.Errorf("read lines from snippets %s: %v", file, err)
		}
		lines = append(lines, pairs)
	}
	return apoco.T{
		Chars:    lines[0],
		Document: doc,
		File:     file,
		ID:       filepath.Base(file),
		Tokens:   makeTokensFromPairs(lines),
	}, nil
}

func makeTokensFromPairs(lines []apoco.Chars) []string {
	ret := make([]string, len(lines))
	for i := range lines {
		ret[i] = lines[i].Chars()
	}
	return ret
}

// TokenizeLines returns a stream function that tokenizes
// and aligns line tokens.
func (e Extensions) TokenizeLines(alev bool) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		var matrix lev.Mat
		mat := &matrix
		if !alev {
			mat = nil
		}
		return apoco.EachToken(ctx, in, func(line apoco.T) error {
			alignments := alignLines(mat, line.Tokens...)
			var ts []apoco.T
			for i := range alignments {
				t := apoco.T{
					File:     line.File,
					Document: line.Document,
					ID:       line.ID + ":" + strconv.Itoa(i+1),
				}
				for j, p := range alignments[i] {
					if j == 0 {
						t.Chars = line.Chars[p.B:p.E]
					}
					t.Tokens = append(t.Tokens, string(p.Slice()))
				}
				ts = append(ts, t)
			}
			// Remove all empty token from the end of the line.
			// Then mark the last token on the line.
			// We do the same for the first tokens on the line.
			for len(ts) > 0 {
				if ts[len(ts)-1].Tokens[0] != "" {
					ts[len(ts)-1].EOL = true
					break
				}
				ts = ts[:len(ts)-1]
			}
			for len(ts) > 0 {
				if ts[0].Tokens[0] != "" {
					ts[0].SOL = true
					break
				}
				ts = ts[1:]
			}
			if len(ts) > 0 { // Mark last token in the line.
				ts[len(ts)-1].EOL = true
			}
			if err := apoco.SendTokens(ctx, out, ts...); err != nil {
				return fmt.Errorf("tokenize lines: %v", err)
			}
			return nil
		})
	}
}

func alignLines(mat *lev.Mat, lines ...string) [][]align.Pos {
	rs := make([][]rune, len(lines))
	for i := range lines {
		rs[i] = []rune(lines[i])
	}
	if mat != nil {
		return align.Lev(mat, rs[0], rs[1:]...)
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
		//lint:ignore SA4004 We allways want to read exactly one line of the file.
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
