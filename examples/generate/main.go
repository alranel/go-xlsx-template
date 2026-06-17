// Command generate writes example template .xlsx files under examples/.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

func main() {
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := writeSimple(filepath.Join(root, "examples", "simple", "template.xlsx")); err != nil {
		fail(err)
	}
	if err := writeInvoice(filepath.Join(root, "examples", "invoice", "template.xlsx")); err != nil {
		fail(err)
	}
	fmt.Println("wrote examples/simple/template.xlsx")
	fmt.Println("wrote examples/invoice/template.xlsx")
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func writeSimple(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f := excelize.NewFile()
	sheet := "Sheet1"
	_ = f.SetCellValue(sheet, "A1", "Report:")
	_ = f.SetCellValue(sheet, "B1", "{{ title }}")
	_ = f.SetCellValue(sheet, "A2", "Year:")
	_ = f.SetCellValue(sheet, "B2", "{{ year }}")
	_ = f.SetCellValue(sheet, "A3", "Status:")
	_ = f.SetCellValue(sheet, "B3", "{% if active %}Active{% else %}Inactive{% endif %}")
	_ = f.SetCellValue(sheet, "A4", "Price: ")
	_ = f.SetCellFormula(sheet, "B4", "{{ price }}")
	_ = f.SetCellValue(sheet, "A5", "Price (2x): ")
	_ = f.SetCellFormula(sheet, "B5", "={{ price }}*2")
	return f.SaveAs(path)
}

func writeInvoice(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f := excelize.NewFile()
	const sheet = "Invoice"
	_ = f.SetSheetName("Sheet1", sheet)

	_ = f.SetCellValue(sheet, "A1", "Invoice #")
	_ = f.SetCellValue(sheet, "B1", "{{ invoice_no }}")
	_ = f.SetCellValue(sheet, "A2", "Customer")
	_ = f.SetCellValue(sheet, "B2", "{{ customer }}")
	_ = f.SetCellValue(sheet, "A3", "Date")
	_ = f.SetCellValue(sheet, "B3", "{{ date }}")

	_ = f.SetCellValue(sheet, "A5", "Description")
	_ = f.SetCellValue(sheet, "B5", "Qty")
	_ = f.SetCellValue(sheet, "C5", "Unit price")
	_ = f.SetCellValue(sheet, "D5", "Line total")

	_ = f.SetCellValue(sheet, "A6", "{%tr for line in lines %}")
	_ = f.SetCellValue(sheet, "A7", "{{ line.description }}")
	_ = f.SetCellValue(sheet, "B7", "{{ line.qty }}")
	_ = f.SetCellValue(sheet, "C7", "{{ line.unit_price }}")
	_ = f.SetCellFormula(sheet, "D7", "={{ line.qty }}*{{ line.unit_price }}")
	_ = f.SetCellValue(sheet, "A8", "{%tr endfor %}")

	_ = f.SetCellValue(sheet, "A9", "{%tr if show_notes %}")
	_ = f.SetCellValue(sheet, "A10", "Notes")
	_ = f.SetCellValue(sheet, "B10", "{{ notes }}")
	_ = f.SetCellValue(sheet, "A11", "Payment terms")
	_ = f.SetCellValue(sheet, "B11", "{{ payment_terms }}")
	_ = f.SetCellValue(sheet, "A12", "{%tr endif %}")

	_ = f.SetCellValue(sheet, "C14", "Subtotal")
	_ = f.SetCellFormula(sheet, "D14", "=SUM(D7:D9)")

	return f.SaveAs(path)
}
