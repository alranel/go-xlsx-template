package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/alranel/go-xlsx-template/internal/data"
	"github.com/alranel/go-xlsx-template/internal/render"
	"github.com/alranel/go-xlsx-template/internal/xlsx"
)

func main() {
	templatePath := flag.String("template", "", "path to the .xlsx template file")
	dataPath := flag.String("data", "", "path to the JSON data file")
	outputPath := flag.String("output", "", "path for the rendered .xlsx output")
	reportPath := flag.String("report", "", "write issues as JSON to this path (render mode) or override validation JSON destination")
	strict := flag.Bool("strict", false, "exit with error if any recoverable issue was recorded (output is not written)")
	validate := flag.Bool("validate", false, "validate template syntax only (-template required; no -data or -output)")
	flag.Parse()

	if *validate {
		runValidate(*templatePath, *dataPath, *outputPath, *reportPath)
		return
	}

	if *templatePath == "" || *dataPath == "" || *outputPath == "" {
		fmt.Fprintln(os.Stderr, "usage: go-xlsx-template -template <file.xlsx> -data <file.json> -output <file.xlsx>")
		fmt.Fprintln(os.Stderr, "       go-xlsx-template -validate -template <file.xlsx>")
		flag.PrintDefaults()
		os.Exit(2)
	}

	ctx, err := data.LoadContext(*dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load data: %v\n", err)
		os.Exit(1)
	}

	opts := xlsx.RenderOptions{Strict: *strict}
	res, err := xlsx.RenderFileWithResult(*templatePath, *outputPath, ctx, opts)
	if err != nil {
		writeRenderReport(*reportPath, res.Issues)
		if summary := render.SummaryFrom(res.Issues); summary != "" {
			fmt.Fprint(os.Stderr, summary)
		}
		fmt.Fprintf(os.Stderr, "render: %v\n", err)
		os.Exit(1)
	}

	writeRenderReport(*reportPath, res.Issues)
	if summary := render.SummaryFrom(res.Issues); summary != "" {
		fmt.Fprint(os.Stderr, summary)
	}
}

func runValidate(templatePath, dataPath, outputPath, reportPath string) {
	if templatePath == "" {
		fmt.Fprintln(os.Stderr, "usage: go-xlsx-template -validate -template <file.xlsx>")
		flag.PrintDefaults()
		os.Exit(2)
	}
	if dataPath != "" || outputPath != "" {
		fmt.Fprintln(os.Stderr, "validate mode: -data and -output are not used")
		os.Exit(2)
	}

	res, err := xlsx.ValidateFile(templatePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validate: %v\n", err)
		os.Exit(1)
	}

	out := os.Stdout
	if reportPath != "" {
		f, err := os.Create(reportPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "report: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		out = f
	}

	if err := json.NewEncoder(out).Encode(res); err != nil {
		fmt.Fprintf(os.Stderr, "report: %v\n", err)
		os.Exit(1)
	}
	if reportPath != "" {
		// When writing to a file, also mirror a compact line to stderr for humans in CI logs.
		if !res.Valid {
			fmt.Fprintf(os.Stderr, "invalid: %d issue(s), see %s\n", res.IssueCount, reportPath)
		}
	}

	if !res.Valid {
		os.Exit(1)
	}
}

func writeRenderReport(path string, issues []render.Issue) {
	if path == "" {
		return
	}
	payload := struct {
		IssueCount int            `json:"issue_count"`
		Issues     []render.Issue `json:"issues"`
	}{
		IssueCount: len(issues),
		Issues:     issues,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "report: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "report: %v\n", err)
		os.Exit(1)
	}
}
