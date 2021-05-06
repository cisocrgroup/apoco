package snippets

import (
	"context"
	"path/filepath"
	"testing"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
)

const (
	testDir  = "testdata/dir"
	testDirA = "testdata/dir/a"
	testDirB = "testdata/dir/b"
	testDir2 = "testdata/dir2"
)

func iterate(t *testing.T, fn func(apoco.T) error) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		if out != nil {
			t.Errorf("out channel is not nil")
		}
		return apoco.EachToken(ctx, in, fn)
	}
}

// testdata/dir/a/00001.gt.txt:Fritſch, ein unverheyratheter Mann von hoͤchſt ein—1
// testdata/dir/b/00002.gt.txt:Da in der Bundes-Acte zu Wien ſo Guͤn nſtiges
func TestTokenizeDir(t *testing.T) {
	ext := Extensions{".prob.1", ".prob.2", ".gt.txt"}
	n, want := 0, 17
	ctx := context.Background()
	err := apoco.Pipe(ctx, ext.Tokenize(ctx, testDir), iterate(t, func(tok apoco.T) error {
		n++
		if len(tok.Tokens) != 3 {
			t.Errorf("bad token: %s", tok)
		}
		if tok.Document.Group != testDir {
			t.Errorf("bad group: %s", tok.Group)
		}
		if tok.File != filepath.Join("testdata", "dir", "a", "00001.prob.1") &&
			tok.File != filepath.Join("testdata", "dir", "b", "00002.prob.1") {
			t.Errorf("bad file: %s", tok.File)
		}
		return nil
	}))
	if err != nil {
		t.Errorf("got error: %v", err)
	}
	if n != want {
		t.Errorf("invalid number of tokens: expected %d; got %d", want, n)
	}
}

// testdata/dir/a/00001.gt.txt:Fritſch, ein unverheyratheter Mann von hoͤchſt ein—1
// testdata/dir/b/00002.gt.txt:Da in der Bundes-Acte zu Wien ſo Guͤn nſtiges
func TestTokenizeDirParallel(t *testing.T) {
	ext := Extensions{".prob.1", ".prob.2", ".gt.txt"}
	n, want := 0, 17
	ctx := context.Background()
	err := apoco.Pipe(ctx, ext.Tokenize(ctx, testDirA, testDirB), iterate(t, func(tok apoco.T) error {
		n++
		if len(tok.Tokens) != 3 {
			t.Errorf("bad token: %s", tok)
		}
		if tok.Document.Group != "a" && tok.Group != "b" {
			t.Errorf("bad group: %s", tok.Group)
		}
		if tok.File != filepath.Join("testdata", "dir", "a", "00001.prob.1") &&
			tok.File != filepath.Join("testdata", "dir", "b", "00002.prob.1") {
			t.Errorf("bad file: %s", tok.File)
		}
		return nil
	}))
	if err != nil {
		t.Errorf("got error: %v", err)
	}
	if n != want {
		t.Errorf("invalid number of tokens: expected %d; got %d", want, n)
	}
}

// voll. Diſe wurtzel reiniget die mů
func TestCalamari(t *testing.T) {
	ext := Extensions{".json"}
	want := []string{"voll.", "Diſe", "wurtzel", "reiniget", "die", "mů"}
	var i int
	ctx := context.Background()
	err := apoco.Pipe(ctx, ext.Tokenize(ctx, testDir), iterate(t, func(tok apoco.T) error {
		if len(tok.Tokens) != 1 {
			t.Errorf("bad token: %s", tok)
		}
		if tok.Document.Group != testDir {
			t.Errorf("bad group: %s", tok.Group)
		}
		if tok.File != filepath.Join("testdata", "dir", "a", "00010.json") {
			t.Errorf("bad file: %s", tok.File)
		}
		if got := tok.Tokens[0]; got != want[i] {
			t.Errorf("expected %q; got %q", want[i], got)
		}
		i++
		return nil
	}))
	if err != nil {
		t.Errorf("got error: %v", err)
	}
	if i != len(want) {
		t.Errorf("invalid number of tokens: expected %d; got %d", len(want), i)
	}
}

func TestTokenizeDir2(t *testing.T) {
	ext := Extensions{".prob.1", ".prob.2", ".gt.txt"}
	n, want := 0, 10
	ctx := context.Background()
	err := apoco.Pipe(ctx, ext.Tokenize(ctx, testDir2), iterate(t, func(tok apoco.T) error {
		n++
		if len(tok.Tokens) != 3 {
			t.Errorf("bad token: %s", tok)
		}
		if tok.Document.Group != testDir2 {
			t.Errorf("bad group: %s", tok.Group)
		}
		if tok.File != filepath.Join("testdata", "dir2", "00001.prob.1") {
			t.Errorf("bad file: %s", tok.File)
		}
		return nil
	}))
	if err != nil {
		t.Errorf("got error: %v", err)
	}
	if n != want {
		t.Errorf("invalid number of tokens: expected %d; got %d", want, n)
	}
}
