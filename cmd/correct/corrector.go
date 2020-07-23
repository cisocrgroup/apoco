package correct

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/mets"
	"git.sr.ht/~flobar/apoco/pkg/apoco/node"
	"git.sr.ht/~flobar/apoco/pkg/apoco/pagexml"
	"github.com/antchfx/xmlquery"
)

type corrector struct {
	info           infoMap
	mets, ofg, ifg string
	doc, fileGrp   *xmlquery.Node
	protocol       bool
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
	if err := mets.AddAgent(cor.doc, "recognition/post-correction", "apoco correct", internal.Version); err != nil {
		return fmt.Errorf("correct: %v", err)
	}
	if err := ioutil.WriteFile(cor.mets, []byte(cor.doc.OutputXML(false)), 0666); err != nil {
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
	unicodes := pagexml.FindUnicodesInRegionSorted(word)
	if len(unicodes) == 0 {
		return nil
	}
	newTE := cor.makeTextEquiv(unicodes)
	newU := cor.makeUnicode(unicodes)
	newStr := &xmlquery.Node{Type: xmlquery.TextNode}
	ocr := node.Data(unicodes[0].FirstChild)

	info := cor.info[file][id]
	// Just skip words that we do not have any info about.
	if info == nil {
		return nil
	}
	if info.skipped {
		newStr.Data = ocr
		if cor.protocol {
			node.SetAttr(newTE, xml.Attr{
				Name:  xml.Name{Local: "dataTypeDetails"},
				Value: info.String(),
			})
		}
	} else {
		if info.cor {
			newStr.Data = apoco.ApplyOCRToCorrection(ocr, info.sug)
		} else {
			newStr.Data = ocr
		}
		node.SetAttr(newTE, xml.Attr{
			Name:  xml.Name{Local: "conf"},
			Value: strconv.FormatFloat(info.conf, 'e', -1, 64),
		})
		if cor.protocol {
			node.SetAttr(newTE, xml.Attr{
				Name:  xml.Name{Local: "dataTypeDetails"},
				Value: info.String(),
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
	newTE := &xmlquery.Node{ // TextEquiv
		Type:         xmlquery.ElementNode,
		Data:         unicodes[0].Parent.Data,
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
	return ioutil.WriteFile(ofile, []byte(doc.OutputXML(false)), 0666)
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
		Type:         xmlquery.ElementNode,
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
		Value: fmt.Sprintf("%s_%s", cor.ofg, fileid),
	})
	// Build child FLocat node.
	flocat := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
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
