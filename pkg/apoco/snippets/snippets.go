package snippets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"example.com/apoco/pkg/apoco"
	"golang.org/x/sync/errgroup"
)

// Tokenize tokenizes tokens from line snippets TSV files (identyfied
// by the given file extensions) and alignes them accordingly.  For each
// file.ext if there exists a file.gt.txt, the ground truth is aligned as well.
func Tokenize(fileExts []string, dirs ...string) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, _ <-chan apoco.Token) <-chan apoco.Token {
		out := make(chan apoco.Token)
		g.Go(func() error {
			defer close(out)
			for _, dir := range dirs {
				if err := readTokensFromDirs(ctx, out, dir, fileExts); err != nil {
					return fmt.Errorf("tokenize: %v", err)
				}
			}
			return nil
		})
		return out
	}
}

func readTokensFromDirs(ctx context.Context, out chan<- apoco.Token, dir string, fileExts []string) error {
	if len(fileExts) == 0 {
		return fmt.Errorf("readTokensFromDir %s: empty file extensions", dir)
	}
	is, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("readTokensFromDir %s: %v", dir, err)
	}
	files, err := is.Readdirnames(-1)
	if err != nil {
		return fmt.Errorf("readTokensFromDirs %s: %v", dir, err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file, fileExts[0]) {
			continue
		}
		if err := readTokensFromSnippets(ctx, out, file, fileExts); err != nil {
			return fmt.Errorf("readTokensFromDirs %s: %v", dir, err)
		}
	}
	return nil
}

func readTokensFromSnippets(ctx context.Context, out chan<- apoco.Token, file string, fileExts []string) error {
	var lines []pairs
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
	if err := sendTokens(ctx, out, lines); err != nil {
		return fmt.Errorf("readTokensFromSnippets: %v", err)
	}
	return nil
}

func sendTokens(ctx context.Context, out chan<- apoco.Token, lines []pairs) error {
	return nil
}

type pair struct {
	conf float64
	char rune
}

type pairs []pair

func (ps pairs) runes() []rune {
	runes := make([]rune, len(ps))
	for i := range ps {
		runes[i] = ps[i].char
	}
	return runes
}

func readFile(path string) (pairs, error) {
	is, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("readFile %s: %v", path, err)
	}
	defer is.Close()
	var pairs []pair
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

func readTXT(is io.Reader) (pairs, error) {
	var pairs pairs
	s := bufio.NewScanner(is)
	for s.Scan() {
		for _, r := range s.Text() {
			pairs = append(pairs, pair{char: r})
		}
		break
	}
	if s.Err() != nil {
		return nil, fmt.Errorf("readTXT: %v", s.Err())
	}
	return pairs, nil
}

func readTSV(is io.Reader) (pairs, error) {
	var pairs pairs
	s := bufio.NewScanner(is)
	for s.Scan() {
		var p pair
		if _, err := fmt.Sscanf("%c\t%f", s.Text(), &p.char, &p.conf); err != nil {
			return nil, fmt.Errorf("readTSV: %v", err)
		}
	}
	if s.Err() != nil {
		return nil, fmt.Errorf("readCSV: %v", s.Err())
	}
	return pairs, nil
}
