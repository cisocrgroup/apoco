package correct

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"example.com/apoco/pkg/apoco"
	"example.com/apoco/pkg/apoco/node"
	"example.com/apoco/pkg/apoco/pagexml"
	"github.com/antchfx/xmlquery"
)

type corrector struct {
	corrected      map[string]map[string]apoco.Token // map file -> id -> token
	lexEntries     map[string]map[string]struct{}    // set file -> id
	ranks          map[string]map[string]int         // set file -> id -> rank
	mets, ofg, ifg string
	doc, fileGrp   *xmlquery.Node
	protocol       bool
}

func (cor *corrector) addCorrected(token apoco.Token) {
	if cor.corrected == nil {
		cor.corrected = make(map[string]map[string]apoco.Token)
	}
	if _, ok := cor.corrected[token.File]; !ok {
		cor.corrected[token.File] = make(map[string]apoco.Token)
	}
	cor.corrected[token.File][token.ID] = token
}

func (cor *corrector) addLex(token apoco.Token) {
	if cor.lexEntries == nil {
		cor.lexEntries = make(map[string]map[string]struct{})
	}
	if _, ok := cor.lexEntries[token.File]; !ok {
		cor.lexEntries[token.File] = make(map[string]struct{})
	}
	cor.lexEntries[token.File][token.ID] = struct{}{}
}

func (cor *corrector) addRank(token apoco.Token, rank int) {
	if cor.ranks == nil {
		cor.ranks = make(map[string]map[string]int)
	}
	if _, ok := cor.ranks[token.File]; !ok {
		cor.ranks[token.File] = make(map[string]int)
	}
	cor.ranks[token.File][token.ID] = rank
}

func (cor *corrector) correct() error {
	files, err := pagexml.FilePathsForFileGrp(cor.mets, cor.ifg)
	if err != nil {
		return fmt.Errorf("correct: %v", err)
	}
	for _, file := range files {
		if err := cor.correctFile(file); err != nil {
			return fmt.Errorf("correct: %v", err)
		}
	}
	if err := writeXML(cor.doc, cor.mets); err != nil {
		return fmt.Errorf("correct: %v", err)
	}
	return nil
}

func (cor *corrector) correctFile(file string) error {
	is, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("writeCorrections: %v", err)
	}
	defer is.Close()
	doc, err := xmlquery.Parse(is)
	if err != nil {
		return fmt.Errorf("writeCorrections: %v", err)
	}
	words, err := xmlquery.QueryAll(doc, "//*[local-name()='Word']")
	if err != nil {
		return fmt.Errorf("correct %s: %v", file, err)
	}
	for _, word := range words {
		if err := cor.correctWord(word, file); err != nil {
			return fmt.Errorf("correct %s: %v", file, err)
		}
	}
	if err := cor.write(doc, file); err != nil {
		return fmt.Errorf("correct %s: %v", file, err)
	}
	return nil
}

func (cor *corrector) correctWord(word *xmlquery.Node, file string) error {
	id, _ := node.LookupAttr(word, xml.Name{Local: "id"})
	unicodes := pagexml.FindUnicodesFromRegionSorted(word)
	if len(unicodes) == 0 {
		return nil
	}
	newTE := cor.makeTextEquiv(unicodes)
	newU := cor.makeUnicode(unicodes)
	newStr := &xmlquery.Node{Type: xmlquery.TextNode}
	ocr := node.Data(unicodes[0].FirstChild)
	const format = "skipped=%t short=%t lex=%t cor=%t rank=%d ocr=%s sug=%s gt=%s"
	if t, ok := cor.corrected[file][id]; !ok {
		_, lex := cor.lexEntries[file][id]
		gtnorm := notEmpty(strings.ToLower(trim(node.Data(unicodes[len(unicodes)-1].FirstChild))))
		ocrnorm := notEmpty(strings.ToLower(trim(ocr)))
		newStr.Data = ocr
		node.SetAttr(newTE, xml.Attr{
			Name:  xml.Name{Local: "dataTypeDetails"},
			Value: fmt.Sprintf(format, true, short(trim(ocr)), lex, false, 0, ocrnorm, notEmpty(""), gtnorm),
		})
	} else {
		rank := cor.ranks[t.File][t.ID]
		cor := t.Payload.(apoco.Correction)
		gt := notEmpty(t.Tokens[len(t.Tokens)-1])
		sug := cor.Candidate.Suggestion
		node.SetAttr(newTE, xml.Attr{
			Name:  xml.Name{Local: "conf"},
			Value: strconv.FormatFloat(cor.Conf, 'e', -1, 64),
		})
		if cor.Conf > .5 {
			newStr.Data = cor.Correction(ocr)
			node.SetAttr(newTE, xml.Attr{
				Name:  xml.Name{Local: "dataTypeDetails"},
				Value: fmt.Sprintf(format, false, false, false, true, rank, t.Tokens[0], sug, gt),
			})
		} else {
			newStr.Data = ocr
			node.SetAttr(newTE, xml.Attr{
				Name:  xml.Name{Local: "dataTypeDetails"},
				Value: fmt.Sprintf(format, false, false, false, false, rank, t.Tokens[0], sug, gt),
			})
		}
	}
	newTE.FirstChild = newU
	newU.Parent = newTE
	newU.FirstChild = newStr
	newStr.Parent = newU
	node.PrependSibling(unicodes[0].Parent, newTE)
	cor.cleanWord(word, unicodes)
	return nil
}

