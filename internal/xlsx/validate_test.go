package xlsx_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alranel/go-xlsx-template/internal/data"
	"github.com/alranel/go-xlsx-template/internal/xlsx"
	"github.com/xuri/excelize/v2"
)

func TestValidateFileValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.xlsx")
	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "{{ title }}")
	_ = f.SetCellValue("Sheet1", "A2", "{% if active %}yes{% endif %}")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	res, err := xlsx.ValidateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.IssueCount != 0 {
		t.Fatalf("expected valid, got %#v", res)
	}
}

func TestValidateFileCompileError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.xlsx")
	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "{{ unclosed")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	res, err := xlsx.ValidateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Valid {
		t.Fatal("expected invalid template")
	}
	if len(res.Issues) == 0 || res.Issues[0].Kind != xlsx.ValidationCompile {
		t.Fatalf("issues: %#v", res.Issues)
	}
	if res.Issues[0].Cell != "A1" {
		t.Fatalf("cell: %q", res.Issues[0].Cell)
	}
}

func TestValidateFileTRStructure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.xlsx")
	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "{%tr for item in items %}")
	_ = f.SetCellValue("Sheet1", "A2", "{{ item.name }}")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	res, err := xlsx.ValidateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Valid {
		t.Fatal("expected unclosed tr block")
	}
	found := false
	for _, iss := range res.Issues {
		if iss.Kind == xlsx.ValidationStructure {
			found = true
		}
	}
	if !found {
		t.Fatalf("issues: %#v", res.Issues)
	}
}

func TestValidateFileJSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.xlsx")
	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "{{ bad")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	res, err := xlsx.ValidateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var decoded xlsx.ValidationResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Valid {
		t.Fatal("expected invalid after round trip")
	}
}

func TestValidateInvoiceTemplate(t *testing.T) {
	path := filepath.Join("..", "..", "examples", "invoice", "template.xlsx")
	if _, err := os.Stat(path); err != nil {
		t.Skip("examples/invoice/template.xlsx not present")
	}
	res, err := xlsx.ValidateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Fatalf("expected valid invoice template, issues: %#v", res.Issues)
	}
}

func TestValidateTRMarkerCellSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.xlsx")
	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "{%tr for item in items %}")
	_ = f.SetCellValue("Sheet1", "A2", "{{ item.name }}")
	_ = f.SetCellValue("Sheet1", "A3", "{%tr endfor %}")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	res, err := xlsx.ValidateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Fatalf("expected valid, issues: %#v", res.Issues)
	}
}

func TestValidatePlainForMarkers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.xlsx")
	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "{% for item in items %}")
	_ = f.SetCellValue("Sheet1", "A2", "{{ item.name }}")
	_ = f.SetCellValue("Sheet1", "A3", "{% endfor %}")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	res, err := xlsx.ValidateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Fatalf("expected valid, issues: %#v", res.Issues)
	}
}

func TestValidateNestedTRForMarkers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.xlsx")
	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "{%tr for order in orders %}")
	_ = f.SetCellValue("Sheet1", "A2", "{%tr for line in order.lines %}")
	_ = f.SetCellValue("Sheet1", "A3", "{{ line.name }}")
	_ = f.SetCellValue("Sheet1", "A4", "{%tr endfor %}")
	_ = f.SetCellValue("Sheet1", "A5", "{%tr endfor %}")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	res, err := xlsx.ValidateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Fatalf("expected valid, issues: %#v", res.Issues)
	}
}

func TestValidateBroken2Template(t *testing.T) {
	path := filepath.Join("..", "..", "broken2.xlsx")
	if _, err := os.Stat(path); err != nil {
		t.Skip("broken2.xlsx not present")
	}
	res, err := xlsx.ValidateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Fatalf("expected valid, issues: %#v", res.Issues)
	}
}

func TestValidateBroken3Template(t *testing.T) {
	path := filepath.Join("..", "..", "broken3.xlsx")
	if _, err := os.Stat(path); err != nil {
		t.Skip("broken3.xlsx not present")
	}
	done := make(chan struct{})
	var res xlsx.ValidationResult
	var err error
	go func() {
		res, err = xlsx.ValidateFile(path)
		close(done)
	}()
	select {
	case <-done:
		if err != nil {
			t.Fatal(err)
		}
		if !res.Valid {
			t.Fatalf("expected valid, issues: %#v", res.Issues)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ValidateFile hung on broken3.xlsx")
	}
}

func TestRenderBroken4NoHang(t *testing.T) {
	tpl := filepath.Join("..", "..", "broken4.xlsx")
	dataPath := filepath.Join("..", "..", "test.json")
	if _, err := os.Stat(tpl); err != nil {
		t.Skip("broken4.xlsx not present")
	}
	if _, err := os.Stat(dataPath); err != nil {
		t.Skip("test.json not present")
	}
	out := filepath.Join(t.TempDir(), "out.xlsx")
	ctx, err := data.LoadContext(dataPath)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- xlsx.RenderFile(tpl, out, ctx) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("RenderFile hung on broken4.xlsx")
	}
}

func TestValidateFormulaSegments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.xlsx")
	f := excelize.NewFile()
	_ = f.SetCellFormula("Sheet1", "A1", "={{ ( }}*2")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	res, err := xlsx.ValidateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Valid {
		t.Fatal("expected invalid formula segment")
	}
}
