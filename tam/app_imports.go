package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"agile-suite/core/importfile"
	jirabackend "agile-suite/tam/internal/backend/jira"
	"agile-suite/tam/internal/importer"
)

// decodeImport base64-decodes an uploaded file and parses it into rows.
func decodeImport(contentB64 string, isXlsx bool) ([][]string, error) {
	data, err := base64.StdEncoding.DecodeString(contentB64)
	if err != nil {
		return nil, fmt.Errorf("decode upload: %w", err)
	}
	return importfile.ParseRecords(data, isXlsx)
}

// PreviewImport parses an uploaded file's header row and counts its data
// rows so the dialog can offer column mapping.
func (a *App) PreviewImport(contentB64 string, isXlsx bool) (importfile.Preview, error) {
	if err := a.requireStore(); err != nil {
		return importfile.Preview{}, err
	}
	records, err := decodeImport(contentB64, isXlsx)
	if err != nil {
		return importfile.Preview{}, err
	}
	return importfile.ParsePreview(records)
}

// AutoMapImport guesses the column for each draft field from the headers.
func (a *App) AutoMapImport(headers []string) importer.Mapping {
	return importer.AutoMap(headers)
}

// ImportIssues validates the file against the mapping and, unless dryRun,
// creates a draft per valid row. It refuses while a sync or commit runs.
func (a *App) ImportIssues(profileID, contentB64 string, isXlsx bool, fileName string, mapping importer.Mapping, dryRun bool) (importer.Result, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return importer.Result{}, err
	}
	if err := a.acquire(p.ID, "import"); err != nil {
		return importer.Result{}, err
	}
	defer a.release(p.ID)
	records, err := decodeImport(contentB64, isXlsx)
	if err != nil {
		return importer.Result{}, err
	}
	reqType, err := a.repo.ProfileSetting(a.ctx, p.ID, settingRequirementType)
	if err != nil {
		return importer.Result{}, err
	}
	if reqType == "" {
		reqType = jirabackend.DefaultRequirementType
	}
	if strings.TrimSpace(fileName) == "" {
		fileName = "an uploaded file"
	}
	res, err := importer.Run(a.ctx, a.repo, p.ID, p.ProjectKey, reqType, records, mapping, fileName, dryRun)
	if err != nil {
		return res, err
	}
	if !dryRun {
		log.Printf("tam: imported %d drafts from %s into %s (%d rows skipped)", len(res.Created), fileName, p.ProjectKey, len(res.Errors))
	}
	return res, nil
}

// SaveImportTemplate writes the starter CSV where the user chooses and
// returns the path, or "" when the dialog was cancelled.
func (a *App) SaveImportTemplate() (string, error) {
	if a.ctx == nil {
		return "", errors.New("the window is not ready")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save import template",
		DefaultFilename: "tam-import-template.csv",
		Filters:         []runtime.FileFilter{{DisplayName: "CSV", Pattern: "*.csv"}},
	})
	if err != nil {
		return "", fmt.Errorf("save dialog: %w", err)
	}
	if path == "" {
		return "", nil
	}
	if err := os.WriteFile(path, importer.TemplateCSV(), 0o644); err != nil {
		return "", fmt.Errorf("write template: %w", err)
	}
	return path, nil
}
