package xlsx

import (
	"strings"

	"github.com/alranel/go-xlsx-template/internal/render"
	"github.com/alranel/go-xlsx-template/internal/template"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/xuri/excelize/v2"
)

// formulaEscapeQuote is a leading single quote so Excel treats the cell as text
// while row insert/remove runs (excelize mishandles stored formulas containing "{{").
// excelize may surface the value as rich text; renderCell checks GetCellValue for
// the escape prefix before handling rich text runs.
const formulaEscapeQuote = "'"

// formulaEscapePrefix follows the quote and identifies our escaped formula payload.
const formulaEscapePrefix = "__go_xlsx_template:fml:"

// EscapeFormulaString stores a formula as a plain cell string safe for row operations.
func EscapeFormulaString(formula string) string {
	return formulaEscapeQuote + formulaEscapePrefix + formula
}

// IsEscapedFormula reports whether val is an escaped formula placeholder cell.
func IsEscapedFormula(val string) bool {
	_, ok := UnwrapEscapedFormula(val)
	return ok
}

// UnwrapEscapedFormula returns the original formula text from an escaped cell value.
func UnwrapEscapedFormula(val string) (string, bool) {
	if raw, ok := strings.CutPrefix(val, formulaEscapeQuote+formulaEscapePrefix); ok {
		return raw, true
	}
	if raw, ok := strings.CutPrefix(val, formulaEscapePrefix); ok {
		return raw, true
	}
	return "", false
}

// escapeFormulaPlaceholders converts templated formulas to escaped text cells.
func escapeFormulaPlaceholders(f *excelize.File, sheet string) error {
	return forEachUsedCell(f, sheet, func(cell string) error {
		formula, err := f.GetCellFormula(sheet, cell)
		if err != nil {
			return err
		}
		if formula == "" || !template.HasSyntax(formula) {
			return nil
		}
		return f.SetCellValue(sheet, cell, EscapeFormulaString(formula))
	})
}

func renderFormulaTemplateLenient(raw string, r *template.Renderer, loc render.Location, ctx *exec.Context, rep *render.Report) string {
	if !template.HasSyntax(raw) {
		return raw
	}
	return template.ReplaceVarSegmentsLenient(raw, func(expr string) (string, error) {
		v, err := r.EvalExpression(expr, ctx)
		if err != nil {
			rep.Add(loc, render.KindExpression, "{{ "+expr+" }}", err)
			return "", err
		}
		return formatCellScalar(v), nil
	})
}

func renderEscapedFormulaCell(f *excelize.File, sheet, cell, val string, r *template.Renderer, ctx *exec.Context, rep *render.Report) error {
	raw, ok := UnwrapEscapedFormula(val)
	if !ok {
		return nil
	}
	loc := render.Location{Sheet: sheet, Cell: cell}
	if expr, ok := wholeCellFormulaVariable(raw); ok {
		v, ok := r.RenderValueLenient(expr, loc, ctx, rep)
		if !ok {
			return nil
		}
		return setRenderedScalar(f, sheet, cell, v)
	}
	out := renderFormulaTemplateLenient(raw, r, loc, ctx, rep)
	return f.SetCellFormula(sheet, cell, out)
}

// forEachUsedCell visits each populated coordinate on the sheet.
func forEachUsedCell(f *excelize.File, sheet string, fn func(cell string) error) error {
	seen := make(map[string]struct{})
	rows, err := f.GetRows(sheet)
	if err != nil {
		return err
	}
	visit := func(col, row int) error {
		cell, err := excelize.CoordinatesToCellName(col, row)
		if err != nil {
			return err
		}
		if _, ok := seen[cell]; ok {
			return nil
		}
		seen[cell] = struct{}{}
		return fn(cell)
	}
	for rowIdx, row := range rows {
		for colIdx := range row {
			if err := visit(colIdx+1, rowIdx+1); err != nil {
				return err
			}
		}
	}
	cols, err := f.GetCols(sheet)
	if err != nil {
		return err
	}
	for colIdx, col := range cols {
		for rowIdx := range col {
			if rowIdx < len(rows) && colIdx < len(rows[rowIdx]) {
				continue
			}
			if col[rowIdx] == "" {
				cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
				formula, _ := f.GetCellFormula(sheet, cell)
				if formula == "" {
					continue
				}
			}
			if err := visit(colIdx+1, rowIdx+1); err != nil {
				return err
			}
		}
	}
	return nil
}
