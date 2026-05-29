# Simple report example

A minimal template with variables, a cell conditional, and a formula placeholder (`={{ price }}*2`).

## Files

| File | Purpose |
|------|---------|
| `template.xlsx` | Excel template |
| `data.json` | Sample data |

## Run

From the repository root:

```bash
go run ./cmd/go-xlsx-template \
  -template examples/simple/template.xlsx \
  -data examples/simple/data.json \
  -output examples/simple/output.xlsx
```

Open `output.xlsx` in Excel or LibreOffice. You should see the title, year, active status, and `price * 2` in cell B4.

## Regenerate the template

If you change the generator, rebuild the `.xlsx` file:

```bash
go run ./examples/generate
```
