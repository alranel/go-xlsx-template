package xlsx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nikolalohinski/gonja/v2/exec"
)

func TestRepairMergeRefRowOnly(t *testing.T) {
	tests := []struct {
		ref, dim, want string
	}{
		{"2:2", "A1:N4", "A2:N2"},
		{"4:4", "A1:N4", "A4:N4"},
		{"A2:N2", "A1:N4", "A2:N2"},
		{"A:B", "A1:N4", ""},
	}
	for _, tc := range tests {
		got := repairMergeRef(tc.ref, tc.dim)
		if got != tc.want {
			t.Errorf("repairMergeRef(%q, %q) = %q, want %q", tc.ref, tc.dim, got, tc.want)
		}
	}
}

func TestRepairInflatedDimension(t *testing.T) {
	xml := []byte(`<?xml version="1.0"?><worksheet><dimension ref="A2:AF1048576"/><sheetData><row r="7"/><row r="11"><c r="A11"/></row><row r="1048576"></row></sheetData></worksheet>`)
	out, ok, err := stripEmptyRows(xml)
	if err != nil || !ok {
		t.Fatalf("strip: ok=%v err=%v", ok, err)
	}
	out, ok, err = repairInflatedDimension(out)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected repair")
	}
	if !strings.Contains(string(out), `ref="A2:AF11"`) {
		t.Fatalf("got %s", string(out))
	}
}

func TestStripEmptyRows(t *testing.T) {
	xml := []byte(`<sheetData><row r="9"><c r="A9"/></row><row r="1048576"></row></sheetData>`)
	out, ok, err := stripEmptyRows(xml)
	if err != nil || !ok {
		t.Fatalf("strip: ok=%v err=%v", ok, err)
	}
	if strings.Contains(string(out), `r="1048576"`) {
		t.Fatalf("empty row not stripped: %s", string(out))
	}
}

func TestValidateBrokenTemplateMergeCells(t *testing.T) {
	path := filepath.Join("..", "..", "broken_template.xlsx")
	if _, err := os.Stat(path); err != nil {
		t.Skip("broken_template.xlsx not present")
	}
	res, err := ValidateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Fatalf("expected valid after merge repair, issues: %#v", res.Issues)
	}
}

func TestRenderBrokenTemplateMergeCells(t *testing.T) {
	path := filepath.Join("..", "..", "broken_template.xlsx")
	if _, err := os.Stat(path); err != nil {
		t.Skip("broken_template.xlsx not present")
	}
	out := filepath.Join(t.TempDir(), "out.xlsx")
	ctx := exec.NewContext(map[string]any{
		"contributi_editore": []any{
			map[string]any{"den_editore": "Test", "tot_digitale": 1},
		},
	})
	_, err := RenderFileWithResult(path, out, ctx, DefaultRenderOptions())
	if err != nil {
		t.Fatal(err)
	}
}
