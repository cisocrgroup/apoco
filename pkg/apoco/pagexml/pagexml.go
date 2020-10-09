package pagexml

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/node"
	"github.com/antchfx/xmlquery"
	"golang.org/x/sync/errgroup"
)

// MIMEType defines the mime type for page xml documents.
const MIMEType = "application/vnd.prima.page+xml"

// Tokenize returns a function that reads tokens from the page xml
// files of the given file groups.  An empty token is inserted as
// sentry between the token of different file groups.  The returned
// function ignores the input stream it just writes tokens to the
// output stream.
func Tokenize(mets string, fgs ...string) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, _ <-chan apoco.Token) <-chan apoco.Token {
		out := make(chan apoco.Token)
		g.Go(func() error {
			defer close(out)
			for _, fg := range fgs {
				files, err := FilePathsForFileGrp(mets, fg)
				if err != nil {
					return fmt.Errorf("tokenize %s: %v", mets, err)
				}
				for _, file := range files {
					if err := tokenizePageXML(ctx, filepath.Dir(file), file, out); err != nil {
						return err
					}
				}
			}
			return nil
		})
		return out
	}
}

// TokenizeDirs returns a function that reads page xml files with a
// matching file extension from the given directories.  The returned
// function ignores the input stream.  It only writes tokens to the
// output stream.
func TokenizeDirs(ext string, dirs ...string) apoco.StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, _ <-chan apoco.Token) <-chan apoco.Token {
		out := make(chan apoco.Token)
		g.Go(func() error {
			defer close(out)
			for _, dir := range dirs {
				files, err := gatherFilesInDir(dir, ext)
				if err != nil {
					return fmt.Errorf("tokenize dir %s: %v", dir, err)
				}
				for _, file := range files {
					if err := tokenizePageXML(ctx, dir, file, out); err != nil {
						return err
					}
				}
			}
			return nil
		})
		return out
	}
}

func gatherFilesInDir(dir, ext string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(p string, i os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if i.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ext) {
			files = append(files, p)
		}
		return nil
	})
	return files, err
}

// FilePathsForFileGrp returns the list of file paths for the given
// file group.  The returned file paths are updated to be relative to
// the mets's file base directory.
func FilePathsForFileGrp(mets, fg string) ([]string, error) {
	is, err := os.Open(mets)
	if err != nil {
		return nil, fmt.Errorf("filePathsForFileGrp %s: cannot open: %v", mets, err)
	}
	defer is.Close()
	doc, err := xmlquery.Parse(is)
	if err != nil {
		return nil, fmt.Errorf("filePathsForFileGrp %s: cannot parse: %v", mets, err)
	}
	nodes, err := findFileGrpFLocatFromRoot(doc, fg)
	if err != nil {
		return nil, fmt.Errorf("filePathsForFileGrp %s: cannot find file group %s: %v",
			mets, fg, err)
	}
	base := filepath.Dir(mets)
	ret := make([]string, len(nodes))
	for i, n := range nodes {
		link, ok := node.LookupAttr(n, xml.Name{Space: "xlink", Local: "href"})
		if !ok {
			return nil, fmt.Errorf("filePathsForFileGrp %s: missing href attribute", mets)
		}
		ret[i] = filepath.Join(base, link)
	}
	return ret, nil
}

func tokenizePageXML(ctx context.Context, fg, file string, out chan<- apoco.Token) error {
	is, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("tokenizePageXML %s: %v", file, err)
	}
	defer is.Close()
	doc, err := xmlquery.Parse(is)
	if err != nil {
		return fmt.Errorf("tokenizePageXML %s: %v", file, err)
	}
	words, err := xmlquery.QueryAll(doc, "//*[local-name()='Word']")
	if err != nil {
		return fmt.Errorf("tokenizePageXML %s: %v", file, err)
	}
	for _, word := range words {
		token, err := newTokenFromNode(fg, file, word)
		if err != nil {
			return fmt.Errorf("tokenizePageXML %s: %v", file, err)
		}
		if !findPrevSibling(word, "Word") {
			token.SetTrait(0, apoco.FirstInLine)
		}
		if !findNextSibling(word, "Word") {
			token.SetTrait(0, apoco.LastInLine)
		}
		if err := apoco.SendTokens(ctx, out, token); err != nil {
			return fmt.Errorf("tokenizePageXML: %v", err)
		}
	}
	return nil
}

