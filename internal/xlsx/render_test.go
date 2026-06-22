package xlsx_test

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/alranel/go-xlsx-template/internal/data"
	"github.com/alranel/go-xlsx-template/internal/xlsx"
	"github.com/xuri/excelize/v2"
)

func TestRenderSimpleVars(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "{{ foo }}")
	_ = f.SetCellValue("Sheet1", "A2", "{{ count }}")
	_ = f.SetCellValue("Sheet1", "A3", "{{ active }}")
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(jsonPath, []byte(`{"foo":"hello","count":42,"active":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := xlsx.RenderFile(tpl, out, ctx); err != nil {
		t.Fatal(err)
	}

	g, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	assertCell(t, g, "Sheet1", "A1", "hello")
	v, _ := g.GetCellValue("Sheet1", "A2")
	if v != "42" {
		t.Fatalf("A2: got %q", v)
	}
	active, _ := g.GetCellValue("Sheet1", "A3")
	if active != "true" && active != "TRUE" && active != "True" {
		t.Fatalf("A3: got %q", active)
	}
	assertCellTypeNotString(t, g, "Sheet1", "A2")
}

func TestRenderNumericWholeCellFormula(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	_ = f.SetCellFormula("Sheet1", "B4", "{{ price }}")
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(jsonPath, []byte(`{"price":42.5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := xlsx.RenderFile(tpl, out, ctx); err != nil {
		t.Fatal(err)
	}

	g, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	assertCell(t, g, "Sheet1", "B4", "42.5")
	assertCellTypeNotString(t, g, "Sheet1", "B4")
}

func TestRenderFormula(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	_ = f.SetCellFormula("Sheet1", "A1", "={{ price }}*2")
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(jsonPath, []byte(`{"price":5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := xlsx.RenderFile(tpl, out, ctx); err != nil {
		t.Fatal(err)
	}

	g, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	formula, err := g.GetCellFormula("Sheet1", "A1")
	if err != nil {
		t.Fatal(err)
	}
	if formula != "=5*2" && formula != "=10" {
		t.Fatalf("formula: %q", formula)
	}
}

func TestRenderFormulaClearsStaleCachedValue(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	_ = f.SetCellFormula("Sheet1", "A1", "={{ price }}*2")
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := injectFormulaCachedValue(tpl, "A1", "0"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonPath, []byte(`{"price":42.5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := xlsx.RenderFile(tpl, out, ctx); err != nil {
		t.Fatal(err)
	}

	cellXML, err := sheetCellXML(out, "A1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cellXML, "<f>") {
		t.Fatalf("expected formula in cell XML: %s", cellXML)
	}
	if strings.Contains(cellXML, "<v>") {
		t.Fatalf("expected stale cached <v> to be cleared, got: %s", cellXML)
	}
}

func injectFormulaCachedValue(xlsxPath, ref, stale string) error {
	data, err := os.ReadFile(xlsxPath)
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	re := regexp.MustCompile(`(<c[^>]*r="` + regexp.QuoteMeta(ref) + `"[^>]*>.*?<f[^>]*>[^<]*</f>)`)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		if strings.HasSuffix(f.Name, "sheet1.xml") {
			body = re.ReplaceAll(body, []byte("${1}<v>"+stale+"</v>"))
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			return err
		}
		if _, err := w.Write(body); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(xlsxPath, buf.Bytes(), 0o644)
}

func sheetCellXML(xlsxPath, ref string) (string, error) {
	data, err := os.ReadFile(xlsxPath)
	if err != nil {
		return "", err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`<c[^>]*r="` + regexp.QuoteMeta(ref) + `"[^>]*>.*?</c>`)
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, "sheet1.xml") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", err
		}
		if m := re.FindString(string(body)); m != "" {
			return m, nil
		}
	}
	return "", nil
}

func TestRenderRowLoop(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "{%tr for item in items %}")
	_ = f.SetCellValue("Sheet1", "A2", "{{ item.foo }}")
	_ = f.SetCellValue("Sheet1", "A3", "{%tr endfor %}")
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(jsonPath, []byte(`{"items":[{"foo":"x"},{"foo":"y"},{"foo":"z"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := xlsx.RenderFile(tpl, out, ctx); err != nil {
		t.Fatal(err)
	}

	g, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	for i, want := range []string{"x", "y", "z"} {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		assertCell(t, g, "Sheet1", cell, want)
	}
}

func TestRenderPlainForMarkers(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "{% for item in items %}")
	_ = f.SetCellValue("Sheet1", "A2", "{{ item.foo }}")
	_ = f.SetCellValue("Sheet1", "A3", "{% endfor %}")
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(jsonPath, []byte(`{"items":[{"foo":"x"},{"foo":"y"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := xlsx.RenderFile(tpl, out, ctx); err != nil {
		t.Fatal(err)
	}

	g, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	for i, want := range []string{"x", "y"} {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		assertCell(t, g, "Sheet1", cell, want)
	}
}

func TestRenderRowIfFalse(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "{%tr if show %}")
	_ = f.SetCellValue("Sheet1", "A2", "hidden")
	_ = f.SetCellValue("Sheet1", "A3", "{%tr endif %}")
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(jsonPath, []byte(`{"show":false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := xlsx.RenderFile(tpl, out, ctx); err != nil {
		t.Fatal(err)
	}

	g, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	rows, _ := g.GetRows("Sheet1")
	if len(rows) != 0 {
		t.Fatalf("expected empty sheet, got %#v", rows)
	}
}

func TestRenderRowIfTrue(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "{%tr if show %}")
	_ = f.SetCellValue("Sheet1", "A2", "visible")
	_ = f.SetCellValue("Sheet1", "A3", "{%tr endif %}")
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(jsonPath, []byte(`{"show":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := xlsx.RenderFile(tpl, out, ctx); err != nil {
		t.Fatal(err)
	}

	g, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	assertCell(t, g, "Sheet1", "A1", "visible")
}

func TestRenderSheetNameIfTrue(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	const sheet = "{% if foo %}Foobar{% endif %}"
	idx, _ := f.NewSheet(sheet)
	f.SetActiveSheet(idx)
	_ = f.SetCellValue(sheet, "A1", "ok")
	_, _ = f.NewSheet("Keep")
	if err := f.DeleteSheet("Sheet1"); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(jsonPath, []byte(`{"foo":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := xlsx.RenderFile(tpl, out, ctx); err != nil {
		t.Fatal(err)
	}

	g, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	names := g.GetSheetList()
	if len(names) != 2 {
		t.Fatalf("sheets: %#v", names)
	}
	found := false
	for _, n := range names {
		if n == "Foobar" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Foobar sheet, got %#v", names)
	}
}

func TestRenderSheetNameIfFalseDeletes(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	const sheet = "{% if foo %}Foobar{% endif %}"
	idx, _ := f.NewSheet(sheet)
	f.SetActiveSheet(idx)
	_ = f.SetCellValue(sheet, "A1", "gone")
	_, _ = f.NewSheet("Keep")
	if err := f.DeleteSheet("Sheet1"); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(jsonPath, []byte(`{"foo":false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := xlsx.RenderFile(tpl, out, ctx); err != nil {
		t.Fatal(err)
	}

	g, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	names := g.GetSheetList()
	if len(names) != 1 || names[0] != "Keep" {
		t.Fatalf("sheets: %#v", names)
	}
}

func TestRenderSheetName(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	const sheet = "Report {{ year }}"
	idx, _ := f.NewSheet(sheet)
	f.SetActiveSheet(idx)
	_ = f.SetCellValue(sheet, "A1", "ok")
	if err := f.DeleteSheet("Sheet1"); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(jsonPath, []byte(`{"year":2025}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := xlsx.RenderFile(tpl, out, ctx); err != nil {
		t.Fatal(err)
	}

	g, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	names := g.GetSheetList()
	if len(names) != 1 || names[0] != "Report 2025" {
		t.Fatalf("sheets: %#v", names)
	}
}

func TestRenderFormulaSumAfterLoop(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "{%tr for item in items %}")
	_ = f.SetCellValue("Sheet1", "B2", "{{ item.val }}")
	_ = f.SetCellValue("Sheet1", "A3", "{%tr endfor %}")
	// Use a range that covers the two expanded body rows; excelize adjusts row refs on insert.
	_ = f.SetCellFormula("Sheet1", "B5", "=SUM(B2:B3)")
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(jsonPath, []byte(`{"items":[{"val":10},{"val":20}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := xlsx.RenderFile(tpl, out, ctx); err != nil {
		t.Fatal(err)
	}

	g, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	assertCell(t, g, "Sheet1", "B1", "10")
	assertCell(t, g, "Sheet1", "B2", "20")
	// Totals row shifts with marker removal; excelize adjusts row positions but not static range widths.
	var formula string
	for row := 1; row <= 8; row++ {
		cell, _ := excelize.CoordinatesToCellName(2, row)
		f, err := g.GetCellFormula("Sheet1", cell)
		if err != nil {
			t.Fatal(err)
		}
		if f != "" {
			formula = f
			break
		}
	}
	if formula == "" {
		t.Fatal("expected a SUM formula cell below the loop block")
	}
	if formula != "SUM(B2:B3)" && formula != "=SUM(B2:B3)" && formula != "SUM(B2:B2)" && formula != "=SUM(B2:B2)" {
		t.Fatalf("unexpected total formula %q", formula)
	}
}

func TestRenderRichText(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	_ = f.SetCellRichText("Sheet1", "A1", []excelize.RichTextRun{
		{Text: "Hello {{ name }}", Font: &excelize.Font{Bold: true}},
	})
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(jsonPath, []byte(`{"name":"World"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := xlsx.RenderFile(tpl, out, ctx); err != nil {
		t.Fatal(err)
	}

	g, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	runs, err := g.GetCellRichText("Sheet1", "A1")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) == 0 || runs[0].Text != "Hello World" {
		t.Fatalf("rich text: %#v", runs)
	}
}

func TestRenderComment(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	if err := f.AddComment("Sheet1", excelize.Comment{
		Cell: "A1",
		Text: "Note: {{ msg }}",
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(jsonPath, []byte(`{"msg":"done"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := xlsx.RenderFile(tpl, out, ctx); err != nil {
		t.Fatal(err)
	}

	g, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	comments, err := g.GetComments("Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 || comments[0].Text != "Note: done" {
		t.Fatalf("comments: %#v", comments)
	}
}

func TestRenderLenientMissingVariable(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "{{ nope.foo.bar }}")
	_ = f.SetCellValue("Sheet1", "A2", "{{ ok }}")
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(jsonPath, []byte(`{"ok":"yes"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	res, err := xlsx.RenderFileWithResult(tpl, out, ctx, xlsx.DefaultRenderOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Issues) != 1 {
		t.Fatalf("issues: got %d want 1: %#v", len(res.Issues), res.Issues)
	}
	if res.Issues[0].Cell != "A1" {
		t.Fatalf("issue cell: %q", res.Issues[0].Cell)
	}

	g, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	assertCell(t, g, "Sheet1", "A1", "{{ nope.foo.bar }}")
	assertCell(t, g, "Sheet1", "A2", "yes")
}

func TestRenderStrictWithIssues(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.xlsx")
	out := filepath.Join(dir, "out.xlsx")
	jsonPath := filepath.Join(dir, "data.json")

	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "{{ nope.foo.bar }}")
	if err := f.SaveAs(tpl); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := os.WriteFile(jsonPath, []byte(`{"ok":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := data.LoadContext(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = xlsx.RenderFileWithResult(tpl, out, ctx, xlsx.RenderOptions{Strict: true})
	if err == nil {
		t.Fatal("expected strict error")
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Fatal("strict mode should not write output")
	}
}

func assertCell(t *testing.T, f *excelize.File, sheet, cell, want string) {
	t.Helper()
	got, err := f.GetCellValue(sheet, cell)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s: got %q want %q", cell, got, want)
	}
}

func assertCellTypeNotString(t *testing.T, f *excelize.File, sheet, cell string) {
	t.Helper()
	typ, err := f.GetCellType(sheet, cell)
	if err != nil {
		t.Fatal(err)
	}
	if typ == excelize.CellTypeSharedString || typ == excelize.CellTypeInlineString {
		t.Fatalf("%s: got string cell type %v", cell, typ)
	}
}
