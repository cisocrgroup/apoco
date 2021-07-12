package correct

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/mets"
	"git.sr.ht/~flobar/apoco/pkg/apoco/node"
	"git.sr.ht/~flobar/apoco/pkg/apoco/pagexml"
	"github.com/antchfx/xmlquery"
)

type corrector struct {
	stoks   stokMap
	ofg     string
	ifgs    []string
	fileGrp *xmlquery.Node
	mets    mets.METS
}

func (cor *corrector) correct(metsName string) error {
	if err := cor.readMETS(metsName); err != nil {
		return fmt.Errorf("correct: %v", err)
	}
	for _, ifg := range cor.ifgs {
		files, err := cor.mets.FilePathsForFileGrp(ifg)
		if err != nil {
			return fmt.Errorf("correct: %v", err)
		}
		for _, file := range files {
			if err := cor.correctFile(file, ifg); err != nil {
				return fmt.Errorf("correct: %v", err)
			}
		}
	}
	if err := cor.mets.Write(); err != nil {
		return fmt.Errorf("correct: %v", err)
	}
	return nil
}

func (cor *corrector) correctFile(file, ifg string) error {
	apoco.Log("correcting file %q in input file group %q", file, ifg)
	is, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("writeCorrections: %v", err)
	}
	defer is.Close()
	doc, err := xmlquery.Parse(is)
	if err != nil {
		return fmt.Errorf("writeCorrections: %v", err)
	}
	// Set correction to Word nodes.
	words := xmlquery.Find(doc, "//*[local-name()='Word']")
	for _, word := range words {
		if err := cor.correctWord(word, file); err != nil {
			return fmt.Errorf("correct %s: %v", file, err)
		}
	}
	// Use corrected words to write new lines.
	lines := xmlquery.Find(doc, "//*[local-name()='TextLine']")
	for _, line := range lines {
		words := gatherUnicodes(line, "./*[local-name()='Word']/*[local-name()='TextEquiv']/*[local-name()='Unicode']")
		resetTextEquiv(line, strings.Join(words, " "))
	}
	// Use corrected lines to write new regions.
	regions := xmlquery.Find(doc, "//*[local-name()='TextRegion']")
	for _, region := range regions {
		lines := gatherUnicodes(region, "./*[local-name()='TextLine']/*[local-name()='TextEquiv']/*[local-name()='Unicode']")
		resetTextEquiv(region, strings.Join(lines, "\n"))
	}
	if err := cor.write(doc, file, ifg); err != nil {
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
	newTE := cor.makeTextEquiv(unicodes[0].Parent)
	newU := newUnicode(unicodes[0].Parent, "")
	ocr := node.Data(unicodes[0].FirstChild)

	info := cor.stoks[file][id]
	// Just skip words that we do not have any info about.
	if info == nil {
		return nil
	}
	prefix := idFromFilePath(file, cor.ofg)
	info.ID = prefix + "_" + info.ID // Prefix the id with the output file group and the file name.
	if info.Skipped {
		newU.FirstChild.Data = ocr
		node.SetAttr(newTE, xml.Attr{
			Name:  xml.Name{Local: "dataTypeDetails"},
			Value: info.String(),
		})
	} else {
		if info.Cor {
			newU.FirstChild.Data = apoco.ApplyOCRToCorrection(ocr, info.Sug)
		} else {
			newU.FirstChild.Data = ocr
		}
		node.SetAttr(newTE, xml.Attr{
			Name:  xml.Name{Local: "conf"},
			Value: strconv.FormatFloat(info.Conf, 'e', -1, 64),
		})
		node.SetAttr(newTE, xml.Attr{
			Name:  xml.Name{Local: "dataTypeDetails"},
			Value: info.String(),
		})
	}
	newTE.FirstChild = newU
	newU.Parent = newTE
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

func (cor *corrector) makeTextEquiv(p *xmlquery.Node) *xmlquery.Node {
	newTE := &xmlquery.Node{ // TextEquiv
		Type:         xmlquery.ElementNode,
		Data:         p.Data,
		Prefix:       p.Prefix,
		NamespaceURI: p.NamespaceURI,
	}
	node.SetAttr(newTE, xml.Attr{
		Name:  xml.Name{Local: "index"},
		Value: "1",
	})
	conf, _ := node.LookupAttr(p, xml.Name{Local: "conf"})
	node.SetAttr(newTE, xml.Attr{
		Name:  xml.Name{Local: "conf"},
		Value: conf,
	})
	node.SetAttr(newTE, xml.Attr{
		Name:  xml.Name{Local: "dataType"},
		Value: "OCR-D-CIS-POST-CORRECTION",
	})
	return newTE
}

func (cor *corrector) write(doc *xmlquery.Node, file, ifg string) error {
	pagexml.SetMetadata(doc, agent, time.Now(), time.Now())
	ofile := cor.addFileToFileGrp(file, ifg)
	dir := filepath.Join(filepath.Dir(cor.mets.Name), cor.ofg)
	ofile = filepath.Join(dir, ofile)
	_ = os.MkdirAll(dir, 0777)
	xmlData := node.PrettyPrint(doc, "", "  ") // doc.OutputXML(false)
	// xmlData = strings.ReplaceAll(xmlData, "><", ">\n<")
	return ioutil.WriteFile(ofile, []byte(xmlData), 0666)
}

const agent = "ocrd/cis/apoco-correct " + internal.Version

func (cor *corrector) readMETS(name string) error {
	m, err := mets.Open(name)
	if err != nil {
		return fmt.Errorf("readMETS %s: %v", name, err)
	}
	cor.mets = m
	// Update agent in mets header file.
	if err := cor.mets.AddAgent(internal.PStep, agent); err != nil {
		return fmt.Errorf("readMETS: %v", err)
	}
	// Check if the given file group already exists and overwrite it.
	existing := xmlquery.FindOne(cor.mets.Root, fmt.Sprintf("//*[local-name()='fileGrp'][@USE=%q]", cor.ofg))
	if existing != nil {
		// Delete all children.
		existing.FirstChild = nil
		existing.LastChild = nil
		cor.fileGrp = existing
		return nil
	}
	// Add a new filegroup entry.
	fileGrps := xmlquery.Find(cor.mets.Root, "//*[local-name()='fileGrp']")
	if len(fileGrps) == 0 {
		return fmt.Errorf("readMETS %s: missing file grp", cor.mets.Name)
	}
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

func (cor *corrector) addFileToFileGrp(file, ifg string) string {
	newID := idFromFilePath(file, cor.ofg)
	filePath := newID + ".xml"
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
		Value: newID,
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
		Value: filepath.Join(cor.ofg, filePath),
	})
	// Add nodes to the tree.
	node.AppendChild(fnode, flocat)
	node.AppendChild(cor.fileGrp, fnode)
	cor.addFileToStructMap(file, newID, ifg)
	return filePath
}

