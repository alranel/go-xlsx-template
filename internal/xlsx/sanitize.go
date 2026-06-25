package xlsx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

var (
	worksheetPathRe = regexp.MustCompile(`^xl/worksheets/sheet\d+\.xml$`)
	dimensionRefRe  = regexp.MustCompile(`<dimension ref="([^"]+)"`)
	mergeCellRefRe  = regexp.MustCompile(`<mergeCell ref="([^"]+)"\s*/>`)
	rowTagRe        = regexp.MustCompile(`<row r="(\d+)"`)
	cellRefRe       = regexp.MustCompile(`<c r="([A-Z]+)(\d+)"`)
	selfClosingRow  = regexp.MustCompile(`<row r="\d+"[^>]*/>`)
	emptyRow        = regexp.MustCompile(`<row r="\d+"[^>]*>\s*</row>`)
)

// repairWorkbookBytes fixes known-bad worksheet metadata that makes excelize fail
// on otherwise readable templates (e.g. LibreOffice row-only merge refs "2:2").
func repairWorkbookBytes(data []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	changed := false
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		if worksheetPathRe.MatchString(f.Name) {
			repaired, ok, err := repairWorksheetMergeCells(body)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", f.Name, err)
			}
			if ok {
				body = repaired
				changed = true
			}
			repaired, ok, err = stripEmptyRows(body)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", f.Name, err)
			}
			if ok {
				body = repaired
				changed = true
			}
			repaired, ok, err = repairInflatedDimension(body)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", f.Name, err)
			}
			if ok {
				body = repaired
				changed = true
			}
		}
		hdr := f.FileHeader
		w, err := zw.CreateHeader(&hdr)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(body); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if !changed {
		return data, nil
	}
	return out.Bytes(), nil
}

// repairInflatedDimension clamps worksheet dimensions that extend to the last Excel
// row while the sheet only contains a handful of populated rows (LibreOffice export quirk).
func repairInflatedDimension(xml []byte) ([]byte, bool, error) {
	s := string(xml)
	m := dimensionRefRe.FindStringSubmatch(s)
	if len(m) != 2 {
		return xml, false, nil
	}
	dim := m[1]
	parts := strings.Split(dim, ":")
	if len(parts) != 2 {
		return xml, false, nil
	}
	_, endRow, err := excelize.CellNameToCoordinates(parts[1])
	if err != nil {
		return xml, false, nil
	}
	maxDataRow := maxRowWithCellsInXML(s)
	if maxDataRow == 0 || endRow <= maxDataRow {
		return xml, false, nil
	}
	endCol, _, err := excelize.CellNameToCoordinates(parts[1])
	if err != nil {
		return xml, false, nil
	}
	newEnd, err := excelize.CoordinatesToCellName(endCol, maxDataRow)
	if err != nil {
		return xml, false, nil
	}
	newDim := parts[0] + ":" + newEnd
	if newDim == dim {
		return xml, false, nil
	}
	out := dimensionRefRe.ReplaceAllString(s, `<dimension ref="`+newDim+`"`)
	return []byte(out), true, nil
}

func maxRowWithCellsInXML(s string) int {
	max := 0
	for _, m := range cellRefRe.FindAllStringSubmatch(s, -1) {
		if len(m) != 3 {
			continue
		}
		row, err := strconv.Atoi(m[2])
		if err == nil && row > max {
			max = row
		}
	}
	return max
}

func stripEmptyRows(xml []byte) ([]byte, bool, error) {
	s := string(xml)
	changed := false
	out := selfClosingRow.ReplaceAllStringFunc(s, func(string) string {
		changed = true
		return ""
	})
	out2 := emptyRow.ReplaceAllStringFunc(out, func(string) string {
		changed = true
		return ""
	})
	if !changed {
		return xml, false, nil
	}
	return []byte(out2), true, nil
}

func maxRowTagInXML(s string) int {
	max := 0
	for _, m := range rowTagRe.FindAllStringSubmatch(s, -1) {
		if len(m) != 2 {
			continue
		}
		row, err := strconv.Atoi(m[1])
		if err == nil && row > max {
			max = row
		}
	}
	return max
}

