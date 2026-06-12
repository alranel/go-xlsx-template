# Invoice example

Line-item invoice with a `{%tr for %}` row loop, a multi-row `{%tr if %}` block, and per-line formulas.

## Files

| File | Purpose |
|------|---------|
| `template.xlsx` | Excel template |
| `data.json` | Sample invoice data |

## Template layout

| Row | Column A | B | C | D |
|-----|----------|---|---|---|
| 1 | Invoice # | `{{ invoice_no }}` | | |
| 2 | Customer | `{{ customer }}` | | |
| 3 | Date | `{{ date }}` | | |
| 5 | Headers (Description, Qty, …) | | | |
| 6 | `{%tr for line in lines %}` (marker only) | | | |
| 7 | `{{ line.description }}` | qty | unit price | `={{ line.qty }}*{{ line.unit_price }}` |
| 8 | `{%tr endfor %}` (marker only) | | | |
| 9 | `{%tr if show_notes %}` (marker only) | | | |
| 10 | Notes | `{{ notes }}` | | |
| 11 | Payment terms | `{{ payment_terms }}` | | |
| 12 | `{%tr endif %}` (marker only) | | | |
| 14 | | | Subtotal | `=SUM(D7:D9)` (size range for max line count) |

## Run

From the repository root:

```bash
go run ./cmd/go-xlsx-template \
  -template examples/invoice/template.xlsx \
  -data examples/invoice/data.json \
  -output examples/invoice/output.xlsx
```

The output workbook should have three line rows, optional notes rows (when `show_notes` is true), and invoice **INV-1042** in cell B1.

Set `"show_notes": false` in `data.json` to verify the whole notes block (both content rows and markers) is removed.

## Regenerate the template

```bash
go run ./examples/generate
```
