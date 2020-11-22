package snippets

import (
	"context"
	"testing"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
)

const testDir = "testdata/dir"

func iterate(fn func(apoco.Token) error) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.Token, out chan<- apoco.Token) error {
		return apoco.EachToken(ctx, in, fn)
	}
}

// testdata/dir/a/00001.gt.txt:Fritſch, ein unverheyratheter Mann von hoͤchſt ein—1
// testdata/dir/b/00002.gt.txt:Da in der Bundes-Acte zu Wien ſo Guͤnſtiges
func TestTokenize(t *testing.T) {
	ext := Extensions{".prob.1", ".prob.2", ".gt.txt"}
	n, want := 0, 16
	ctx := context.Background()
	err := apoco.Pipe(ctx, ext.Tokenize(ctx, testDir), iterate(func(tok apoco.Token) error {
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
	err := apoco.Pipe(ctx, ext.Tokenize(ctx, testDir), iterate(func(tok apoco.Token) error {
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
