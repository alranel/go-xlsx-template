package xlsx

import (
	"github.com/alranel/go-xlsx-template/internal/render"
	"github.com/alranel/go-xlsx-template/internal/template"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/xuri/excelize/v2"
)

func renderRichCell(f *excelize.File, sheet, cell string, r *template.Renderer, ctx *exec.Context, rep *render.Report) (bool, error) {
	runs, err := f.GetCellRichText(sheet, cell)
	if err != nil {
		return false, err
	}
	if len(runs) == 0 {
		return false, nil
	}
	combined := ""
	for _, run := range runs {
		combined += run.Text
	}
	if !template.HasSyntax(combined) {
		return false, nil
	}
	loc := render.Location{Sheet: sheet, Cell: cell}
	if expr, ok := template.WholeCellVariable(combined); ok {
		v, ok := r.RenderValueLenient(expr, loc, ctx, rep)
		if !ok {
			return true, nil
		}
		return true, setRenderedScalar(f, sheet, cell, v)
	}
	rendered := r.RenderStringLenient(combined, loc, ctx, rep)
	font := runs[0].Font
	if font == nil && len(runs) > 0 {
		font = runs[len(runs)-1].Font
	}
	newRuns := []excelize.RichTextRun{{Text: rendered, Font: font}}
	if err := f.SetCellRichText(sheet, cell, newRuns); err != nil {
		return false, err
	}
	return true, nil
}
