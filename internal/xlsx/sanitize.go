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
