package data

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/nikolalohinski/gonja/v2/exec"
)

// LoadContext reads a JSON object from path and builds a gonja execution context.
func LoadContext(path string) (*exec.Context, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadContextFromReader(f)
}

// LoadContextFromReader decodes a JSON object from r into a gonja context.
func LoadContextFromReader(r io.Reader) (*exec.Context, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	var raw map[string]any
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	if raw == nil {
		return nil, fmt.Errorf("json root must be an object")
	}
	return exec.NewContext(normalizeMap(raw)), nil
}

func normalizeMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = normalizeValue(v)
	}
	return out
}

func normalizeValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return normalizeMap(x)
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = normalizeValue(item)
		}
		return out
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		f, err := x.Float64()
		if err != nil {
			return x.String()
		}
		return f
	default:
		return v
	}
}

// ToJSONString is a test helper that round-trips values through JSON numbers.
func ToJSONString(v any) string {
	switch x := v.(type) {
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(x, 10)
	default:
		return fmt.Sprint(v)
	}
}
