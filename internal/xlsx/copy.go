package xlsx

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

// copyRow copies all populated cells from srcRow to dstRow on the same sheet.
func copyRow(f *excelize.File, sheet string, srcRow, dstRow, maxCol int) error {
	if maxCol < 1 {
		var err error
		maxCol, err = maxColumnInRow(f, sheet, srcRow)
		if err != nil {
			return err
		}
	}
	for col := 1; col <= maxCol; col++ {
		srcCell, err := excelize.CoordinatesToCellName(col, srcRow)
		if err != nil {
			return err
		}
		dstCell, err := excelize.CoordinatesToCellName(col, dstRow)
		if err != nil {
			return err
		}
		if err := copyCell(f, sheet, srcCell, dstCell); err != nil {
			return fmt.Errorf("copy %s -> %s: %w", srcCell, dstCell, err)
		}
	}
	return nil
}

func copyCell(f *excelize.File, sheet, srcCell, dstCell string) error {
	formula, err := f.GetCellFormula(sheet, srcCell)
	if err != nil {
		return err
	}
	if formula != "" {
		if err := f.SetCellFormula(sheet, dstCell, formula); err != nil {
			return err
		}
		styleID, err := f.GetCellStyle(sheet, srcCell)
		if err != nil {
			return err
		}
		if styleID != 0 {
			_ = f.SetCellStyle(sheet, dstCell, dstCell, styleID)
		}
		return nil
	}

	runs, err := f.GetCellRichText(sheet, srcCell)
	if err != nil {
		return err
	}
	if len(runs) > 0 {
		if err := f.SetCellRichText(sheet, dstCell, runs); err != nil {
			return err
		}
		styleID, err := f.GetCellStyle(sheet, srcCell)
		if err != nil {
			return err
		}
		if styleID != 0 {
			_ = f.SetCellStyle(sheet, dstCell, dstCell, styleID)
		}
		return nil
	}

	val, err := f.GetCellValue(sheet, srcCell)
	if err != nil {
		return err
	}
	if val == "" {
		return nil
	}
	if err := f.SetCellValue(sheet, dstCell, val); err != nil {
		return err
	}
	styleID, err := f.GetCellStyle(sheet, srcCell)
	if err != nil {
		return err
	}
	if styleID != 0 {
		_ = f.SetCellStyle(sheet, dstCell, dstCell, styleID)
	}
	return nil
}

func maxColumnInRow(f *excelize.File, sheet string, row int) (int, error) {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return 26, err
	}
	max := 0
	if row > 0 && row <= len(rows) {
		max = len(rows[row-1])
	}
	if max == 0 {
		max = 26
	}
	return max, nil
}

// copyRowBlock copies rows [srcStart, srcEnd] to starting at dstStart.
func copyRowBlock(f *excelize.File, sheet string, srcStart, srcEnd, dstStart int) error {
	height := srcEnd - srcStart + 1
	maxCol, err := maxColumnInSheet(f, sheet)
	if err != nil {
		return err
	}
	for i := 0; i < height; i++ {
		if err := copyRow(f, sheet, srcStart+i, dstStart+i, maxCol); err != nil {
			return err
		}
	}
	return copyMergesInBlock(f, sheet, srcStart, srcEnd, dstStart)
}

func maxColumnInSheet(f *excelize.File, sheet string) (int, error) {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return 26, err
	}
	max := 0
	for _, row := range rows {
		if len(row) > max {
			max = len(row)
		}
	}
	if max == 0 {
		max = 26
	}
	return max, nil
}

func maxColumnInRowRange(rows [][]string, startRow, endRow int) int {
	max := 0
	for row := startRow; row <= endRow; row++ {
		if row < 1 || row > len(rows) {
			continue
		}
		if len(rows[row-1]) > max {
			max = len(rows[row-1])
		}
	}
	if max == 0 {
		max = 1
	}
	return max
}

func copyMergesInBlock(f *excelize.File, sheet string, srcStart, srcEnd, dstStart int) error {
	merges, err := f.GetMergeCells(sheet)
	if err != nil {
		return err
	}
	offset := dstStart - srcStart
	for _, m := range merges {
		startCol, startRow, err := excelize.CellNameToCoordinates(m.GetStartAxis())
		if err != nil {
			continue
		}
		endCol, endRow, err := excelize.CellNameToCoordinates(m.GetEndAxis())
		if err != nil {
			continue
		}
		if startRow < srcStart || endRow > srcEnd {
			continue
		}
		newStart, _ := excelize.CoordinatesToCellName(startCol, startRow+offset)
		newEnd, _ := excelize.CoordinatesToCellName(endCol, endRow+offset)
		_ = f.MergeCell(sheet, newStart, newEnd)
	}
	return nil
}
