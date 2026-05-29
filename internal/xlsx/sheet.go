package xlsx

import (
	"github.com/alranel/go-xlsx-template/internal/render"
	"github.com/alranel/go-xlsx-template/internal/template"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/xuri/excelize/v2"
)

func renderSheet(f *excelize.File, sheet string, r *template.Renderer, ctx *exec.Context, rep *render.Report) error {
	if err := escapeFormulaPlaceholders(f, sheet); err != nil {
		return err
	}
	if err := expandTRBlocks(f, sheet, r, ctx, rep); err != nil {
		return err
	}
	if err := renderSheetCells(f, sheet, r, ctx, rep); err != nil {
		return err
	}
	return renderComments(f, sheet, r, ctx, rep)
}
