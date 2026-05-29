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
	reportPath := flag.String("report", "", "write recoverable issues as JSON to this path")
	strict := flag.Bool("strict", false, "exit with error if any recoverable issue was recorded (output is not written)")
	flag.Parse()

	if *templatePath == "" || *dataPath == "" || *outputPath == "" {
		fmt.Fprintln(os.Stderr, "usage: go-xlsx-template -template <file.xlsx> -data <file.json> -output <file.xlsx>")
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
		writeReport(*reportPath, res.Issues)
		if summary := render.SummaryFrom(res.Issues); summary != "" {
			fmt.Fprint(os.Stderr, summary)
		}
		fmt.Fprintf(os.Stderr, "render: %v\n", err)
		os.Exit(1)
	}

	writeReport(*reportPath, res.Issues)
	if summary := render.SummaryFrom(res.Issues); summary != "" {
		fmt.Fprint(os.Stderr, summary)
	}
}

func writeReport(path string, issues []render.Issue) {
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