// <mets:structMap TYPE="PHYSICAL">
//     <mets:div TYPE="physSequence" ID="physroot">
//       <mets:div TYPE="page" ORDER="1" ID="phys_0001" DMDID="DMDGT_0001">
//         <mets:fptr FILEID="OCR-D-GT-SEG-PAGE_0001"/>
//         <mets:fptr FILEID="OCR-D-GT-SEG-BLOCK_0001"/>
//         <mets:fptr FILEID="OCR-D-GT-SEG-LINE_0001"/>
//         <mets:fptr FILEID="OCR-D-IMG_0001"/>
func (cor *corrector) addFileToStructMap(path, newID, ifg string) {
	// Check if the according new id already exists.
	fptr := cor.mets.FindFptr(newID)
	if fptr != nil {
		return
	}
	// Search for the flocat with the according file path and use
	// its id.
	flocats := cor.mets.FindFlocats(ifg)
	var oldID string
	for _, flocat := range flocats {
		if filepath.Base(path) == filepath.Base(cor.mets.FlocatGetPath(flocat)) {
			oldID, _ = node.LookupAttr(flocat.Parent, xml.Name{Local: "ID"})
			break
		}
	}
	fptr = cor.mets.FindFptr(oldID)
	if fptr == nil {
		apoco.Log("[warning] cannot find fptr for %s", oldID)
		return
	}
	newFptr := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "fptr",
		Prefix:       fptr.Prefix,
		NamespaceURI: fptr.NamespaceURI,
	}
	node.SetAttr(newFptr, xml.Attr{
		Name:  xml.Name{Local: "FILEID"},
		Value: newID,
	})
	node.AppendChild(fptr.Parent, newFptr)
}

func newUnicode(p *xmlquery.Node, data string) *xmlquery.Node {
	unicode := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "Unicode",
		Prefix:       p.Prefix,
		NamespaceURI: p.NamespaceURI,
	}
	text := &xmlquery.Node{Type: xmlquery.TextNode, Data: data}
	node.AppendChild(unicode, text)
	return unicode
}

func gatherUnicodes(p *xmlquery.Node, expr string) []string {
	unicodes := xmlquery.Find(p, expr)
	var ret []string
	for _, u := range unicodes {
		ret = append(ret, u.FirstChild.Data)
	}
	return ret
}

func resetTextEquiv(p *xmlquery.Node, data string) {
	// Delete old TextEquiv nodes.
	tes := xmlquery.Find(p, "./*[local-name()='TextEquiv']")
	for _, te := range tes {
		node.Delete(te)
	}
	// Create new TextEquiv/Unicode/Text node
	te := &xmlquery.Node{ // TextEquiv
		Type:         xmlquery.ElementNode,
		Data:         "TextEquiv",
		Prefix:       p.Prefix,
		NamespaceURI: p.NamespaceURI,
	}
	node.SetAttr(te, xml.Attr{
		Name:  xml.Name{Local: "index"},
		Value: "1",
	})
	unicode := newUnicode(te, data)
	node.AppendChild(te, unicode)
	node.AppendChild(p, te)
}

// idFromFilePath generates an id based on the file group and the file path.
func idFromFilePath(path, fg string) string {
	// Use base path and remove file extensions.
	path = filepath.Base(path)
	path = path[0 : len(path)-len(filepath.Ext(path))]
	// Split everything after the last `_`.
	splits := strings.Split(path, "_")
	return fmt.Sprintf("%s_%s", fg, splits[len(splits)-1])
}
