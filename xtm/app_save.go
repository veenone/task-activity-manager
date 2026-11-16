package main

import (
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// askSavePath opens the save dialog and returns the path the user chose. The
// path is empty, with no error, when they cancel.
func (a *App) askSavePath(title, filename string, filters ...runtime.FileFilter) (string, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           title,
		DefaultFilename: filename,
		Filters:         filters,
	})
	if err != nil {
		return "", fmt.Errorf("save dialog: %w", err)
	}
	return path, nil
}
