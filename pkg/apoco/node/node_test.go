package node

import (
	"strings"
	"testing"

	"github.com/antchfx/xmlquery"
)

func TestPrettyPrint(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<root>
<first>
<second>second</second>
</first>
</root>`
	doc, err := xmlquery.Parse(strings.NewReader(xml))
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// t.Logf("%s", doc.FirstChild.OutputXML(true))
	if got := PrettyPrint(doc, "", "\t"); got != xml {
		t.Errorf("expected %s; got %s", xml, got)
	}
}
