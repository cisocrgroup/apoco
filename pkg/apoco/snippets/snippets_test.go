package snippets

import (
	"context"
	"testing"

	"golang.org/x/sync/errgroup"
)

// testdata/dir/a/00001.gt.txt:Fritſch, ein unverheyratheter Mann von hoͤchſt ein—1
// testdata/dir/b/00002.gt.txt:Da in der Bundes-Acte zu Wien ſo Guͤnſtiges
func TestTokenize(t *testing.T) {
	ext := Extensions{".prob.1", ".prob.2", ".gt.txt"}
	var g errgroup.Group
	n, want := 0, 16
	for token := range ext.Tokenize("testdata/dir")(context.Background(), &g, nil) {
		n++
		// for token := range tok(context.Background(), &g, nil) {
		if len(token.Tokens) != 3 {
			t.Fatalf("bad token: %s", token)
		}
		if token.Group != "dir" {
			t.Fatalf("bad group: %s", token.Group)
		}
		if token.File != "testdata/dir/a/00001.prob.1" &&
			token.File != "testdata/dir/b/00002.prob.1" {
			t.Fatalf("bad file: %s", token.File)
		}
	}
	if n != want {
		t.Fatalf("invalid number of tokens: expected %d; got %d", want, n)
	}
	if err := g.Wait(); err != nil {
		t.Fatalf("got error: %v", err)
	}
}

// voll. Diſe wurtzel reiniget die mů
func TestCalamari(t *testing.T) {
	ext := Extensions{".json"}
	var g errgroup.Group
	want := []string{"voll.", "Diſe", "wurtzel", "reiniget", "die", "mů"}
	var i int
	for token := range ext.Tokenize("testdata/dir")(context.Background(), &g, nil) {
		if len(token.Tokens) != 1 {
			t.Fatalf("bad token: %s", token)
		}
		if token.Group != "dir" {
			t.Fatalf("bad group: %s", token.Group)
		}
		if token.File != "testdata/dir/a/00010.json" {
			t.Fatalf("bad file: %s", token.File)
		}
		if got := token.Tokens[0]; got != want[i] {
			t.Fatalf("expected %q; got %q", want[i], got)
		}
		i++
	}
	if i != len(want) {
		t.Fatalf("invalid number of tokens: expected %d; got %d", len(want), i)
	}
	if err := g.Wait(); err != nil {
		t.Fatalf("got error: %v", err)
	}
}