func newTokenFromNode(fg, file string, wordNode *xmlquery.Node) (apoco.Token, error) {
	id, ok := node.LookupAttr(wordNode, xml.Name{Local: "id"})
	if !ok {
		return apoco.Token{}, fmt.Errorf("newTokenFromNode: missing id for word node")
	}
	ret := apoco.Token{Group: fg, File: file, ID: id}
	lines := FindUnicodesInRegionSorted(node.Parent(wordNode))
	words := FindUnicodesInRegionSorted(wordNode)
	for i := 0; i < len(lines) && i < len(words); i++ {
		if i == 0 {
			chars, err := readCharsFromNode(node.Parent(node.Parent(words[i])))
			if err != nil {
				return apoco.Token{}, fmt.Errorf("newTokenFromNode: %v", err)
			}
			ret.Chars = chars
		}
		conf, _ := node.LookupAttrAsFloat(node.Parent(words[i]), xml.Name{Local: "conf"})
		ret.Confs = append(ret.Confs, conf)
		ret.Tokens = append(ret.Tokens, node.Data(node.FirstChild(words[i])))
		ret.Lines = append(ret.Lines, node.Data(node.FirstChild(lines[i])))
	}
	return ret, nil
}

func readCharsFromNode(wordNode *xmlquery.Node) ([]apoco.Char, error) {
	chars, err := node.QueryAll(wordNode, "./*[local-name()='Glyph']/*[local-name()='TextEquiv']/*[local-name()='Unicode']")
	if err != nil {
		return nil, fmt.Errorf("readCharsFromNode: %v", err)
	}
	var ret []apoco.Char
	for _, char := range chars {
		conf, _ := node.LookupAttrAsFloat(char.Parent, xml.Name{Local: "conf"})
		data := node.Data(node.FirstChild(char))
		for _, r := range data {
			ret = append(ret, apoco.Char{Char: r, Conf: conf})
		}
	}
	return ret, nil
}

func findFileGrpFLocatFromRoot(doc *xmlquery.Node, fg string) ([]*xmlquery.Node, error) {
	expr := fmt.Sprintf("/*[local-name()='mets']/*[local-name()='fileSec']"+
		"/*[local-name()='fileGrp'][@USE=%q]/*[local-name()='file']/*[local-name()='FLocat']", fg)
	return node.QueryAll(doc, expr)
}

// FindUnicodesInRegionSorted searches for the TextEquiv / Unicode
// nodes beneath a text region (TextRegion, Line, Word, Glyph).  The
// returend node list is ordered by the TextEquiv's index entries
// (interpreted as integers).
func FindUnicodesInRegionSorted(region *xmlquery.Node) []*xmlquery.Node {
	expr := "./*[local-name()='TextEquiv']/*[local-name()='Unicode']"
	nodes := xmlquery.Find(region, expr)
	index := xml.Name{Local: "index"}
	sort.Slice(nodes, func(i, j int) bool {
		ii, _ := node.LookupAttrAsInt(nodes[i].Parent, index)
		ij, _ := node.LookupAttrAsInt(nodes[j].Parent, index)
		return ii < ij
	})
	return nodes
}

// NodeForToken is just a tmp testing function.
func NodeForToken(doc *xmlquery.Node, t apoco.Token) (*xmlquery.Node, error) {
	return findWordFromRoot(doc, t.ID)
}

func findWordFromRoot(doc *xmlquery.Node, id string) (*xmlquery.Node, error) {
	expr := fmt.Sprintf("//*[local-name()='Word'][@id=%q]", id)
	return node.Query(doc, expr)
}

func findNextSibling(node *xmlquery.Node, data string) bool {
	for n := node.NextSibling; n != nil; n = n.NextSibling {
		if n.Data == data {
			return true
		}
	}
	return false
}

func findPrevSibling(node *xmlquery.Node, data string) bool {
	for n := node.PrevSibling; n != nil; n = n.PrevSibling {
		if n.Data == data {
			return true
		}
	}
	return false
}
