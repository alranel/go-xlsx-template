package xlsx

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/alranel/go-xlsx-template/internal/render"
	"github.com/alranel/go-xlsx-template/internal/template"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/xuri/excelize/v2"
)

const maxSheetNameLen = 31

func renderWorkbook(f *excelize.File, r *template.Renderer, ctx *exec.Context, rep *render.Report) error {
	sheets := f.GetSheetList()
	for _, sheet := range sheets {
		if err := renderSheet(f, sheet, r, ctx, rep); err != nil {
			return fmt.Errorf("sheet %q: %w", sheet, err)
		}
	}
	return renderSheetNames(f, r, ctx, rep)
}

func renderSheetNames(f *excelize.File, r *template.Renderer, ctx *exec.Context, rep *render.Report) error {
	for _, oldName := range f.GetSheetList() {
		if !template.HasSyntax(oldName) {
			continue
		}
		loc := render.Location{Sheet: oldName}
		newName := r.RenderStringLenient(oldName, loc, ctx, rep)
		newName = strings.TrimSpace(newName)
		if newName == "" {
			rep.Add(loc, render.KindSheetName, oldName, fmt.Errorf("sheet name rendered to empty string"))
			continue
		}
		if utf8.RuneCountInString(newName) > maxSheetNameLen {
			rep.Add(loc, render.KindSheetName, oldName,
				fmt.Errorf("sheet name exceeds %d characters after render (%q)", maxSheetNameLen, newName))
			continue
		}
		if newName == oldName {
			continue
		}
		if err := f.SetSheetName(oldName, newName); err != nil {
			return fmt.Errorf("rename sheet %q -> %q: %w", oldName, newName, err)
		}
	}
	return nil
}
