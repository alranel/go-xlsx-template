# go-xlsx-template

Fill Excel `.xlsx` templates with JSON data using [Jinja-style](https://jinja.palletsprojects.com/) syntax, powered by [gonja](https://github.com/NikolaLohinski/gonja).

## Install

```bash
go install github.com/alranel/go-xlsx-template/cmd/go-xlsx-template@latest
```

Or build from source:

```bash
go build -o go-xlsx-template ./cmd/go-xlsx-template
```

## Usage

```bash
go-xlsx-template -template report.xlsx -data data.json -output filled.xlsx
```

### Fault-tolerant rendering (default)

By default the tool **always writes** `output` when rendering can continue. Recoverable problems (undefined variables, bad expressions, failed `{%tr %}` iterable/condition) leave the original template text in place and are collected as issues.

```bash
# Human summary on stderr when issues exist; still writes output
go-xlsx-template -template t.xlsx -data data.json -output out.xlsx

# Machine-readable issue report
go-xlsx-template ... -report issues.json

# Strict: no output file if any issue; exit 1
go-xlsx-template ... -strict
```

Fatal errors (invalid JSON, corrupt workbook, unclosed `{%tr %}` markers) abort with no output.

Gonja may render a missing top-level variable as an empty string without error; only compile/execute failures and invalid expressions are reported as issues.

### Library API

```go
res, err := xlsx.RenderFileWithResult("tpl.xlsx", "out.xlsx", ctx, xlsx.DefaultRenderOptions())
// res.Issues — recoverable problems; err — fatal or xlsx.ErrStrictIssues when Strict is true
```

See [examples/](examples/) for ready-made templates and sample JSON data.

## Examples

| Example | Command |
|---------|---------|
| [simple/](examples/simple/) | `go-xlsx-template -template examples/simple/template.xlsx -data examples/simple/data.json -output examples/simple/output.xlsx` |
| [invoice/](examples/invoice/) | `go-xlsx-template -template examples/invoice/template.xlsx -data examples/invoice/data.json -output examples/invoice/output.xlsx` |

The JSON file must be a single object at the root. Supported value types: strings, numbers, booleans, arrays, and nested objects.

### Example `data.json`

```json
{
  "title": "Q1 Report",
  "year": 2025,
  "active": true,
  "price": 12.5,
  "items": [
    { "name": "Apples", "qty": 3 },
    { "name": "Pears", "qty": 5 }
  ]
}
```

## Template syntax

### Variables

```
{{ title }}
{{ items[0].name }}
```

### Formulas

Placeholders work inside formulas:

```
={{ price }}*2
=SUM(B2:B10)
```

### Cell conditionals

Tags are evaluated and removed; inner text follows normal Jinja rules:

```
{% if active %}Yes{% else %}No{% endif %}
```

### Row loops (`{%tr %}`)

Put `{%tr for %}` / `{%tr endfor %}` on **marker-only rows** (no data in other columns on the same row). The rows strictly between the markers are repeated for each element.

| Row | Content |
|-----|---------|
| 1 | `{%tr for item in items %}` |
| 2 | `{{ item.name }}` \| `{{ item.qty }}` |
| 3 | `{%tr endfor %}` |

### Row conditionals (`{%tr %}`)

| Row | Content |
|-----|---------|
| 1 | `{%tr if active %}` |
| 2 | Optional content |
| 3 | `{%tr endif %}` |

If the condition is false, the whole block (markers and body) is removed.

### Sheet names

```
Report {{ year }}
```

Sheet names are rendered after sheet content. Excel limits names to 31 characters; invalid rendered names are left unchanged and recorded as issues.

## Filters and expressions

Any [gonja](https://github.com/NikolaLohinski/gonja) filter or expression supported by the engine works in `{{ }}` and `{% %}` tags, for example `{{ name | upper }}` or `{% if count > 0 %}`.

## Limitations

- **Marker rows**: `{%tr %}` rows should contain only tags (one marker per row). Mixing markers and data on the same row can break row expansion.
- **Formulas below loops**: excelize shifts formula row/column references when rows are inserted, but it does not widen fixed ranges (e.g. `SUM(B2:B2)` stays `B2:B2`). Size total ranges in the template to fit the maximum expanded row count.
- **Formula placeholders and row loops**: Before `{%tr %}` row insert/remove, cells with `={{ ... }}` formulas are temporarily stored as plain text (prefix `'` + `__go_xlsx_template:fml:`) so excelize can adjust rows safely. They are turned back into real formulas when placeholders are evaluated, including loop variables such as `item` inside `{%tr for %}` bodies.
- **Charts and drawings**: Not updated when rows change.
- **Rich text**: Keep `{{ }}` / `{% %}` inside a single rich-text run when possible; otherwise runs are merged for rendering.
- **Sheet rename**: Formulas that reference sheets by name are not rewritten after a tab is renamed.
- **Comments**: Row insert/remove adjusts comment positions manually; behavior may differ from Excel for complex layouts.

## Development

```bash
go test ./...
```

## Author

Designed by Alessandro Ranellucci, developed by an AI tool

## License

MIT
