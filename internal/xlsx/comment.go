package xlsx

import (
	"fmt"

	"github.com/alranel/go-xlsx-template/internal/render"
	"github.com/alranel/go-xlsx-template/internal/template"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/xuri/excelize/v2"
)

func shiftComments(f *excelize.File, sheet string, pivotRow, delta int) error {
	comments, err := f.GetComments(sheet)
	if err != nil {
		return err
	}
	if len(comments) == 0 || delta == 0 {
		return nil
	}
	for _, c := range comments {
		col, row, err := excelize.CellNameToCoordinates(c.Cell)
		if err != nil {
			continue
		}
		if err := f.DeleteComment(sheet, c.Cell); err != nil {
			return fmt.Errorf("delete comment %s: %w", c.Cell, err)
		}
		newRow := row
		if row >= pivotRow {
			newRow = row + delta
			if newRow < 1 {
				continue
			}
		}
		newCell, err := excelize.CoordinatesToCellName(col, newRow)
		if err != nil {
			return err
		}
		c.Cell = newCell
		if err := f.AddComment(sheet, c); err != nil {
			return fmt.Errorf("add comment %s: %w", newCell, err)
		}
	}
	return nil
}

func renderComments(f *excelize.File, sheet string, r *template.Renderer, ctx *exec.Context, rep *render.Report) error {
	comments, err := f.GetComments(sheet)
	if err != nil {
		return err
	}
	for _, c := range comments {
		loc := render.Location{Sheet: sheet, Cell: c.Cell}
		changed := false
		if template.HasSyntax(c.Text) {
			c.Text = r.RenderStringLenient(c.Text, loc, ctx, rep)
			changed = true
		}
		for i := range c.Paragraph {
			if !template.HasSyntax(c.Paragraph[i].Text) {
				continue
			}
			c.Paragraph[i].Text = r.RenderStringLenient(c.Paragraph[i].Text, loc, ctx, rep)
			changed = true
		}
		if !changed {
			continue
		}
		if err := f.DeleteComment(sheet, c.Cell); err != nil {
			return err
		}
		if err := f.AddComment(sheet, c); err != nil {
			return err
		}
	}
	return nil
}
