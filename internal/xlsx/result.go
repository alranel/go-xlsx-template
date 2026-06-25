package xlsx

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/alranel/go-xlsx-template/internal/render"
	"github.com/alranel/go-xlsx-template/internal/template"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/xuri/excelize/v2"
)

// ErrStrictIssues is returned when -strict is set and recoverable issues were recorded.
var ErrStrictIssues = errors.New("render completed with issues (strict mode)")

// RenderOptions configures fault-tolerant rendering.
type RenderOptions struct {
	// Strict aborts before saving when any recoverable issue was recorded.
	Strict bool
}

// DefaultRenderOptions returns the default lenient options (always save unless fatal).
func DefaultRenderOptions() RenderOptions {
	return RenderOptions{Strict: false}
}

// Result is returned by RenderFileWithResult.
type Result struct {
	OutputPath string
	Issues     []render.Issue
}

// RenderFile renders a template workbook (lenient; use RenderFileWithResult for issues).
func RenderFile(templatePath, outputPath string, ctx *exec.Context) error {
	_, err := RenderFileWithResult(templatePath, outputPath, ctx, DefaultRenderOptions())
	return err
}

// RenderFileWithResult renders the template and returns recoverable issues separately from fatal errors.
func RenderFileWithResult(templatePath, outputPath string, ctx *exec.Context, opts RenderOptions) (Result, error) {
	f, err := openTemplate(templatePath)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()

	rep := render.NewReport()
	r := template.NewRenderer()
	if err := renderWorkbook(f, r, ctx, rep); err != nil {
		return Result{Issues: rep.Issues()}, err
	}
	if opts.Strict && rep.HasIssues() {
		return Result{Issues: rep.Issues()}, ErrStrictIssues
	}
	// Spreadsheet apps may show stale <v> cached results instead of recalculating
	// after we rewrite formula text (e.g. templates edited in LibreOffice).
	if err := f.UpdateLinkedValue(); err != nil {
		return Result{Issues: rep.Issues()}, fmt.Errorf("clear formula cache: %w", err)
	}
	if err := tightenSheetDimensions(f); err != nil {
		return Result{Issues: rep.Issues()}, fmt.Errorf("tighten sheet dimensions: %w", err)
	}
	if err := f.SaveAs(outputPath); err != nil {
		return Result{Issues: rep.Issues()}, fmt.Errorf("save output: %w", err)
	}
	return Result{
		OutputPath: outputPath,
		Issues:     rep.Issues(),
	}, nil
}

func openTemplate(path string) (*excelize.File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open template: %w", err)
	}
	data, err = repairWorkbookBytes(data)
	if err != nil {
		return nil, fmt.Errorf("repair template: %w", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("open template: %w", err)
	}
	return f, nil
}
