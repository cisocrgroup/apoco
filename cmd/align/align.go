package align

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/align"
	"git.sr.ht/~flobar/apoco/pkg/apoco/mets"
	"git.sr.ht/~flobar/apoco/pkg/apoco/node"
	"git.sr.ht/~flobar/apoco/pkg/apoco/pagexml"
	"github.com/antchfx/xmlquery"
	"github.com/spf13/cobra"
)

var flags = struct {
	ifgs      []string
	ofg, mets string
}{}

// CMD defines the apoco align command.
var CMD = &cobra.Command{
	Use:   "align",
	Short: "Align multiple input file groups",
	Run:   run,
}

func init() {
	CMD.Flags().StringVarP(&flags.ofg, "out-file-grp", "O", "", "set output file group of alignments")
	CMD.Flags().StringVarP(&flags.mets, "mets", "m", "mets.xml", "set path to mets file")
	CMD.Flags().StringSliceVarP(&flags.ifgs, "input-file-grp", "I", nil, "set input file groups")
}

func run(_ *cobra.Command, args []string) {
	chk(alignFiles(flags.mets, flags.ofg, flags.ifgs))
}

type file struct {
	path, id string
}

func getPaths(m mets.METS, ifgs []string) ([][]file, error) {
	// Append sorted list of files in the filegroups.
	var tmp [][]file
	for _, ifg := range ifgs {
		flocats := m.FindFlocats(ifg)
		files := make([]file, len(flocats))
		for i := range flocats {
			id, _ := node.LookupAttr(flocats[i].Parent, xml.Name{Local: "ID"})
			files[i] = file{
				path: m.FlocatGetPath(flocats[i]),
				id:   id,
			}
		}
		sort.Slice(files, func(i, j int) bool {
			return filepath.Base(files[i].path) < filepath.Base(files[j].path)
		})
		tmp = append(tmp, files)
	}
	// Check that we have the same number of files for each input
	// file group.
	var n int
	for i, paths := range tmp {
		if i == 0 {
			n = len(paths)
		}
		if len(paths) != n {
			return nil, fmt.Errorf("cannot align files")
		}
	}
	// Transpose the temporary array and return it.
	ret := make([][]file, n)
	for i := range ret {
		ret[i] = make([]file, len(ifgs))
	}
	for i := range tmp {
		for j := range tmp[i] {
			ret[j][i] = tmp[i][j]
		}
	}
	return ret, nil
}

const agent = "apoco align " + internal.Version

func alignFiles(mpath, ofg string, ifgs []string) error {
	m, fg, err := readMETS(mpath, ofg)
	if err != nil {
		return err
	}
	files, err := getPaths(m, ifgs)
	if err != nil {
		return err
	}
	for i := range files {
		doc, err := alignFile(files[i])
		if err != nil {
			return err
		}
		opath := addFileToMETS(m, fg, ofg, files[i][0])
		if err := writeToWS(doc, mpath, ofg, opath); err != nil {
			return err
		}
	}
	if err := m.AddAgent(internal.PStep, agent); err != nil {
		return err
	}
	return m.Write()
}

func alignFile(files []file) (*xmlquery.Node, error) {
	lines, err := alignLines(files)
	if err != nil {
		return nil, err
	}
	for i := range lines {
		if err := alignWords(lines[i]); err != nil {
			return nil, err
		}
	}
	return root(lines[0][0].node), nil
}

func alignLines(files []file) ([][]region, error) {
	// Load xml files from paths.
	docs := make([]*xmlquery.Node, len(files))
	for i, f := range files {
		doc, err := readXML(f.path)
		if err != nil {
			return nil, err
		}
		docs[i] = doc
	}
	// Read lines from documents nodes.
	lines := make([][]region, len(files))
	var n int
	for i, node := range docs {
		tmp, err := getLines(node)
		if err != nil {
			return nil, err
		}
		if i == 0 {
			n = len(tmp)
		}
		if len(tmp) != n {
			return nil, fmt.Errorf("cannot align lines")
		}
		lines[i] = tmp
	}
	// Transpose lines
	linesT := make([][]region, n)
	for i := range linesT {
		linesT[i] = make([]region, len(files))
	}
	for i := range lines {
		for j := range lines[i] {
			linesT[j][i] = lines[i][j]
		}
	}
	return linesT, nil
}

func alignWords(lines []region) error {
	if len(lines) == 0 {
		return fmt.Errorf("cannot align words: empty")
	}
	lines[0].prepareForAlignment()
	for i := 1; i < len(lines); i++ {
		if err := lines[0].alignWith(lines[i]); err != nil {
			return err
		}
	}
	return nil
}

type region struct {
	node       *xmlquery.Node
	text       []rune
	subregions []region
	unicodes   []*xmlquery.Node
}

