package template

import (
	"fmt"
	"strings"
)

// ValidateString checks that s compiles as a gonja template (no execution).
func (r *Renderer) ValidateString(s string) error {
	if !HasSyntax(s) {
		return nil
	}
	if _, err := r.getTemplate(s); err != nil {
		return fmt.Errorf("compile template: %w", err)
	}
	return nil
}

// ValidateExpression checks that expr compiles as a Jinja expression.
func (r *Renderer) ValidateExpression(expr string) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return fmt.Errorf("compile template: empty expression")
	}
	tplStr := fmt.Sprintf("{{ (%s) | tojson }}", expr)
	return r.ValidateString(tplStr)
}

// ValidateCondition checks that expr compiles inside an {% if %} tag.
func (r *Renderer) ValidateCondition(expr string) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return fmt.Errorf("compile template: empty condition")
	}
	tplStr := fmt.Sprintf("{%% if %s %%}true{%% else %%}false{%% endif %%}", expr)
	return r.ValidateString(tplStr)
}

// EachVarExpression calls fn for every {{ expr }} segment in s.
func EachVarExpression(s string, fn func(expr string)) {
	if !strings.Contains(s, "{{") {
		return
	}
	for _, m := range reVar.FindAllStringSubmatch(s, -1) {
		if len(m) == 2 {
			fn(strings.TrimSpace(m[1]))
		}
	}
}
