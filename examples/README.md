# Examples

| Example | What it demonstrates |
|---------|----------------------|
| [simple/](simple/) | Variables, `{% if %}`, formula placeholder (`={{ price }}*2`) |
| [invoice/](invoice/) | `{%tr for %}` row loop, per-line formulas, multiple columns |

## Quick start

Build the tool (from the repo root):

```bash
go build -o go-xlsx-template ./cmd/go-xlsx-template
```

Render both examples:

```bash
go-xlsx-template -template examples/simple/template.xlsx \
  -data examples/simple/data.json \
  -output examples/simple/output.xlsx

go-xlsx-template -template examples/invoice/template.xlsx \
  -data examples/invoice/data.json \
  -output examples/invoice/output.xlsx
```

Generated `output.xlsx` files are gitignored; open them in Excel or LibreOffice after running the commands above.

## Regenerate template workbooks

Example `.xlsx` templates are produced by a small generator so they stay reproducible:

```bash
go run ./examples/generate
```

Commit updated `template.xlsx` files when you change the generator.