func getLines(doc *xmlquery.Node) ([]region, error) {
	lines, err := xmlquery.QueryAll(doc, "//*[local-name()='TextLine']")
	if err != nil {
		return nil, err
	}
	sort.Slice(lines, func(i, j int) bool {
		iid, _ := node.LookupAttr(lines[i], xml.Name{Local: "id"})
		jid, _ := node.LookupAttr(lines[j], xml.Name{Local: "id"})
		return iid < jid
	})
	var ret []region
	for _, node := range lines {
		line, err := newLine(node)
		if err != nil {
			return nil, err
		}
		ret = append(ret, line)
	}
	return ret, nil
}

func newLine(r *xmlquery.Node) (region, error) {
	unicodes := pagexml.FindUnicodesInRegionSorted(r)
	if len(unicodes) == 0 {
		return region{}, fmt.Errorf("missing unicode for line")
	}
	words, err := getWords(r)
	if err != nil {
		return region{}, err
	}
	return region{
		node:       r,
		text:       []rune(node.Data(node.FirstChild(unicodes[0]))),
		subregions: words,
		unicodes:   unicodes,
	}, nil
}

func getWords(node *xmlquery.Node) ([]region, error) {
	words, err := xmlquery.QueryAll(node, "./*[local-name()='Word']")
	if err != nil {
		return nil, err
	}
	var ret []region
	for _, node := range words {
		unicodes := pagexml.FindUnicodesInRegionSorted(node)
		if len(unicodes) == 0 {
			continue
		}
		text := []rune(unicodes[0].FirstChild.Data)
		ret = append(ret, region{
			node:     node,
			text:     text,
			unicodes: unicodes,
		})
	}
	return ret, nil
}

func (r *region) id() string {
	id, _ := node.LookupAttr(r.node, xml.Name{Local: "id"})
	return id
}

func (r *region) prepareForAlignment() {
	// Delete all of r's text equivs but the first one and set the
	// index to 1.
	for i := 1; i < len(r.unicodes); i++ {
		node.Delete(r.unicodes[i].Parent)
	}
	r.unicodes = r.unicodes[:1]
	node.SetAttr(r.unicodes[0].Parent, xml.Attr{
		Name:  xml.Name{Local: "index"},
		Value: "1",
	})
	// Do the same recursively for all its subregions.
	for i := range r.subregions {
		r.subregions[i].prepareForAlignment()
	}
}

func (r *region) alignWith(o region) error {
	// Both aligned lines need to have the same ids (to
	// make sure that we are really aligning the right lines with
	// each other).
	if r.id() != o.id() {
		return fmt.Errorf("cannot align line id %s with line id %s", r.id(), o.id())
	}
	// Both vars r and o are supposed to be lines.  Words are
	// aligned below r's word nodes using r as primary alignment
	// line.
	pstr, pepos := r.eposMap()
	sstr, sepos := o.eposMap()
	pos := align.Do(pstr, sstr)
	for i := range pos {
		// Since we align two things, len(pos[i]) = 2.
		pi := pepos[pos[i][0].E]
		si := sepos[pos[i][1].E]
		text := string(pos[i][1].Slice())
		var b int
		if i > 0 {
			b = sepos[pos[i-1][1].E]
		}
		for len(r.subregions) <= pi {
			r.appendEmptyWord()
		}
		r.subregions[pi].appendTextEquiv(text, o.subregions[b:si+1]...)
	}
	// Append the secondary line to r.
	r.appendTextEquiv(string(sstr), o)
	return nil
}

// eposMap concatenates the subregions of a region to a string
// (separated by ' ') and returns the end positions of the subregion
// indices as a map epos -> index.
func (r *region) eposMap() ([]rune, map[int]int) {
	var epos int
	var str []rune
	pmap := make(map[int]int, len(r.subregions))
	for i, sr := range r.subregions {
		if i > 0 {
			str = append(str, ' ')
			epos++
		}
		str = append(str, sr.text...)
		epos += len(sr.text)
		pmap[epos] = i
	}
	return str, pmap
}

func (r *region) appendEmptyWord() {
	w := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "Word",
		Prefix:       r.node.Prefix,
		NamespaceURI: r.node.NamespaceURI,
	}
	te := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "TextEquiv",
		Prefix:       r.node.Prefix,
		NamespaceURI: r.node.NamespaceURI,
	}
	node.SetAttr(te, xml.Attr{
		Name:  xml.Name{Local: "index"},
		Value: strconv.Itoa(len(r.subregions) + 1),
	})
	node.SetAttr(te, xml.Attr{
		Name:  xml.Name{Local: "conf"},
		Value: "0",
	})
	u := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "Unicode",
		Prefix:       r.node.Prefix,
		NamespaceURI: r.node.NamespaceURI,
	}
	t := &xmlquery.Node{
		Type: xmlquery.TextNode,
		Data: "",
	}
	node.AppendChild(u, t)
	node.AppendChild(te, u)
	node.AppendChild(w, te)
	node.AppendChild(r.node, w)
	r.subregions = append(r.subregions, region{
		node:     w,
		unicodes: []*xmlquery.Node{u},
	})
}

