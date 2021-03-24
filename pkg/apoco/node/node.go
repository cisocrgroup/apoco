// Package node provides helper functions to work with queryxml.Node
// pointers.  All functions explicitly handle nil nodes and therefore
// allow for deep nesting of these function calls.
package node

import (
	"encoding/xml"
	"strconv"

	"github.com/antchfx/xmlquery"
)

// Data returns the data for the node or the empty string if the node
// in nil.
func Data(node *xmlquery.Node) string {
	if node == nil {
		return ""
	}
	return node.Data
}

// FirstChild returns the first child of the node or nil if the node
// is nil.
func FirstChild(node *xmlquery.Node) *xmlquery.Node {
	if node == nil {
		return nil
	}
	return node.FirstChild
}

// Parent returns the node's parent or nil if the node is nil.
func Parent(node *xmlquery.Node) *xmlquery.Node {
	if node == nil {
		return nil
	}
	return node.Parent
}

// QueryAll calls xmlquery.QueryAll.  If the given node is nil, an
// empty node list is returned.
func QueryAll(node *xmlquery.Node, expr string) ([]*xmlquery.Node, error) {
	if node == nil {
		return nil, nil
	}
	return xmlquery.QueryAll(node, expr)
}

// Query calls xmlquery.Query.  If the given node is nil, an
// emtpy node list is returned.
func Query(node *xmlquery.Node, expr string) (*xmlquery.Node, error) {
	if node == nil {
		return nil, nil
	}
	return xmlquery.Query(node, expr)
}

// AppendChild appends the child c to the parent p. Both given nodes
// must not be null.
func AppendChild(p, c *xmlquery.Node) {
	if p.FirstChild == nil || p.LastChild == nil {
		p.FirstChild = c
		p.LastChild = c
		c.Parent = p
		c.PrevSibling = nil
		c.NextSibling = nil
		return
	}
	c.PrevSibling = p.LastChild
	c.NextSibling = nil
	c.Parent = p
	p.LastChild.NextSibling = c
	p.LastChild = c
}

// PrependChild prepends the child c to the parent p. Both given nodes
// must not be null.
func PrependChild(p, c *xmlquery.Node) {
	if p.FirstChild == nil || p.LastChild == nil {
		p.FirstChild = c
		p.LastChild = c
		c.Parent = p
		c.PrevSibling = nil
		c.NextSibling = nil
		return
	}
	c.NextSibling = p.FirstChild
	c.PrevSibling = nil
	c.Parent = p
	p.FirstChild.PrevSibling = c
	p.FirstChild = c
}

// PrependSibling prepends to the node n a new sibling s. Both given
// nodes must not be null.
func PrependSibling(n, s *xmlquery.Node) {
	if n.PrevSibling == nil {
		n.Parent.FirstChild = s
		s.Parent = n.Parent
		s.PrevSibling = nil
		s.NextSibling = n
		n.PrevSibling = s
		return
	}
	n.PrevSibling.NextSibling = s
	s.Parent = n.Parent
	s.PrevSibling = n.PrevSibling
	s.NextSibling = n
	n.PrevSibling = s
}

// Delete removes the given node from its tree.  The given node must
// not be nil.
func Delete(n *xmlquery.Node) {
	if n.PrevSibling == nil && n.NextSibling == nil {
		n.Parent.FirstChild = nil
		n.Parent.LastChild = nil
		n.Parent = nil
		return
	}
	if n.PrevSibling == nil {
		n.Parent.FirstChild = n.NextSibling
		n.Parent = nil
		return
	}
	if n.NextSibling == nil {
		n.Parent.LastChild = n.PrevSibling
		n.Parent = nil
		return
	}
	n.PrevSibling.NextSibling = n.NextSibling
	n.NextSibling.PrevSibling = n.PrevSibling
	n.Parent = nil
}

// LookupAttr looks up an attribute by its key and if the key was found.
func LookupAttr(node *xmlquery.Node, key xml.Name) (string, bool) {
	if node == nil {
		return "", false
	}
	for _, attr := range node.Attr {
		if key == attr.Name {
			return attr.Value, true
		}
	}
	return "", false
}

// LookupAttrAsFloat looks up an attribute by its key and interprets
// it as a float.
func LookupAttrAsFloat(node *xmlquery.Node, key xml.Name) (float64, bool) {
	str, ok := LookupAttr(node, key)
	if !ok {
		return 0, false
	}
	val, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0, false
	}
	return val, true
}

// LookupAttrAsInt looks up an attribute by its key and interprets it
// as an int.
func LookupAttrAsInt(node *xmlquery.Node, key xml.Name) (int, bool) {
	str, ok := LookupAttr(node, key)
	if !ok {
		return 0, false
	}
	val, err := strconv.Atoi(str)
	if err != nil {
		return 0, false
	}
	return val, true
}

// SetAttr sets or overwrites the attribute in the given node.  If the
// node contains the given attribute is updated with the value.
// Otherwise a new attribute is appended to the node's attribute list.
func SetAttr(node *xmlquery.Node, attr xml.Attr) {
	if node == nil {
		return
	}
	for i := range node.Attr {
		if node.Attr[i].Name == attr.Name {
			node.Attr[i].Value = attr.Value
			return
		}
	}
	node.Attr = append(node.Attr, xmlquery.Attr{Name: attr.Name, Value: attr.Value})
}
