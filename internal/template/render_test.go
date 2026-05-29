package template_test

import (
	"testing"

	"github.com/alranel/go-xlsx-template/internal/template"
	"github.com/nikolalohinski/gonja/v2/exec"
)

func TestRenderString(t *testing.T) {
	r := template.NewRenderer()
	ctx := exec.NewContext(map[string]any{
		"name":  "bob",
		"price": 5,
		"items": []any{
			map[string]any{"foo": "a"},
			map[string]any{"foo": "b"},
		},
		"ok": true,
	})

	out, err := r.RenderString("Hello {{ name | capitalize }}!", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if out != "Hello Bob!" {
		t.Fatalf("got %q", out)
	}

	ok, err := r.EvalCondition("ok", ctx)
	if err != nil || !ok {
		t.Fatalf("EvalCondition ok: %v %v", ok, err)
	}

	items, err := r.EvalIterable("items", ctx)
	if err != nil || len(items) != 2 {
		t.Fatalf("EvalIterable items: %v %v", items, err)
	}
}
