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
	tok := ext.Tokenize("testdata/dir")
	for token := range tok(context.Background(), &g, nil) {
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
	if err := g.Wait(); err != nil {
		t.Fatalf("got error: %v", err)
	}
}
