package xlsx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alranel/go-xlsx-template/internal/data"
	"github.com/alranel/go-xlsx-template/internal/render"
	"github.com/alranel/go-xlsx-template/internal/template"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/xuri/excelize/v2"
)

func TestRenderCellEscapedRestore(t *testing.T) {
	f := excelize.NewFile()
	if err := f.SetCellValue("Sheet1", "A1", EscapeFormulaString("={{ price }}*2")); err != nil {
		t.Fatal(err)
	}
	val, _ := f.GetCellValue("Sheet1", "A1")
	if !IsEscapedFormula(val) {
		t.Fatalf("before render: val=%q isEsc=false", val)
	}
	ctx := exec.NewContext(map[string]any{"price": 5})
	if err := renderCell(f, "Sheet1", "A1", template.NewRenderer(), ctx, render.NewReport()); err != nil {
		t.Fatal(err)
	}
	fml, _ := f.GetCellFormula("Sheet1", "A1")
	if fml != "=5*2" && fml != "=10" {
		val2, _ := f.GetCellValue("Sheet1", "A1")
		t.Fatalf("formula %q value %q", fml, val2)
	}
}

func TestEscapeUnwrapFormula(t *testing.T) {
	raw := "={{ price }}*2"
	escaped := EscapeFormulaString(raw)
	if !IsEscapedFormula(escaped) {
		t.Fatalf("expected escaped marker, got %q", escaped)
	}
	got, ok := UnwrapEscapedFormula(escaped)
	if !ok || got != raw {
		t.Fatalf("unwrap: %q ok=%v", got, ok)
	}
}

func TestRenderFormulaInsideTRLoop(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "{%tr for item in items %}")
	_ = f.SetCellFormula("Sheet1", "B2", "={{ item.qty }}*{{ item.price }}")
	_ = f.SetCellValue("Sheet1", "A3", "{%tr endfor %}")
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(jsonPath, []byte(`{"items":[{"qty":2,"price":5},{"qty":3,"price":10}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := RenderFile(tpl, out, ctx); err != nil {
		t.Fatal(err)
	}

	g, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()

	f1, _ := g.GetCellFormula("Sheet1", "B1")
	f2, _ := g.GetCellFormula("Sheet1", "B2")
	if f1 != "=2*5" && f1 != "=10" && f1 != "=2.0*5.0" {
		t.Fatalf("B1 formula: %q", f1)
	}
	if f2 != "=3*10" && f2 != "=30" && f2 != "=3.0*10.0" {
		t.Fatalf("B2 formula: %q", f2)
	}
}
