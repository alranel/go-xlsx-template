package render

import (
	"encoding/json"
	"fmt"
	"strings"
)

// IssueKind classifies a non-fatal template problem.
type IssueKind string

const (
	KindCompile     IssueKind = "compile"
	KindExecute     IssueKind = "execute"
	KindExpression  IssueKind = "expression"
	KindSheetName   IssueKind = "sheet_name"
	KindRowBlock    IssueKind = "row_block"
)

// Location identifies where a template fragment was found.
type Location struct {
	Sheet string `json:"sheet,omitempty"`
	Cell  string `json:"cell,omitempty"`
	Row   int    `json:"row,omitempty"`
}

// Issue records one recoverable rendering problem.
type Issue struct {
	Sheet   string    `json:"sheet,omitempty"`
	Cell    string    `json:"cell,omitempty"`
	Row     int       `json:"row,omitempty"`
	Kind    IssueKind `json:"kind"`
	Source  string    `json:"source,omitempty"`
	Message string    `json:"message"`
}

// Report collects recoverable issues during rendering.
type Report struct {
	issues []Issue
}

// NewReport returns an empty issue report.
func NewReport() *Report {
	return &Report{}
}

// Add records an issue.
func (r *Report) Add(loc Location, kind IssueKind, source string, err error) {
	if r == nil || err == nil {
		return
	}
	r.issues = append(r.issues, Issue{
		Sheet:   loc.Sheet,
		Cell:    loc.Cell,
		Row:     loc.Row,
		Kind:    kind,
		Source:  source,
		Message: err.Error(),
	})
}

// Issues returns a copy of collected issues.
func (r *Report) Issues() []Issue {
	if r == nil || len(r.issues) == 0 {
		return nil
	}
	out := make([]Issue, len(r.issues))
	copy(out, r.issues)
	return out
}

// HasIssues reports whether any issue was recorded.
func (r *Report) HasIssues() bool {
	return r != nil && len(r.issues) > 0
}

// Count returns the number of issues.
func (r *Report) Count() int {
	if r == nil {
		return 0
	}
	return len(r.issues)
}

// SummaryFrom formats a human-readable summary of issues.
func SummaryFrom(issues []Issue) string {
	if len(issues) == 0 {
		return ""
	}
	return (&Report{issues: append([]Issue(nil), issues...)}).HumanSummary()
}

// HumanSummary formats issues for stderr or logs.
func (r *Report) HumanSummary() string {
	if !r.HasIssues() {
		return ""
	}
	var b strings.Builder
	for _, iss := range r.issues {
		where := iss.Sheet
		if iss.Cell != "" {
			where += "!" + iss.Cell
		} else if iss.Row > 0 {
			where += fmt.Sprintf(" row %d", iss.Row)
		}
		fmt.Fprintf(&b, "%s: [%s] %s", where, iss.Kind, iss.Message)
		if iss.Source != "" && !strings.Contains(iss.Message, iss.Source) {
			fmt.Fprintf(&b, " (source: %q)", truncateSource(iss.Source))
		}
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "%d issue(s)\n", len(r.issues))
	return b.String()
}

// MarshalJSON implements machine-readable export of all issues.
func (r *Report) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		IssueCount int     `json:"issue_count"`
		Issues     []Issue `json:"issues"`
	}{
		IssueCount: r.Count(),
		Issues:     r.Issues(),
	})
}

func truncateSource(s string) string {
	const max = 80
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
