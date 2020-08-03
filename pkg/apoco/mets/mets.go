package mets

import (
	"encoding/xml"
	"fmt"
	"path/filepath"
	"time"

	"git.sr.ht/~flobar/apoco/pkg/apoco/node"
	"github.com/antchfx/xmlquery"
)

// AddAgent adds an agent to the metsHdr of the mets tree.
func AddAgent(mets *xmlquery.Node, pstep, processor, version string) error {
	// Get metsHdr node or create it if it does not exist, yet.
	hdr := xmlquery.FindOne(mets, "/*[local-name()='mets']/*[local-name()='metsHdr']")
	if hdr == nil {
		var err error
		hdr, err = addHdr(mets)
		if err != nil {
			return err
		}
	}
	// Create the new agent node.
	agent := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "agent",
		Prefix:       hdr.Prefix,
		NamespaceURI: hdr.NamespaceURI,
	}
	node.SetAttr(agent, xml.Attr{Name: xml.Name{Local: "TYPE"}, Value: "OTHER"})
	node.SetAttr(agent, xml.Attr{Name: xml.Name{Local: "OTHERTYPE"}, Value: "SOFTWARE"})
	node.SetAttr(agent, xml.Attr{Name: xml.Name{Local: "ROLE"}, Value: "OTHER"})
	node.SetAttr(agent, xml.Attr{Name: xml.Name{Local: "OTHERROLE"}, Value: pstep})
	name := &xmlquery.Node{
		Type:         xmlquery.ElementNode,
		Data:         "name",
		Prefix:       hdr.Prefix,
		NamespaceURI: hdr.NamespaceURI,
	}
	node.AppendChild(name, &xmlquery.Node{Type: xmlquery.TextNode, Data: processor + " " + version})
	node.AppendChild(agent, name)
	node.AppendChild(hdr, agent)
	return nil
}

func addHdr(mets *xmlquery.Node) (*xmlquery.Node, error) {
	root := xmlquery.FindOne(mets, "/*[local-name()='mets']")
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

// FindFlocats returns the Flocat nodes for the given file group.
func FindFlocats(doc *xmlquery.Node, fg string) []*xmlquery.Node {
	expr := fmt.Sprintf("/*[local-name()='mets']/*[local-name()='fileSec']"+
		"/*[local-name()='fileGrp'][@USE=%q]/*[local-name()='file']"+
		"/*[local-name()='FLocat']", fg)
	return xmlquery.Find(doc, expr)
}

// FlocatGetPath returns the path of the flocat's link relative to the
// given mets file's base directory.
func FlocatGetPath(flocat *xmlquery.Node, metsPath string) string {
	link, _ := node.LookupAttr(flocat, xml.Name{Space: "xlink", Local: "href"})
	return filepath.Join(filepath.Dir(metsPath), link)
}
