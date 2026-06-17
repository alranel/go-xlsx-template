package xlsx

import (
	"strings"

	"github.com/alranel/go-xlsx-template/internal/render"
	"github.com/alranel/go-xlsx-template/internal/template"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/xuri/excelize/v2"
)

type cellSnapshot struct {
	ref     string
	value   any
	formula string
	rich    []excelize.RichTextRun
	styleID int
}

type rowSnapshot struct {
	cells []cellSnapshot
}

func snapshotRowBlock(f *excelize.File, sheet string, startRow, endRow int) ([]rowSnapshot, error) {
	maxCol, err := maxColumnInSheet(f, sheet)
	if err != nil {
		return nil, err
	}
	var rows []rowSnapshot
	for row := startRow; row <= endRow; row++ {
		var cells []cellSnapshot
		for col := 1; col <= maxCol; col++ {
			ref, err := excelize.CoordinatesToCellName(col, row)
			if err != nil {
				return nil, err
			}
			snap, ok, err := snapshotCell(f, sheet, ref)
			if err != nil {
				return nil, err
			}
			if ok {
				cells = append(cells, snap)
			}
		}
		rows = append(rows, rowSnapshot{cells: cells})
	}
	return rows, nil
}

func snapshotCell(f *excelize.File, sheet, ref string) (cellSnapshot, bool, error) {
	formula, err := f.GetCellFormula(sheet, ref)
	if err != nil {
		return cellSnapshot{}, false, err
	}
	styleID, err := f.GetCellStyle(sheet, ref)
	if err != nil {
		return cellSnapshot{}, false, err
	}
	if formula != "" {
		return cellSnapshot{ref: ref, formula: formula, styleID: styleID}, true, nil
	}
	runs, err := f.GetCellRichText(sheet, ref)
	if err != nil {
		return cellSnapshot{}, false, err
	}
	if len(runs) > 0 {
		return cellSnapshot{ref: ref, rich: runs, styleID: styleID}, true, nil
	}
	val, err := f.GetCellValue(sheet, ref)
	if err != nil {
		return cellSnapshot{}, false, err
	}
	if val == "" && styleID == 0 {
		return cellSnapshot{}, false, nil
	}
	return cellSnapshot{ref: ref, value: val, styleID: styleID}, true, nil
}

func cloneSnapshots(in []rowSnapshot) ([]rowSnapshot, error) {
	out := make([]rowSnapshot, len(in))
	for i, row := range in {
		cells := make([]cellSnapshot, len(row.cells))
		copy(cells, row.cells)
		out[i] = rowSnapshot{cells: cells}
	}
	return out, nil
}

func renderSnapshots(f *excelize.File, sheet string, rows []rowSnapshot, r *template.Renderer, ctx *exec.Context, rep *render.Report) error {
	for _, row := range rows {
		for i := range row.cells {
			c := &row.cells[i]
			loc := render.Location{Sheet: sheet, Cell: c.ref}
			if c.formula != "" && template.HasSyntax(c.formula) {
				if expr, ok := wholeCellFormulaVariable(c.formula); ok {
					v, ok := r.RenderValueLenient(expr, loc, ctx, rep)
					if ok {
						c.formula = ""
						c.value = cellScalarValue(v)
					}
				} else {
					c.formula = renderFormulaTemplateLenient(c.formula, r, loc, ctx, rep)
				}
				continue
			}
			if s, ok := c.value.(string); ok {
				if raw, ok := UnwrapEscapedFormula(s); ok {
					if expr, ok := wholeCellFormulaVariable(raw); ok {
						v, ok := r.RenderValueLenient(expr, loc, ctx, rep)
						if ok {
							c.formula = ""
							c.value = cellScalarValue(v)
							continue
						}
					}
					c.formula = renderFormulaTemplateLenient(raw, r, loc, ctx, rep)
					c.value = nil
					continue
				}
			}
			if len(c.rich) > 0 {
				combined := ""
				for _, run := range c.rich {
					combined += run.Text
				}
				if template.HasSyntax(combined) {
					if expr, ok := template.WholeCellVariable(combined); ok {
						v, ok := r.RenderValueLenient(expr, loc, ctx, rep)
						if ok {
							c.rich = nil
							c.value = cellScalarValue(v)
						}
					} else {
						out := r.RenderStringLenient(combined, loc, ctx, rep)
						font := c.rich[0].Font
						c.rich = []excelize.RichTextRun{{Text: out, Font: font}}
						c.value = nil
					}
				}
				continue
			}
			if s, ok := c.value.(string); ok && s != "" && template.HasSyntax(s) && !strings.HasPrefix(s, formulaEscapeQuote) {
				if expr, ok := template.WholeCellVariable(s); ok {
					v, ok := r.RenderValueLenient(expr, loc, ctx, rep)
					if ok {
						c.value = cellScalarValue(v)
					}
				} else {
					c.value = r.RenderStringLenient(s, loc, ctx, rep)
				}
			}
		}
	}
	return nil
}

func applySnapshots(f *excelize.File, sheet string, startRow int, rows []rowSnapshot) error {
	for i, row := range rows {
		for _, snap := range row.cells {
			col, _, err := excelize.CellNameToCoordinates(snap.ref)
			if err != nil {
				return err
			}
			dst, err := excelize.CoordinatesToCellName(col, startRow+i)
			if err != nil {
				return err
			}
			if snap.formula != "" {
				if err := f.SetCellFormula(sheet, dst, snap.formula); err != nil {
					return err
				}
			} else if len(snap.rich) > 0 {
				if err := f.SetCellRichText(sheet, dst, snap.rich); err != nil {
					return err
				}
			} else if !isEmptyScalar(snap.value) {
				if err := f.SetCellValue(sheet, dst, snap.value); err != nil {
					return err
				}
			}
			if snap.styleID != 0 {
				_ = f.SetCellStyle(sheet, dst, dst, snap.styleID)
			}
		}
	}
	return nil
}