func (r *region) appendTextEquiv(text string, others ...region) {
	sum := 0.0
	for _, other := range others {
		conf, _ := node.LookupAttrAsFloat(other.unicodes[0].Parent, xml.Name{Local: "conf"})
		sum += conf
	}
	te := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "TextEquiv",
		Prefix:       r.unicodes[0].Parent.Prefix,
		NamespaceURI: r.unicodes[0].Parent.NamespaceURI,
	}
	u := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "Unicode",
		Prefix:       r.unicodes[0].Prefix,
		NamespaceURI: r.unicodes[0].NamespaceURI,
	}
	t := &xmlquery.Node{
		Type: xmlquery.TextNode,
		Data: text,
	}
	node.AppendChild(u, t)
	node.AppendChild(te, u)
	r.unicodes = append(r.unicodes, u)
	node.SetAttr(te, xml.Attr{
		Name:  xml.Name{Local: "index"},
		Value: strconv.Itoa(len(r.unicodes)),
	})
	node.SetAttr(te, xml.Attr{
		Name:  xml.Name{Local: "conf"},
		Value: strconv.FormatFloat(sum/float64(len(others)), 'E', 4, 64),
	})
	node.AppendChild(r.unicodes[0].Parent.Parent, te)
}

func readMETS(name, ofg string) (mets.METS, *xmlquery.Node, error) {
	m, err := mets.Open(name)
	if err != nil {
		return mets.METS{}, nil, err
	}
	// Check if the given file group already exists and overwrite
	// it if it already exists.
	expr := fmt.Sprintf("//*[local-name()='fileGrp'][@USE=%q]", ofg)
	existing := xmlquery.FindOne(m.Root, expr)
	if existing != nil {
		// Delete all children.
		existing.FirstChild = nil
		existing.LastChild = nil
		return m, existing, nil
	}
	// The given output file group does not yet exist. Add a new
	// filegroup entry.
	fileGrps := xmlquery.Find(m.Root, "//*[local-name()='fileGrp']")
	if len(fileGrps) == 0 {
		return mets.METS{}, nil, fmt.Errorf("missing file grp in %s", m.Name)
	}
	fileGrp := &xmlquery.Node{
		Data:         "fileGrp",
		Prefix:       fileGrps[0].Prefix,
		NamespaceURI: fileGrps[0].NamespaceURI,
	}
	node.SetAttr(fileGrp, xml.Attr{
		Name:  xml.Name{Local: "USE"},
		Value: ofg,
	})
	node.PrependSibling(fileGrps[0], fileGrp)
	return m, fileGrp, nil
}

func addFileToMETS(m mets.METS, fg *xmlquery.Node, ofg string, f file) string {
	newID := internal.IDFromFilePath(f.path, ofg)
	filePath := newID + ".xml"
	// Build parent file node
	fnode := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "file",
		Prefix:       fg.Prefix,
		NamespaceURI: fg.NamespaceURI,
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
		Prefix:       fg.Prefix,
		NamespaceURI: fg.NamespaceURI,
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
		Value: filepath.Join(ofg, filePath),
	})
	// Add nodes to the tree.
	node.AppendChild(fnode, flocat)
	node.AppendChild(fg, fnode)
	addFileToStructMap(m, f.id, newID)
	return filePath
}

func addFileToStructMap(m mets.METS, id, newID string) {
	// Check if the according fptr already exists and skip
	// inserting a fptr already exists.
	fptr := m.FindFptr(newID)
	if fptr != nil {
		return
	}
	// Find fptr for the aligned id and append the new id.
	fptr = m.FindFptr(id)
	if fptr == nil {
		apoco.L("[warning] cannot find fptr for %s", id)
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

func writeToWS(doc *xmlquery.Node, mets, ofg, path string) error {
	dir := filepath.Join(filepath.Dir(mets), ofg)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}
	return ioutil.WriteFile(
		filepath.Join(dir, filepath.Base(path)),
		[]byte(doc.OutputXML(false)),
		0666,
	)
}

func readXML(path string) (*xmlquery.Node, error) {
	in, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	doc, err := xmlquery.Parse(in)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func root(n *xmlquery.Node) *xmlquery.Node {
	for n.Parent != nil {
		n = n.Parent
	}
	return n
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
