package template

import (
	"strings"

	"github.com/alranel/go-xlsx-template/internal/render"
	"github.com/nikolalohinski/gonja/v2/exec"
)

func issueKind(err error) render.IssueKind {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "compile template"):
		return render.KindCompile
	case strings.Contains(msg, "execute template"):
		return render.KindExecute
	default:
		return render.KindExpression
	}
}

// RenderStringLenient renders s or returns the original on failure, recording an issue.
func (r *Renderer) RenderStringLenient(s string, loc render.Location, ctx *exec.Context, rep *render.Report) string {
	if !HasSyntax(s) {
		return s
	}
	out, err := r.RenderString(s, ctx)
	if err != nil {
		rep.Add(loc, issueKind(err), s, err)
		return s
	}
	return out
}

// RenderValueLenient evaluates a whole-cell variable or records an issue.
func (r *Renderer) RenderValueLenient(expr string, loc render.Location, ctx *exec.Context, rep *render.Report) (any, bool) {
	v, err := r.RenderValue(expr, ctx)
	if err != nil {
		rep.Add(loc, issueKind(err), "{{ "+expr+" }}", err)
		return nil, false
	}
	return v, true
}

// EvalIterableLenient evaluates a for-expression or records an issue and reports failure.
func (r *Renderer) EvalIterableLenient(expr string, loc render.Location, ctx *exec.Context, rep *render.Report) ([]any, bool) {
	items, err := r.EvalIterable(expr, ctx)
	if err != nil {
		rep.Add(loc, render.KindRowBlock, expr, err)
		return nil, false
	}
	return items, true
}

// EvalConditionLenient evaluates a tr-if condition or records an issue and reports failure.
func (r *Renderer) EvalConditionLenient(expr string, loc render.Location, ctx *exec.Context, rep *render.Report) (bool, bool) {
	ok, err := r.EvalCondition(expr, ctx)
	if err != nil {
		rep.Add(loc, render.KindRowBlock, expr, err)
		return false, false
	}
	return ok, true
}

// ReplaceVarSegmentsLenient substitutes {{ }} segments; failed segments stay unchanged.
func ReplaceVarSegmentsLenient(s string, renderFn func(expr string) (string, error)) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	return reVar.ReplaceAllStringFunc(s, func(match string) string {
		m := reVar.FindStringSubmatch(match)
		if len(m) != 2 {
			return match
		}
		rendered, err := renderFn(strings.TrimSpace(m[1]))
		if err != nil {
			return match
		}
		return rendered
	})
}
