package xlsx

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alranel/go-xlsx-template/internal/render"
	"github.com/alranel/go-xlsx-template/internal/template"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/xuri/excelize/v2"
)

type trBlock struct {
	kind     template.TRMarkerKind
	startRow int
	endRow   int
	forVar   string
	forExpr  string
	ifExpr   string
}

func expandTRBlocks(f *excelize.File, sheet string, r *template.Renderer, ctx *exec.Context, rep *render.Report) error {
	for {
		blocks, err := findTRBlocks(f, sheet)
		if err != nil {
			return err
		}
		if len(blocks) == 0 {
			return nil
		}
		// Process innermost block first (highest start row).
		b := blocks[0]
		if err := processTRBlock(f, sheet, r, ctx, rep, b); err != nil {
			return err
		}
	}
}

func findTRBlocks(f *excelize.File, sheet string) ([]trBlock, error) {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	return findTRBlocksFromRows(sheet, rows)
}

func findTRBlocksFromRows(sheet string, rows [][]string) ([]trBlock, error) {
	maxRow := len(rows)
	type frame struct {
		kind    template.TRMarkerKind
		row     int
		forVar  string
		forExpr string
		ifExpr  string
	}
	var stack []frame
	var blocks []trBlock

	for row := 1; row <= maxRow; row++ {
		text := rowTextFromRows(rows, row)
		marker, ok := template.FindTRMarker(text)
		if !ok {
			continue
		}
		marker.Row = row
		switch marker.Kind {
		case template.TRFor:
			stack = append(stack, frame{
				kind:    template.TRFor,
				row:     row,
				forVar:  marker.ForVar,
				forExpr: marker.ForExpr,
			})
		case template.TRIf:
			stack = append(stack, frame{
				kind:   template.TRIf,
				row:    row,
				ifExpr: marker.IfExpr,
			})
		case template.TREndFor:
			if len(stack) == 0 || stack[len(stack)-1].kind != template.TRFor {
				return nil, fmt.Errorf("sheet %q row %d: unexpected {%%tr endfor %%}", sheet, row)
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			blocks = append(blocks, trBlock{
				kind:     template.TRFor,
				startRow: top.row,
				endRow:   row,
				forVar:   top.forVar,
				forExpr:  top.forExpr,
			})
		case template.TREndIf:
			if len(stack) == 0 || stack[len(stack)-1].kind != template.TRIf {
				return nil, fmt.Errorf("sheet %q row %d: unexpected {%%tr endif %%}", sheet, row)
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			blocks = append(blocks, trBlock{
				kind:     template.TRIf,
				startRow: top.row,
				endRow:   row,
				ifExpr:   top.ifExpr,
			})
		}
	}
	if len(stack) > 0 {
		return nil, fmt.Errorf("sheet %q: unclosed {%%tr %%} block starting at row %d", sheet, stack[0].row)
	}
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].startRow > blocks[j].startRow
	})
	return blocks, nil
}

func processTRBlock(f *excelize.File, sheet string, r *template.Renderer, ctx *exec.Context, rep *render.Report, b trBlock) error {
	switch b.kind {
	case template.TRFor:
		return expandTRFor(f, sheet, r, ctx, rep, b)
	case template.TRIf:
		return expandTRIf(f, sheet, r, ctx, rep, b)
	default:
		return nil
	}
}

func expandTRFor(f *excelize.File, sheet string, r *template.Renderer, ctx *exec.Context, rep *render.Report, b trBlock) error {
	below, err := hasContentBelow(f, sheet, b.endRow)
	if err != nil {
		return err
	}
	if below {
		return expandTRForIncremental(f, sheet, r, ctx, rep, b)
	}
	return expandTRForReplace(f, sheet, r, ctx, rep, b)
}

func expandTRForReplace(f *excelize.File, sheet string, r *template.Renderer, ctx *exec.Context, rep *render.Report, b trBlock) error {
	loc := render.Location{Sheet: sheet, Row: b.startRow}
	items, ok := r.EvalIterableLenient(b.forExpr, loc, ctx, rep)
	if !ok {
		return nil
	}
	bodyStart := b.startRow + 1
	bodyEnd := b.endRow - 1
	height := bodyEnd - bodyStart + 1

	if len(items) == 0 {
		return removeRows(f, sheet, b.startRow, b.endRow)
	}

	templateRows, err := snapshotRowBlock(f, sheet, bodyStart, bodyEnd)
	if err != nil {
		return err
	}

	rendered := make([][]rowSnapshot, len(items))
	for i, item := range items {
		child := ctx.Inherit()
		child.Set(b.forVar, item)
		rows, err := cloneSnapshots(templateRows)
		if err != nil {
			return err
		}
		if err := renderSnapshots(f, sheet, rows, r, child, rep); err != nil {
			return err
		}
		rendered[i] = rows
	}

	if err := removeRows(f, sheet, b.startRow, b.endRow); err != nil {
		return err
	}

	dst := b.startRow
	for _, block := range rendered {
		if err := applySnapshots(f, sheet, dst, block); err != nil {
			return err
		}
		dst += height
	}
	return nil
}

