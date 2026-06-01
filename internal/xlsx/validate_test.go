package xlsx_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

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
