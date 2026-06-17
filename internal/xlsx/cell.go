package xlsx

import (
	"fmt"
	"strconv"
	"strings"

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
			if expr, ok := wholeCellFormulaVariable(raw); ok {
				v, ok := r.RenderValueLenient(expr, loc, ctx, rep)
				if !ok {
					return nil
				}
				return setRenderedScalar(f, sheet, cell, v)
			}
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
		return setRenderedScalar(f, sheet, cell, v)
	}

	out := r.RenderStringLenient(val, loc, ctx, rep)
	return f.SetCellValue(sheet, cell, out)
}

func setRenderedScalar(f *excelize.File, sheet, cell string, v any) error {
	return f.SetCellValue(sheet, cell, cellScalarValue(v))
}

func wholeCellFormulaVariable(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if expr, ok := template.WholeCellVariable(raw); ok {
		return expr, true
	}
	if strings.HasPrefix(raw, "=") {
		return template.WholeCellVariable(strings.TrimSpace(raw[1:]))
	}
	return "", false
}

func cellScalarValue(v any) any {
	switch x := v.(type) {
	case string, bool, float64, float32, int, int64, int32:
		return x
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func isEmptyScalar(v any) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return s == ""
	}
	return false
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
