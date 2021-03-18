package snippets

import (
	"context"
	"testing"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
)

const (
	testDir  = "testdata/dir"
	testDir2 = "testdata/dir2"
)

func iterate(fn func(apoco.T) error) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		return apoco.EachToken(ctx, in, fn)
	}
}

// testdata/dir/a/00001.gt.txt:Fritſch, ein unverheyratheter Mann von hoͤchſt ein—1
// testdata/dir/b/00002.gt.txt:Da in der Bundes-Acte zu Wien ſo Guͤn nſtiges
func TestTokenizeDir(t *testing.T) {
	ext := Extensions{".prob.1", ".prob.2", ".gt.txt"}
	n, want := 0, 17
	ctx := context.Background()
	err := apoco.Pipe(ctx, ext.Tokenize(ctx, testDir), iterate(func(tok apoco.T) error {
		n++
		if len(tok.Tokens) != 3 {
			t.Fatalf("bad token: %s", tok)
		}
		if tok.Group != "dir" {
			t.Fatalf("bad group: %s", tok.Group)
		}
		if tok.File != "testdata/dir/a/00001.prob.1" &&
			tok.File != "testdata/dir/b/00002.prob.1" {
			t.Fatalf("bad file: %s", tok.File)
		}
		return nil
	}))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if n != want {
		t.Fatalf("invalid number of tokens: expected %d; got %d", want, n)
	}
}

// voll. Diſe wurtzel reiniget die mů
func TestCalamari(t *testing.T) {
	ext := Extensions{".json"}
	want := []string{"voll.", "Diſe", "wurtzel", "reiniget", "die", "mů"}
	var i int
	ctx := context.Background()
	err := apoco.Pipe(ctx, ext.Tokenize(ctx, testDir), iterate(func(tok apoco.T) error {
		if len(tok.Tokens) != 1 {
			t.Fatalf("bad token: %s", tok)
		}
		if tok.Group != "dir" {
			t.Fatalf("bad group: %s", tok.Group)
		}
		if tok.File != "testdata/dir/a/00010.json" {
			t.Fatalf("bad file: %s", tok.File)
		}
		if got := tok.Tokens[0]; got != want[i] {
			t.Fatalf("expected %q; got %q", want[i], got)
		}
		i++
		return nil
	}))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if i != len(want) {
		t.Fatalf("invalid number of tokens: expected %d; got %d", len(want), i)
	}
}

func TestTokenizeDir2(t *testing.T) {
	ext := Extensions{".prob.1", ".prob.2", ".gt.txt"}
	n, want := 0, 10
	ctx := context.Background()
	err := apoco.Pipe(ctx, ext.Tokenize(ctx, testDir2), iterate(func(tok apoco.T) error {
		n++
		if len(tok.Tokens) != 3 {
			t.Fatalf("bad token: %s", tok)
		}
		if tok.Group != "dir2" {
			t.Fatalf("bad group: %s", tok.Group)
		}
		if tok.File != "testdata/dir2/00001.prob.1" {
			t.Fatalf("bad file: %s", tok.File)
		}
		return nil
	}))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if n != want {
		t.Fatalf("invalid number of tokens: expected %d; got %d", want, n)
	}
}