func (cor *corrector) cleanWord(word *xmlquery.Node, unicodes []*xmlquery.Node) {
	// Remove other TextEquivs.
	for _, u := range unicodes {
		node.Delete(u.Parent)
	}
	// Remove glyph nodes.
	for _, glyph := range xmlquery.Find(word, "./*[local-name()='Glyph']") {
		node.Delete(glyph)
	}
}

func (cor *corrector) makeTextEquiv(unicodes []*xmlquery.Node) *xmlquery.Node {
	newTE := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         unicodes[0].Parent.Data, // TextEquiv
		Prefix:       unicodes[0].Parent.Prefix,
		NamespaceURI: unicodes[0].Parent.NamespaceURI,
	}
	node.SetAttr(newTE, xml.Attr{
		Name:  xml.Name{Local: "index"},
		Value: "1",
	})
	conf, _ := node.LookupAttr(unicodes[0].Parent, xml.Name{Local: "conf"})
	node.SetAttr(newTE, xml.Attr{
		Name:  xml.Name{Local: "conf"},
		Value: conf,
	})
	if cor.protocol {
		node.SetAttr(newTE, xml.Attr{
			Name:  xml.Name{Local: "dataType"},
			Value: "apoco-correct",
		})
	}
	return newTE
}

func (cor *corrector) makeUnicode(unicodes []*xmlquery.Node) *xmlquery.Node {
	return &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "Unicode",
		Prefix:       unicodes[0].Parent.Prefix,
		NamespaceURI: unicodes[0].Parent.NamespaceURI,
	}
}

func (cor *corrector) write(doc *xmlquery.Node, file string) error {
	if cor.doc == nil || cor.fileGrp == nil {
		if err := cor.readMETS(); err != nil {
			return err
		}
	}
	cor.addFileToFileGrp(file)
	dir := filepath.Join(filepath.Dir(cor.mets), cor.ofg)
	ofile := filepath.Join(dir, filepath.Base(file))
	_ = os.MkdirAll(dir, 0777)
	return writeXML(doc, ofile)
}

func (cor *corrector) readMETS() error {
	is, err := os.Open(cor.mets)
	if err != nil {
		return fmt.Errorf("readMETS %s: %v", cor.mets, err)
	}
	defer is.Close()
	doc, err := xmlquery.Parse(is)
	if err != nil {
		return fmt.Errorf("readMETS %s: %v", cor.mets, err)
	}
	// Check if the given file group already exists and overwrite it.
	existing := xmlquery.FindOne(doc, fmt.Sprintf("//*[local-name()='fileGrp'][@USE=%q]", cor.ofg))
	if existing != nil {
		// Delete all children.
		existing.FirstChild = nil
		existing.LastChild = nil
		cor.doc = doc
		cor.fileGrp = existing
		return nil
	}
	// Add a new filegroup entry.
	fileGrps := xmlquery.Find(doc, "//*[local-name()='fileGrp']")
	if len(fileGrps) == 0 {
		return fmt.Errorf("readMETS %s: missing file grp", cor.mets)
	}
	cor.doc = doc
	cor.fileGrp = &xmlquery.Node{
		Data:         "fileGrp",
		Prefix:       fileGrps[0].Prefix,
		NamespaceURI: fileGrps[0].NamespaceURI,
	}
	node.SetAttr(cor.fileGrp, xml.Attr{
		Name:  xml.Name{Local: "USE"},
		Value: cor.ofg,
	})
	node.PrependSibling(fileGrps[0], cor.fileGrp)
	return nil
}

func (cor *corrector) addFileToFileGrp(file string) {
	fileid := filepath.Base(file[0 : len(file)-len(filepath.Ext(file))])
	// Build parent file node
	fnode := &xmlquery.Node{
		Data:         "file",
		Prefix:       cor.fileGrp.Prefix,
		NamespaceURI: cor.fileGrp.NamespaceURI,
	}
	node.SetAttr(fnode, xml.Attr{
		Name:  xml.Name{Local: "MIMETYPE"},
		Value: pagexml.MIMEType,
	})
	node.SetAttr(fnode, xml.Attr{
		Name:  xml.Name{Local: "ID"},
		Value: fmt.Sprintf("%s-%s", cor.ofg, fileid),
	})
	// Build child FLocat node.
	flocat := &xmlquery.Node{
		Data:         "FLocat",
		Prefix:       cor.fileGrp.Prefix,
		NamespaceURI: cor.fileGrp.NamespaceURI,
	}
	node.SetAttr(flocat, xml.Attr{
		Name:  xml.Name{Local: "LOCTYPE"},
		Value: "OTHER",
	})
	node.SetAttr(flocat, xml.Attr{
		Name:  xml.Name{Local: "OTHERLOCTYPE"},
		Value: "FILE",
	})
	node.SetAttr(flocat, xml.Attr{
		Name:  xml.Name{Local: "href", Space: "xlink"},
		Value: filepath.Join(cor.ofg, filepath.Base(file)),
	})
	// Add nodes to the tree.
	node.AppendChild(fnode, flocat)
	node.AppendChild(cor.fileGrp, fnode)
}

func writeXML(doc *xmlquery.Node, path string) (err error) {
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("writeXML %s: %v", path, err)
	}
	defer func() {
		if exx := out.Close(); exx != nil && err == nil {
			err = exx
		}
	}()
	in := strings.NewReader(doc.OutputXML(false))
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("writeXML %s: %v", path, err)
	}
	return nil
}

func trim(str string) string {
	return strings.TrimFunc(str, func(r rune) bool {
		return unicode.IsPunct(r)
	})
}

func notEmpty(str string) string {
	if len(str) == 0 {
		return "\u03b5" // small letter epsilon
	}
	return str
}

func short(str string) bool {
	return utf8.RuneCountInString(str) <= 3
}
