package xlsx

import (
	"fmt"
	"strings"

	"github.com/alranel/go-xlsx-template/internal/template"
	"github.com/xuri/excelize/v2"
)

// ValidationKind classifies a template validation problem.
type ValidationKind string

const (
	ValidationCompile    ValidationKind = "compile"
	ValidationStructure  ValidationKind = "structure"
	ValidationTRMarker   ValidationKind = "tr_marker"
)

// ValidationIssue describes one problem found during validation.
type ValidationIssue struct {
	Sheet   string         `json:"sheet,omitempty"`
	Cell    string         `json:"cell,omitempty"`
	Row     int            `json:"row,omitempty"`
	Kind    ValidationKind `json:"kind"`
	Source  string         `json:"source,omitempty"`
	Message string         `json:"message"`
}

// ValidationResult is the machine-readable outcome of ValidateFile.
type ValidationResult struct {
	Valid      bool              `json:"valid"`
	IssueCount int               `json:"issue_count"`
	Issues     []ValidationIssue `json:"issues"`
}

// ValidateFile checks template syntax and {%tr %} structure without JSON data.
func ValidateFile(path string) (ValidationResult, error) {
	f, err := openTemplate(path)
	if err != nil {
		return ValidationResult{}, err
	}
	defer f.Close()

	r := template.NewRenderer()
	collector := &validationCollector{}
	for _, sheet := range f.GetSheetList() {
		validateSheet(f, sheet, r, collector)
	}
	for _, name := range f.GetSheetList() {
		if template.HasSyntax(name) {
			loc := valLoc{sheet: name}
			collector.compile(r, loc, name, r.ValidateString(name))
		}
	}
	return collector.result(), nil
}

type valLoc struct {
	sheet string
	cell  string
	row   int
}

type validationCollector struct {
	issues []ValidationIssue
}

func (c *validationCollector) result() ValidationResult {
	return ValidationResult{
		Valid:      len(c.issues) == 0,
		IssueCount: len(c.issues),
		Issues:     c.issues,
	}
}

func (c *validationCollector) add(loc valLoc, kind ValidationKind, source, message string) {
	c.issues = append(c.issues, ValidationIssue{
		Sheet:   loc.sheet,
		Cell:    loc.cell,
		Row:     loc.row,
		Kind:    kind,
		Source:  source,
		Message: message,
	})
}

func (c *validationCollector) compile(r *template.Renderer, loc valLoc, source string, err error) {
	if err == nil {
		return
	}
	c.add(loc, ValidationCompile, source, err.Error())
}

func validateSheet(f *excelize.File, sheet string, r *template.Renderer, c *validationCollector) {
	validateTRStructure(f, sheet, r, c)
	if err := forEachUsedCell(f, sheet, func(cell string) error {
		validateCell(f, sheet, cell, r, c)
		return nil
	}); err != nil {
		c.add(valLoc{sheet: sheet}, ValidationStructure, sheet, err.Error())
	}
	validateComments(f, sheet, r, c)
}

func validateTRStructure(f *excelize.File, sheet string, r *template.Renderer, c *validationCollector) {
	maxRow, err := sheetMaxRow(f, sheet)
	if err != nil {
		c.add(valLoc{sheet: sheet}, ValidationStructure, sheet, err.Error())
		return
	}
	for row := 1; row <= maxRow; row++ {
		text, err := rowText(f, sheet, row)
		if err != nil {
			c.add(valLoc{sheet: sheet, row: row}, ValidationStructure, "", err.Error())
			continue
		}
		if template.FindTRMarkerLine(text) {
			continue
		}
		if strings.Contains(strings.ToLower(text), "{%tr") {
			loc := valLoc{sheet: sheet, row: row}
			c.add(loc, ValidationTRMarker, text, "invalid or unrecognized {%tr %} marker")
		}
	}

	blocks, err := findTRBlocks(f, sheet)
	if err != nil {
		c.add(valLoc{sheet: sheet}, ValidationStructure, "", err.Error())
		return
	}
	for _, b := range blocks {
		loc := valLoc{sheet: sheet, row: b.startRow}
		switch b.kind {
		case template.TRFor:
			source := fmt.Sprintf("{%%tr for %s in %s %%}", b.forVar, b.forExpr)
			c.compileExpr(r, loc, source, b.forExpr)
		case template.TRIf:
			source := fmt.Sprintf("{%%tr if %s %%}", b.ifExpr)
			c.compileCond(r, loc, source, b.ifExpr)
		}
	}
}

func (c *validationCollector) compileExpr(r *template.Renderer, loc valLoc, source, expr string) {
	c.compile(r, loc, source, r.ValidateExpression(expr))
}

func (c *validationCollector) compileCond(r *template.Renderer, loc valLoc, source, expr string) {
	c.compile(r, loc, source, r.ValidateCondition(expr))
}

func validateCell(f *excelize.File, sheet, cell string, r *template.Renderer, c *validationCollector) {
	loc := valLoc{sheet: sheet, cell: cell}
	val, err := f.GetCellValue(sheet, cell)
	if err != nil {
		c.add(loc, ValidationStructure, cell, err.Error())
		return
	}
	if raw, ok := UnwrapEscapedFormula(val); ok {
		validateTemplateText(r, loc, raw, c)
		return
	}
	if runs, err := f.GetCellRichText(sheet, cell); err == nil && len(runs) > 0 {
		combined := ""
		for _, run := range runs {
			combined += run.Text
		}
		validateTemplateText(r, loc, combined, c)
		return
	}
	formula, _ := f.GetCellFormula(sheet, cell)
	raw := formula
	if raw == "" {
		raw = val
	}
	if raw != "" {
		validateTemplateText(r, loc, raw, c)
	}
}

func validateComments(f *excelize.File, sheet string, r *template.Renderer, c *validationCollector) {
	comments, err := f.GetComments(sheet)
	if err != nil {
		c.add(valLoc{sheet: sheet}, ValidationStructure, sheet, err.Error())
		return
	}
	for _, cm := range comments {
		loc := valLoc{sheet: sheet, cell: cm.Cell}
		if template.HasSyntax(cm.Text) {
			validateTemplateText(r, loc, cm.Text, c)
		}
		for _, p := range cm.Paragraph {
			if template.HasSyntax(p.Text) {
				validateTemplateText(r, loc, p.Text, c)
			}
		}
	}
}

func validateTemplateText(r *template.Renderer, loc valLoc, s string, c *validationCollector) {
	if !template.HasSyntax(s) {
		return
	}
	if isFormulaTemplate(s) {
		template.EachVarExpression(s, func(expr string) {
			source := "{{ " + expr + " }}"
			c.compileExpr(r, loc, source, expr)
		})
		return
	}
	if expr, ok := template.WholeCellVariable(s); ok {
		source := "{{ " + expr + " }}"
		c.compileExpr(r, loc, source, expr)
		return
	}
	c.compile(r, loc, s, r.ValidateString(s))
}

func isFormulaTemplate(s string) bool {
	s = strings.TrimSpace(s)
	return len(s) > 0 && s[0] == '=' && strings.Contains(s, "{{")
}
