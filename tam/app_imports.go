package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"agile-suite/core/importfile"
	jirabackend "agile-suite/tam/internal/backend/jira"
	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/importer"
)

// maxImportBytes is the largest decoded upload ImportIssues and PreviewImport
// accept.
const maxImportBytes = 20 * 1024 * 1024

// decodeImport base64-decodes an uploaded file and parses it into rows.
func decodeImport(contentB64 string, isXlsx bool) ([][]string, error) {
	data, err := base64.StdEncoding.DecodeString(contentB64)
	if err != nil {
		return nil, fmt.Errorf("decode upload: %w", err)
	}
	if len(data) > maxImportBytes {
		return nil, errors.New("the file is larger than 20 MB")
	}
	return importfile.ParseRecords(data, isXlsx)
}

// requirementType is the profile's own name for its requirement level,
// falling back to the built-in default when the profile has not set one.
func (a *App) requirementType(profileID string) (string, error) {
	t, err := a.repo.ProfileSetting(a.ctx, profileID, settingRequirementType)
	if err != nil {
		return "", err
	}
	if t == "" {
		return jirabackend.DefaultRequirementType, nil
	}
	return t, nil
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
	reqType, err := a.requirementType(p.ID)
	if err != nil {
		return importer.Result{}, err
	}
	if strings.TrimSpace(fileName) == "" {
		fileName = "an uploaded file"
	}
	// The Sprint column is matched against these by name. A read failure is
	// not worth failing the whole import for: it costs the Sprint column its
	// list, and every row naming a sprint then says so with the rest.
	open, err := a.boards.OpenSprints(a.ctx, p.ID)
	if err != nil {
		log.Printf("tam: open sprints for the import of %s: %v", fileName, err)
		open = nil
	}
	res, err := importer.Run(a.ctx, a.repo, p.ID, p.ProjectKey, reqType, open, records, mapping, fileName, dryRun)
	if err != nil {
		return res, err
	}
	if !dryRun {
		log.Printf("tam: imported %s into %s: %d drafts, %d issues updated, %d rows skipped", fileName, p.ProjectKey, len(res.Created), len(res.Updated), len(res.Errors))
	}
	return res, nil
}

// SaveImportTemplate writes the starter workbook where the user chooses and
// returns the path, or "" when the dialog was cancelled. The default is the
// XLSX, which carries the notes sheet and the Type dropdown; picking a .csv
// name in the dialog writes the plain CSV of the same columns instead, for
// anyone whose toolchain would rather have one.
func (a *App) SaveImportTemplate(profileID string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("the window is not ready")
	}
	reqType := jirabackend.DefaultRequirementType
	var open []boardrepo.SprintChoice
	// A template is worth offering even without a usable profile, so a
	// profile that cannot be read falls back to the default type name and
	// an unlisted Sprint column rather than failing the save.
	if p, err := a.requireProfile(profileID); err == nil {
		if t, err := a.requirementType(p.ID); err == nil {
			reqType = t
		}
		if s, err := a.boards.OpenSprints(a.ctx, p.ID); err == nil {
			open = s
		}
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save import template",
		DefaultFilename: "tam-import-template.xlsx",
		Filters: []runtime.FileFilter{
			{DisplayName: "Excel workbook", Pattern: "*.xlsx"},
			{DisplayName: "CSV", Pattern: "*.csv"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("save dialog: %w", err)
	}
	if path == "" {
		return "", nil
	}
	data := []byte(nil)
	if strings.EqualFold(filepath.Ext(path), ".csv") {
		data = importer.TemplateCSV(reqType)
	} else {
		if data, err = importer.TemplateXLSX(reqType, open); err != nil {
			return "", fmt.Errorf("build template: %w", err)
		}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write template: %w", err)
	}
	return path, nil
}
