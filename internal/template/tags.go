package template

import (
	"regexp"
	"strings"
)

var (
	reVar         = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)
	reTRFor       = regexp.MustCompile(`(?i)\{%\s*tr\s+for\s+(\w+)\s+in\s+(.+?)\s*%\}`)
	reTRIf        = regexp.MustCompile(`(?i)\{%\s*tr\s+if\s+(.+?)\s*%\}`)
	reTREndFor    = regexp.MustCompile(`(?i)\{%\s*tr\s+endfor\s*%\}`)
	reTRendif     = regexp.MustCompile(`(?i)\{%\s*tr\s+endif\s*%\}`)
	reTRAny       = regexp.MustCompile(`(?i)\{%\s*tr\s+`)
	rePlainFor    = regexp.MustCompile(`(?i)^\{%\s*for\s+(\w+)\s+in\s+([^%]+?)\s*%\}$`)
	rePlainIf     = regexp.MustCompile(`(?i)^\{%\s*if\s+([^%]+?)\s*%\}$`)
	rePlainEndFor = regexp.MustCompile(`(?i)^\{%\s*endfor\s*%\}$`)
	rePlainEndIf  = regexp.MustCompile(`(?i)^\{%\s*endif\s*%\}$`)
)

// TRMarkerKind classifies a {%tr %} row marker.
type TRMarkerKind int

const (
	TRNone TRMarkerKind = iota
	TRFor
	TRIf
	TREndFor
	TREndIf
)

// TRMarker describes a {%tr %} tag found on a row.
type TRMarker struct {
	Kind    TRMarkerKind
	Row     int
	ForVar  string
	ForExpr string
	IfExpr  string
}

// HasSyntax reports whether s contains template delimiters.
func HasSyntax(s string) bool {
	return strings.Contains(s, "{{") || strings.Contains(s, "{%")
}

// FindTRMarkerLine reports whether s is only a {%tr %} tag (optional whitespace).
func FindTRMarkerLine(s string) bool {
	return IsTRMarkerCell(s)
}

// IsTRMarkerCell reports whether cell text is only a recognized row loop/conditional marker
// ({%tr %} or plain {% for %}/{% if %}/{% endfor %}/{% endif %} on a marker-only row).
func IsTRMarkerCell(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	_, ok := FindTRMarker(s)
	return ok
}

// FindTRMarker inspects concatenated row text for a row loop/conditional marker.
func FindTRMarker(rowText string) (TRMarker, bool) {
	rowText = strings.TrimSpace(rowText)
	if rowText == "" {
		return TRMarker{}, false
	}
	if reTRAny.MatchString(rowText) {
		if reTREndFor.MatchString(rowText) {
			return TRMarker{Kind: TREndFor}, true
		}
		if reTRendif.MatchString(rowText) {
			return TRMarker{Kind: TREndIf}, true
		}
		if m := reTRFor.FindStringSubmatch(rowText); len(m) == 3 {
			return TRMarker{Kind: TRFor, ForVar: m[1], ForExpr: strings.TrimSpace(m[2])}, true
		}
		if m := reTRIf.FindStringSubmatch(rowText); len(m) == 2 {
			return TRMarker{Kind: TRIf, IfExpr: strings.TrimSpace(m[1])}, true
		}
		return TRMarker{}, false
	}
	if rePlainEndFor.MatchString(rowText) {
		return TRMarker{Kind: TREndFor}, true
	}
	if rePlainEndIf.MatchString(rowText) {
		return TRMarker{Kind: TREndIf}, true
	}
	if m := rePlainFor.FindStringSubmatch(rowText); len(m) == 3 {
		return TRMarker{Kind: TRFor, ForVar: m[1], ForExpr: strings.TrimSpace(m[2])}, true
	}
	if m := rePlainIf.FindStringSubmatch(rowText); len(m) == 2 {
		return TRMarker{Kind: TRIf, IfExpr: strings.TrimSpace(m[1])}, true
	}
	return TRMarker{}, false
}

// ReplaceVarSegments renders each {{ ... }} segment via renderFn and splices results.
func ReplaceVarSegments(s string, renderFn func(expr string) (string, error)) (string, error) {
	if !strings.Contains(s, "{{") {
		return s, nil
	}
	var err error
	out := reVar.ReplaceAllStringFunc(s, func(match string) string {
		if err != nil {
			return match
		}
		m := reVar.FindStringSubmatch(match)
		if len(m) != 2 {
			return match
		}
		rendered, e := renderFn(strings.TrimSpace(m[1]))
		if e != nil {
			err = e
			return match
		}
		return rendered
	})
	return out, err
}

// WholeCellVariable returns the expression if the cell is exactly one {{ expr }}.
func WholeCellVariable(s string) (string, bool) {
	s = strings.TrimSpace(s)
	m := regexp.MustCompile(`^\{\{\s*([^}]+?)\s*\}\}$`).FindStringSubmatch(s)
	if len(m) != 2 {
		return "", false
	}
	return strings.TrimSpace(m[1]), true
}
