package xlsx

import (
	"fmt"
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
	sheetRows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	maxCol := maxColumnInRowRange(sheetRows, startRow, endRow)
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

func snapshotRowsToStrings(rows []rowSnapshot) [][]string {
	out := make([][]string, len(rows))
	for i, row := range rows {
		var parts []string
		for _, c := range row.cells {
			if c.formula != "" {
				parts = append(parts, c.formula)
				continue
			}
			if len(c.rich) > 0 {
				for _, run := range c.rich {
					parts = append(parts, run.Text)
				}
				continue
			}
			if s, ok := c.value.(string); ok && s != "" {
				parts = append(parts, s)
			}
		}
		out[i] = parts
	}
	return out
}

// expandTRSnapshots expands nested {%tr %} blocks inside a for-loop body snapshot
// using the loop iteration context before cell templates are rendered.
func expandTRSnapshots(rows []rowSnapshot, f *excelize.File, sheet string, r *template.Renderer, ctx *exec.Context, rep *render.Report, loc render.Location) ([]rowSnapshot, error) {
	for {
		rowStrs := snapshotRowsToStrings(rows)
		blocks, err := findTRBlocksFromRows("", rowStrs)
		if err != nil {
			return nil, err
		}
		if len(blocks) == 0 {
			return rows, nil
		}
		b := blocks[0]
		rows, err = processTRSnapshotBlock(rows, f, sheet, b, r, ctx, rep, loc)
		if err != nil {
			return nil, err
		}
	}
}

func processTRSnapshotBlock(rows []rowSnapshot, f *excelize.File, sheet string, b trBlock, r *template.Renderer, ctx *exec.Context, rep *render.Report, loc render.Location) ([]rowSnapshot, error) {
	switch b.kind {
	case template.TRFor:
		return expandTRForSnapshots(rows, f, sheet, b, r, ctx, rep, loc)
	case template.TRIf:
		return expandTRIfSnapshots(rows, b, r, ctx, rep, loc)
	default:
		return rows, nil
	}
}

func expandTRIfSnapshots(rows []rowSnapshot, b trBlock, r *template.Renderer, ctx *exec.Context, rep *render.Report, loc render.Location) ([]rowSnapshot, error) {
	ok, evaluated := r.EvalConditionLenient(b.ifExpr, loc, ctx, rep)
	if !evaluated {
		return nil, fmt.Errorf("row %d: cannot evaluate {%%tr if %%} condition %q", b.startRow, b.ifExpr)
	}
	start := b.startRow - 1
	end := b.endRow - 1
	if start < 0 || end >= len(rows) {
		return nil, fmt.Errorf("row %d: {%%tr if %%} block out of range", b.startRow)
	}
	if !ok {
		return append(append([]rowSnapshot{}, rows[:start]...), rows[end+1:]...), nil
	}
	body := rows[start+1 : end]
	return append(append(append([]rowSnapshot{}, rows[:start]...), body...), rows[end+1:]...), nil
}

func expandTRForSnapshots(rows []rowSnapshot, f *excelize.File, sheet string, b trBlock, r *template.Renderer, ctx *exec.Context, rep *render.Report, loc render.Location) ([]rowSnapshot, error) {
	items, ok := r.EvalIterableLenient(b.forExpr, loc, ctx, rep)
	if !ok {
		return rows, nil
	}
	start := b.startRow - 1
	end := b.endRow - 1
	if start < 0 || end >= len(rows) {
		return nil, fmt.Errorf("row %d: {%%tr for %%} block out of range", b.startRow)
	}
	body := rows[start+1 : end]
	if len(items) == 0 {
		return append(append([]rowSnapshot{}, rows[:start]...), rows[end+1:]...), nil
	}
	var expanded []rowSnapshot
	for _, item := range items {
		child := ctx.Inherit()
		child.Set(b.forVar, item)
		cloned, err := cloneSnapshots(body)
		if err != nil {
			return nil, err
		}
		cloned, err = expandTRSnapshots(cloned, f, sheet, r, child, rep, loc)
		if err != nil {
			return nil, err
		}
		if err := renderSnapshots(f, sheet, cloned, r, child, rep); err != nil {
			return nil, err
		}
		expanded = append(expanded, cloned...)
	}
	return append(append(append([]rowSnapshot{}, rows[:start]...), expanded...), rows[end+1:]...), nil
}