func expandTRForIncremental(f *excelize.File, sheet string, r *template.Renderer, ctx *exec.Context, rep *render.Report, b trBlock) error {
	loc := render.Location{Sheet: sheet, Row: b.startRow}
	items, ok := r.EvalIterableLenient(b.forExpr, loc, ctx, rep)
	if !ok {
		return nil
	}
	bodyStart := b.startRow + 1
	bodyEnd := b.endRow - 1
	height := bodyEnd - bodyStart + 1

	if len(items) == 0 {
		return removeRows(f, sheet, b.startRow, b.endRow)
	}

	templateRows, err := snapshotRowBlock(f, sheet, bodyStart, bodyEnd)
	if err != nil {
		return err
	}

	applyItem := func(item any, dstStart int) error {
		child := ctx.Inherit()
		child.Set(b.forVar, item)
		rows, err := cloneSnapshots(templateRows)
		if err != nil {
			return err
		}
		if err := renderSnapshots(f, sheet, rows, r, child, rep); err != nil {
			return err
		}
		return applySnapshots(f, sheet, dstStart, rows)
	}

	extraRows := (len(items) - 1) * height
	if extraRows > 0 {
		if err := f.InsertRows(sheet, bodyEnd, extraRows); err != nil {
			return err
		}
		if err := shiftComments(f, sheet, bodyEnd, extraRows); err != nil {
			return err
		}
	}

	for i, item := range items {
		if err := applyItem(item, bodyStart+i*height); err != nil {
			return err
		}
	}

	endMarkerRow, err := findTRMarkerRow(f, sheet, template.TREndFor, b.endRow+(len(items)-1)*height)
	if err != nil {
		return err
	}
	if err := removeSingleRow(f, sheet, endMarkerRow); err != nil {
		return err
	}
	startMarkerRow, err := findTRMarkerRow(f, sheet, template.TRFor, b.startRow)
	if err != nil {
		return err
	}
	return removeSingleRow(f, sheet, startMarkerRow)
}

func findTRMarkerRow(f *excelize.File, sheet string, kind template.TRMarkerKind, hint int) (int, error) {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return 0, err
	}
	maxRow := len(rows)
	best := 0
	bestDist := maxRow + 1
	for row := 1; row <= maxRow; row++ {
		if !rowIsTRMarkerOnly(f, sheet, row) {
			continue
		}
		text := rowTextFromRows(rows, row)
		m, ok := template.FindTRMarker(text)
		if !ok || m.Kind != kind {
			continue
		}
		dist := row - hint
		if dist < 0 {
			dist = -dist
		}
		if dist < bestDist {
			bestDist = dist
			best = row
		}
	}
	if best == 0 {
		return 0, fmt.Errorf("sheet %q: {%%tr %%} marker not found near row %d", sheet, hint)
	}
	return best, nil
}

// rowIsTRMarkerOnly reports whether every non-empty cell on the row is a {%tr %} tag only.
func rowIsTRMarkerOnly(f *excelize.File, sheet string, row int) bool {
	maxCol, err := maxColumnInSheet(f, sheet)
	if err != nil {
		return false
	}
	hasMarker := false
	for col := 1; col <= maxCol; col++ {
		ref, err := excelize.CoordinatesToCellName(col, row)
		if err != nil {
			return false
		}
		val, err := f.GetCellValue(sheet, ref)
		if err != nil {
			return false
		}
		formula, _ := f.GetCellFormula(sheet, ref)
		text := val
		if formula != "" {
			text = formula
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if !template.FindTRMarkerLine(text) {
			return false
		}
		hasMarker = true
	}
	return hasMarker
}

func hasContentBelow(f *excelize.File, sheet string, afterRow int) (bool, error) {
	maxRow, err := sheetMaxRow(f, sheet)
	if err != nil {
		return false, err
	}
	return afterRow < maxRow, nil
}

func expandTRIf(f *excelize.File, sheet string, r *template.Renderer, ctx *exec.Context, rep *render.Report, b trBlock) error {
	loc := render.Location{Sheet: sheet, Row: b.startRow}
	ok, evaluated := r.EvalConditionLenient(b.ifExpr, loc, ctx, rep)
	if !evaluated {
		return nil
	}
	if !ok {
		return removeRows(f, sheet, b.startRow, b.endRow)
	}
	if err := removeSingleRow(f, sheet, b.endRow); err != nil {
		return err
	}
	return removeSingleRow(f, sheet, b.startRow)
}

func removeRows(f *excelize.File, sheet string, startRow, endRow int) error {
	for row := endRow; row >= startRow; row-- {
		if err := removeSingleRow(f, sheet, row); err != nil {
			return err
		}
	}
	return nil
}

func removeSingleRow(f *excelize.File, sheet string, row int) error {
	if err := shiftComments(f, sheet, row, -1); err != nil {
		return err
	}
	return f.RemoveRow(sheet, row)
}

// rowTextFromRows builds concatenated non-empty cell text for a 1-based row.
// {%tr %} markers are stored as cell values; avoid per-row GetCols/GetCellFormula
// which can make excelize pathologically slow on some workbooks.
func rowTextFromRows(rows [][]string, row int) string {
	if row < 1 || row > len(rows) {
		return ""
	}
	r := rows[row-1]
	parts := make([]string, 0, len(r))
	for _, v := range r {
		if v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, " ")
}

func sheetMaxRow(f *excelize.File, sheet string) (int, error) {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return len(rows), nil
}
