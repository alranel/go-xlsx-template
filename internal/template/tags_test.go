package template_test

import (
	"testing"

	"github.com/alranel/go-xlsx-template/internal/template"
)

func TestFindTRMarkerPlainFor(t *testing.T) {
	m, ok := template.FindTRMarker("{% for record in contributi %}")
	if !ok || m.Kind != template.TRFor || m.ForVar != "record" || m.ForExpr != "contributi" {
		t.Fatalf("marker: %#v ok=%v", m, ok)
	}
	if _, ok := template.FindTRMarker("{% if active %}yes{% else %}no{% endif %}"); ok {
		t.Fatal("cell conditional should not be a row marker")
	}
}

func TestFindTRMarkerPlainEndFor(t *testing.T) {
	m, ok := template.FindTRMarker("{% endfor %}")
	if !ok || m.Kind != template.TREndFor {
		t.Fatalf("marker: %#v ok=%v", m, ok)
	}
}