func repairWorksheetMergeCells(xml []byte) ([]byte, bool, error) {
	s := string(xml)
	if !strings.Contains(s, "<mergeCells") {
		return xml, false, nil
	}
	dim := ""
	if m := dimensionRefRe.FindStringSubmatch(s); len(m) == 2 {
		dim = m[1]
	}
	refs := mergeCellRefRe.FindAllStringSubmatch(s, -1)
	if len(refs) == 0 {
		return xml, false, nil
	}
	var repaired []string
	changed := false
	for _, m := range refs {
		ref := m[1]
		fixed := repairMergeRef(ref, dim)
		if fixed == ref {
			repaired = append(repaired, ref)
			continue
		}
		changed = true
		if fixed != "" {
			repaired = append(repaired, fixed)
		}
	}
	if !changed {
		return xml, false, nil
	}
	start := strings.Index(s, "<mergeCells")
	end := strings.Index(s[start:], "</mergeCells>")
	if start < 0 || end < 0 {
		return xml, false, nil
	}
	end += start + len("</mergeCells>")
	var replacement string
	if len(repaired) == 0 {
		replacement = ""
	} else {
		var b strings.Builder
		b.WriteString(`<mergeCells count="`)
		b.WriteString(strconv.Itoa(len(repaired)))
		b.WriteString(`">`)
		for _, ref := range repaired {
			b.WriteString(`<mergeCell ref="`)
			b.WriteString(ref)
			b.WriteString(`"/>`)
		}
		b.WriteString(`</mergeCells>`)
		replacement = b.String()
	}
	out := s[:start] + replacement + s[end:]
	return []byte(out), true, nil
}

func repairMergeRef(ref, dimension string) string {
	if validMergeRef(ref) {
		return ref
	}
	parts := strings.Split(ref, ":")
	if len(parts) != 2 {
		return ""
	}
	row1, err1 := strconv.Atoi(parts[0])
	row2, err2 := strconv.Atoi(parts[1])
	if err1 == nil && err2 == nil && row1 > 0 && row1 == row2 {
		endCol := lastColFromDimension(dimension)
		if endCol == 0 {
			endCol = 1
		}
		start, err := excelize.CoordinatesToCellName(1, row1)
		if err != nil {
			return ""
		}
		end, err := excelize.CoordinatesToCellName(endCol, row1)
		if err != nil {
			return ""
		}
		return start + ":" + end
	}
	return ""
}

func validMergeRef(ref string) bool {
	parts := strings.Split(ref, ":")
	if len(parts) == 1 {
		parts = append(parts, parts[0])
	}
	if len(parts) != 2 {
		return false
	}
	if _, _, err := excelize.CellNameToCoordinates(parts[0]); err != nil {
		return false
	}
	if _, _, err := excelize.CellNameToCoordinates(parts[1]); err != nil {
		return false
	}
	return true
}

func lastColFromDimension(dim string) int {
	if dim == "" {
		return 0
	}
	parts := strings.Split(dim, ":")
	if len(parts) != 2 {
		return 0
	}
	col, _, err := excelize.CellNameToCoordinates(parts[1])
	if err != nil {
		return 0
	}
	return col
}

// tightenSheetDimensions resets each sheet's used range to the actual populated rows
// so excelize save does not scan spurious million-row dimensions.
func tightenSheetDimensions(f *excelize.File) error {
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			return fmt.Errorf("sheet %q: %w", sheet, err)
		}
		if len(rows) == 0 {
			if err := f.SetSheetDimension(sheet, ""); err != nil {
				return fmt.Errorf("sheet %q: %w", sheet, err)
			}
			continue
		}
		minRow, maxRow := 0, 0
		maxCol := 0
		for r, row := range rows {
			for c, v := range row {
				if strings.TrimSpace(v) == "" {
					continue
				}
				rowNum := r + 1
				colNum := c + 1
				if minRow == 0 || rowNum < minRow {
					minRow = rowNum
				}
				if rowNum > maxRow {
					maxRow = rowNum
				}
				if colNum > maxCol {
					maxCol = colNum
				}
			}
		}
		if maxRow == 0 {
			if err := f.SetSheetDimension(sheet, ""); err != nil {
				return fmt.Errorf("sheet %q: %w", sheet, err)
			}
			continue
		}
		if maxCol == 0 {
			maxCol = 1
		}
		start, err := excelize.CoordinatesToCellName(1, minRow)
		if err != nil {
			return err
		}
		end, err := excelize.CoordinatesToCellName(maxCol, maxRow)
		if err != nil {
			return err
		}
		ref := start + ":" + end
		if err := f.SetSheetDimension(sheet, ref); err != nil {
			return fmt.Errorf("sheet %q: %w", sheet, err)
		}
	}
	return nil
}
