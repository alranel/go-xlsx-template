package xlsx

import (
	"fmt"
	"strconv"

	"github.com/alranel/go-xlsx-template/internal/render"
	"github.com/alranel/go-xlsx-template/internal/template"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/xuri/excelize/v2"
)

func renderSheetCells(f *excelize.File, sheet string, r *template.Renderer, ctx *exec.Context, rep *render.Report) error {
	return forEachUsedCell(f, sheet, func(cell string) error {
		return renderCell(f, sheet, cell, r, ctx, rep)
	})
}

func renderCell(f *excelize.File, sheet, cell string, r *template.Renderer, ctx *exec.Context, rep *render.Report) error {
	loc := render.Location{Sheet: sheet, Cell: cell}
	val, err := f.GetCellValue(sheet, cell)
	if err != nil {
		return err
	}
	if IsEscapedFormula(val) {
		return renderEscapedFormulaCell(f, sheet, cell, val, r, ctx, rep)
	}

	if ok, err := renderRichCell(f, sheet, cell, r, ctx, rep); err != nil {
		return err
	} else if ok {
		return nil
	}

	formula, err := f.GetCellFormula(sheet, cell)
	if err != nil {
		return err
	}
	if formula != "" || looksLikeFormulaCell(f, sheet, cell) {
		raw := formula
		if raw == "" {
			raw = val
		}
		if template.HasSyntax(raw) {
			out := renderFormulaTemplateLenient(raw, r, loc, ctx, rep)
			if err := f.SetCellFormula(sheet, cell, out); err != nil {
				return err
			}
		}
		return nil
	}

	if !template.HasSyntax(val) {
		return nil
	}

	if expr, ok := template.WholeCellVariable(val); ok {
		v, ok := r.RenderValueLenient(expr, loc, ctx, rep)
		if !ok {
			return nil
		}
		return f.SetCellValue(sheet, cell, formatCellScalar(v))
	}

	out := r.RenderStringLenient(val, loc, ctx, rep)
	return f.SetCellValue(sheet, cell, out)
}

func looksLikeFormulaCell(f *excelize.File, sheet, cell string) bool {
	val, _ := f.GetCellValue(sheet, cell)
	return len(val) > 0 && val[0] == '=' && template.HasSyntax(val)
}

func formatCellScalar(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "TRUE"
		}
		return "FALSE"
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	default:
		return fmt.Sprint(v)
	}
}
