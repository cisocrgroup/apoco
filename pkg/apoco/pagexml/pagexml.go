package pagexml

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/mets"
	"git.sr.ht/~flobar/apoco/pkg/apoco/node"
	"github.com/antchfx/xmlquery"
)

// MIMEType defines the mime type for page xml documents.
const MIMEType = "application/vnd.prima.page+xml"

// Tokenize returns a function that reads tokens from the page xml
// files of the given file groups.  An empty token is inserted as
// sentry between the token of different file groups.  The returned
// function ignores the input stream it just writes tokens to the
// output stream.
func Tokenize(metsName string, fgs ...string) apoco.StreamFunc {
	return func(ctx context.Context, _ <-chan apoco.T, out chan<- apoco.T) error {
		m, err := mets.Open(metsName)
		if err != nil {
			return fmt.Errorf("tokenize: %v", err)
		}
		for _, fg := range fgs {
			files, err := m.FilePathsForFileGrp(fg)
			if err != nil {
				return fmt.Errorf("tokenize: %v", err)
			}
			for _, file := range files {
				err := tokenizePageXML(ctx, filepath.Dir(file), file, out)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
}

// TokenizeDirs returns a function that reads page xml files with a
// matching file extension from the given directories.  The returned
// function ignores the input stream.  It only writes tokens to the
// output stream.
func TokenizeDirs(ext string, dirs ...string) apoco.StreamFunc {
	return func(ctx context.Context, _ <-chan apoco.T, out chan<- apoco.T) error {
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

func tokenizePageXML(ctx context.Context, fg, file string, out chan<- apoco.T) error {
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
		if err := apoco.SendTokens(ctx, out, token); err != nil {
			return fmt.Errorf("tokenizePageXML %s: %v", file, err)
		}
	}
	return nil
}

func newTokenFromNode(fg, file string, wordNode *xmlquery.Node) (apoco.T, error) {
	id, ok := node.LookupAttr(wordNode, xml.Name{Local: "id"})
	if !ok {
		return apoco.T{}, fmt.Errorf("newTokenFromNode: missing id for word node")
	}
	base := filepath.Base(file)
	base = base[0 : len(base)-len(filepath.Ext(base))]
	id = base + "_" + id
	ret := apoco.T{Group: fg, File: file, ID: id}
	lines := FindUnicodesInRegionSorted(node.Parent(wordNode))
	words := FindUnicodesInRegionSorted(wordNode)
	for i := 0; i < len(lines) && i < len(words); i++ {
		if i == 0 {
			chars, err := readCharsFromNode(node.Parent(node.Parent(words[i])))
			if err != nil {
				return apoco.T{}, fmt.Errorf("newTokenFromNode: %v", err)
			}
			ret.Chars = chars
		}
		ret.Tokens = append(ret.Tokens, node.Data(node.FirstChild(words[i])))
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

// SetMetadata creates a new metadata node with the given content.  If
// a previous metadata node exists, it is deleted.
func SetMetadata(doc *xmlquery.Node, creator string, created, lastChange time.Time) {
	metadata := xmlquery.FindOne(doc, "/*[local-name()='PcGts']/*[local-name()='Metadata']")
	var pcgts *xmlquery.Node
	if metadata == nil {
		pcgts = xmlquery.FindOne(doc, "/*[local-name()='PcGts']")
		if pcgts == nil {
			return
		}
	} else {
		pcgts = metadata.Parent
	}
	newMetadata := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "Metadata",
		Prefix:       metadata.Prefix,
		NamespaceURI: metadata.NamespaceURI,
	}
	newCreator := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "Creator",
		Prefix:       metadata.Prefix,
		NamespaceURI: metadata.NamespaceURI,
	}
	node.AppendChild(newCreator, &xmlquery.Node{Type: xmlquery.TextNode, Data: creator})
	newCreated := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "Created",
		Prefix:       metadata.Prefix,
		NamespaceURI: metadata.NamespaceURI,
	}
	node.AppendChild(newCreated, &xmlquery.Node{
		Type: xmlquery.TextNode,
		Data: created.String(),
	})
	newLastChange := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "LastChange",
		Prefix:       metadata.Prefix,
		NamespaceURI: metadata.NamespaceURI,
	}
	node.AppendChild(newLastChange, &xmlquery.Node{
		Type: xmlquery.TextNode,
		Data: lastChange.String(),
	})
	node.AppendChild(newMetadata, newCreator)
	node.AppendChild(newMetadata, newCreated)
	node.AppendChild(newMetadata, newLastChange)
	node.Delete(metadata)
	node.PrependChild(pcgts, newMetadata)
}
