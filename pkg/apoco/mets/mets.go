package mets

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"git.sr.ht/~flobar/apoco/pkg/apoco/node"
	"github.com/antchfx/xmlquery"
)

// METS represents an METS file.
type METS struct {
	Root *xmlquery.Node // Document root node.
	Name string         // File name.
}

// Open opens a mets xml file.
func Open(name string) (METS, error) {
	is, err := os.Open(name)
	if err != nil {
		return METS{}, fmt.Errorf("open mets file %s: %v", name, err)
	}
	defer is.Close()
	root, err := xmlquery.Parse(is)
	if err != nil {
		return METS{}, fmt.Errorf("open mets file %s: %v", name, err)
	}
	return METS{Root: root, Name: name}, nil
}

// Write writes the xml file.
func (mets METS) Write() error {
	return ioutil.WriteFile(mets.Name, []byte(node.PrettyPrint(mets.Root, "", "  ")), 0666)
}

// AddAgent adds an agent to the metsHdr of the mets tree.
func (mets METS) AddAgent(pstep, agent string) error {
	// Check if the according agent is already registered.
	if mets.checkAgent(pstep, agent) {
		return nil
	}
	// Get metsHdr node or create it if it does not exist, yet.
	hdr := xmlquery.FindOne(mets.Root, "/*[local-name()='mets']/*[local-name()='metsHdr']")
	if hdr == nil {
		var err error
		hdr, err = mets.addHdr()
		if err != nil {
			return err
		}
	}
	// Create the new agent node.
	agentnode := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "agent",
		Prefix:       hdr.Prefix,
		NamespaceURI: hdr.NamespaceURI,
	}
	node.SetAttr(agentnode, xml.Attr{Name: xml.Name{Local: "TYPE"}, Value: "OTHER"})
	node.SetAttr(agentnode, xml.Attr{Name: xml.Name{Local: "OTHERTYPE"}, Value: "SOFTWARE"})
	node.SetAttr(agentnode, xml.Attr{Name: xml.Name{Local: "ROLE"}, Value: "OTHER"})
	node.SetAttr(agentnode, xml.Attr{Name: xml.Name{Local: "OTHERROLE"}, Value: pstep})
	name := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "name",
		Prefix:       hdr.Prefix,
		NamespaceURI: hdr.NamespaceURI,
	}
	node.AppendChild(name, &xmlquery.Node{Type: xmlquery.TextNode, Data: agent})
	node.AppendChild(agentnode, name)
	node.AppendChild(hdr, agentnode)
	return nil
}

func (mets METS) addHdr() (*xmlquery.Node, error) {
	root := xmlquery.FindOne(mets.Root, "/*[local-name()='mets']")
	if root == nil {
		return nil, fmt.Errorf("invalid mets: missing /mets root node")
	}
	hdr := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "metsHdr",
		Prefix:       root.Prefix,
		NamespaceURI: root.NamespaceURI,
	}
	node.SetAttr(hdr, xml.Attr{
		Name:  xml.Name{Local: "CREATEDATE"},
		Value: time.Now().Format(time.RFC3339),
	})
	node.PrependChild(root, hdr)
	return hdr, nil
}

// checkAgent checks for an existing agent entry in the header.
func (mets METS) checkAgent(pstep, agent string) bool {
	expr := fmt.Sprintf("/*[local-name()='mets']/*[local-name()='metsHdr']"+
		"/*[local-name()='agent'][@OTHERROLE=%q]", pstep)
	agents := xmlquery.Find(mets.Root, expr)
	for _, agentnode := range agents {
		for c := agentnode.FirstChild; c != nil; c = c.NextSibling {
			if node.Data(node.FirstChild(c)) == agent {
				return true
			}
		}
	}
	return false
}

// FindFlocats returns the Flocat nodes for the given file group.
func (mets METS) FindFlocats(fg string) []*xmlquery.Node {
	expr := fmt.Sprintf("/*[local-name()='mets']/*[local-name()='fileSec']"+
		"/*[local-name()='fileGrp'][@USE=%q]/*[local-name()='file']"+
		"/*[local-name()='FLocat']", fg)
	return xmlquery.Find(mets.Root, expr)
}

// FlocatGetPath returns the path of the flocat's link relative to the
// given mets file's base directory unless the stored path is absolute.
func (mets METS) FlocatGetPath(n *xmlquery.Node) string {
	link, _ := node.LookupAttr(n, xml.Name{Space: "xlink", Local: "href"})
	if filepath.IsAbs(link) {
		return link
	}
	return filepath.Join(filepath.Dir(mets.Name), link)
}

// FindFptr returns the Fptr node for the given (unique) id.
func (mets METS) FindFptr(id string) *xmlquery.Node {
	expr := fmt.Sprintf("/*[local-name()='mets']/*[local-name()='structMap']"+
		"/*[local-name()='div']/*[local-name()='div']"+
		"/*[local-name()='fptr'][@FILEID=%q]", id)
	return xmlquery.FindOne(mets.Root, expr)
}

// FilePathsForFileGrp returns the list of file paths for the given
// file group.  The returned file paths are updated to be relative to
// the mets's file base directory if they are not absolute.
func (mets METS) FilePathsForFileGrp(fg string) ([]string, error) {
	nodes := findFileGrpFLocatFromRoot(mets.Root, fg)
	ret := make([]string, len(nodes))
	for i, n := range nodes {
		link, ok := node.LookupAttr(n, xml.Name{Space: "xlink", Local: "href"})
		if !ok {
			return nil, fmt.Errorf("filePathsForFileGrp %s: missing href attribute", mets.Name)
		}
		if filepath.IsAbs(link) {
			ret[i] = link
		} else {
			ret[i] = filepath.Join(filepath.Dir(mets.Name), link)
		}
	}
	return ret, nil
}

func findFileGrpFLocatFromRoot(doc *xmlquery.Node, fg string) []*xmlquery.Node {
	expr := fmt.Sprintf("/*[local-name()='mets']/*[local-name()='fileSec']"+
		"/*[local-name()='fileGrp'][@USE=%q]/*[local-name()='file']"+
		"/*[local-name()='FLocat']", fg)
	return xmlquery.Find(doc, expr)
}
