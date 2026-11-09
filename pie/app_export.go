package main

import (
	"fmt"
	"os"
	"strings"

	"agile-suite/pie/internal/workbook"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ExportWorkbook asks where to save the sheets and writes them there as an
// .xlsx. It answers with the path written, or "" when the analyst cancelled.
func (a *App) ExportWorkbook(name string, sheets []workbook.Sheet) (string, error) {
	raw, err := workbook.Workbook(sheets)
	if err != nil {
		return "", err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export to Excel",
		DefaultFilename: name + ".xlsx",
		Filters:         []runtime.FileFilter{{DisplayName: "Excel workbook", Pattern: "*.xlsx"}},
	})
	if err != nil {
		return "", fmt.Errorf("save dialog: %w", err)
	}
	if path == "" {
		return "", nil
	}
	if !strings.HasSuffix(strings.ToLower(path), ".xlsx") {
		path += ".xlsx"
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}
