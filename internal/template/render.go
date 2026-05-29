package template

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
)

// Renderer evaluates Jinja-style templates via gonja.
type Renderer struct {
	cache sync.Map
}

// NewRenderer returns a Renderer with a template compile cache.
func NewRenderer() *Renderer {
	return &Renderer{}
}

func (r *Renderer) getTemplate(s string) (*exec.Template, error) {
	if v, ok := r.cache.Load(s); ok {
		return v.(*exec.Template), nil
	}
	tpl, err := gonja.FromString(s)
	if err != nil {
		return nil, err
	}
	r.cache.Store(s, tpl)
	return tpl, nil
}

// RenderString executes a template string with ctx.
func (r *Renderer) RenderString(s string, ctx *exec.Context) (string, error) {
	if !HasSyntax(s) {
		return s, nil
	}
	tpl, err := r.getTemplate(s)
	if err != nil {
		return "", fmt.Errorf("compile template: %w", err)
	}
	out, err := tpl.ExecuteToString(ctx)
	if err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return out, nil
}

// EvalCondition evaluates a boolean Jinja expression.
func (r *Renderer) EvalCondition(expr string, ctx *exec.Context) (bool, error) {
	expr = strings.TrimSpace(expr)
	tplStr := fmt.Sprintf("{%% if %s %%}true{%% else %%}false{%% endif %%}", expr)
	out, err := r.RenderString(tplStr, ctx)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}

// EvalExpression evaluates an expression to a Go value using tojson round-trip.
func (r *Renderer) EvalExpression(expr string, ctx *exec.Context) (any, error) {
	expr = strings.TrimSpace(expr)
	tplStr := fmt.Sprintf("{{ (%s) | tojson }}", expr)
	out, err := r.RenderString(tplStr, ctx)
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	var v any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return nil, fmt.Errorf("eval expression %q: invalid json %q: %w", expr, out, err)
	}
	return v, nil
}

// EvalIterable evaluates an expression that must yield a sequence (not a string).
func (r *Renderer) EvalIterable(expr string, ctx *exec.Context) ([]any, error) {
	v, err := r.EvalExpression(expr, ctx)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	if arr, ok := v.([]any); ok {
		return arr, nil
	}
	return nil, fmt.Errorf("expression %q is not iterable (got %T)", expr, v)
}

// RenderValue renders a whole-cell {{ expr }} and returns the native JSON-like value when possible.
func (r *Renderer) RenderValue(expr string, ctx *exec.Context) (any, error) {
	return r.EvalExpression(expr, ctx)
}
