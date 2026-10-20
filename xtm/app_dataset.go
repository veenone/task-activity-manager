package main

import (
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/jira"
	"agile-suite/xtm/internal/testrepo"
)

// GetTestDataset returns a test's dataset: the parameters its ${placeholders}
// refer to, and one row per iteration (#267). The detail panel calls it when
// it opens a test.
//
// Reads are lazy and cache-first, like GetTestSteps, but the cache check is
// deliberately NOT the same. GetTestSteps treats "rows came back" as a hit,
// which works because nearly every test has steps. Nearly every test has no
// dataset, so this stores the absence as a fact and hits on the row existing.
// Checking for a non-empty dataset instead would mean the nineteen tests in
// twenty that are not parameterized each pay for a Jira round trip on every
// single open, to be told again that there is nothing.
//
// forceRefresh is the panel's Refresh: skip the cache and overwrite it unless
// that would discard an uncommitted local dataset edit.
func (a *App) GetTestDataset(profileID, testKey string, forceRefresh bool) (testrepo.Dataset, error) {
	if err := a.requireStore(); err != nil {
		return testrepo.Dataset{}, err
	}
	// An uncommitted local Test has a placeholder key Jira would reject, so it
	// is answered from the store alone, as its steps are.
	if isLocalTestKey(testKey) {
		ds, _, err := a.repo.GetTestDataset(profileID, testKey, "")
		return ds, err
	}
	if forceRefresh {
		pending, err := a.repo.HasPendingDatasetChange(profileID, testKey)
		if err != nil {
			return testrepo.Dataset{}, err
		}
		if pending {
			ds, _, err := a.repo.GetTestDataset(profileID, testKey, "")
			return ds, err
		}
	}
	if !forceRefresh {
		cached, found, err := a.repo.GetTestDataset(profileID, testKey, "")
		if err != nil {
			return testrepo.Dataset{}, err
		}
		if found {
			return cached, nil
		}
	}

	b, err := a.backendFor(profileID)
	if err != nil {
		return testrepo.Dataset{}, err
	}
	remote, err := b.GetDataset(a.ctx, testKey)
	if err != nil {
		return testrepo.Dataset{}, err
	}

	ds := toRepoDataset(remote)
	// Cache on success only. Writing a row after a failed fetch would record
	// "this test has no dataset" on the strength of a server error, and the
	// panel would keep showing nothing until someone pressed Refresh.
	if err := a.repo.SaveTestDataset(profileID, testKey, "", ds); err != nil {
		return testrepo.Dataset{}, err
	}
	return ds, nil
}

// GetContainerDataset returns the dataset a Test Execution defines for a
// test, and an empty one when it defines none (#277 W3).
//
// Xray resolves a dataset nearest-first, so an execution's own table is what
// runs there. This answers what that level defines rather than what will run,
// because the two together are what the panel needs: an empty answer means
// the test's own dataset is in force, and the panel says which.
//
// Cached per (test, container) in the same table as the test-level read, so
// reopening a member costs nothing. forceRefresh is the panel's Refresh.
func (a *App) GetContainerDataset(profileID, containerKey, testKey string, forceRefresh bool) (testrepo.Dataset, error) {
	if err := a.requireStore(); err != nil {
		return testrepo.Dataset{}, err
	}
	if containerKey == "" || isLocalTestKey(testKey) {
		return testrepo.Dataset{}, nil
	}
	if !forceRefresh {
		cached, found, err := a.repo.GetTestDataset(profileID, testKey, containerKey)
		if err != nil {
			return testrepo.Dataset{}, err
		}
		if found {
			return cached, nil
		}
	}

	b, err := a.backendFor(profileID)
	if err != nil {
		return testrepo.Dataset{}, err
	}
	remote, err := b.GetRunDataset(a.ctx, containerKey, testKey)
	if err != nil {
		return testrepo.Dataset{}, err
	}
	ds := toRepoDataset(remote)
	if err := a.repo.SaveTestDataset(profileID, testKey, containerKey, ds); err != nil {
		return testrepo.Dataset{}, err
	}
	return ds, nil
}

// GetRunIterations returns how each row of a test's dataset went in one
// execution (#267 Phase 4).
//
// Deliberately not cached. A dataset changes rarely and is cached; results
// change every time someone runs the test, and the panel that asks for these
// is open in front of the person who just changed them. Nothing else reads
// them, so there is no table to keep in step either.
func (a *App) GetRunIterations(profileID, containerKey, testKey string) ([]backend.RunIteration, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	if containerKey == "" || isLocalTestKey(testKey) {
		return nil, nil
	}
	b, err := a.backendFor(profileID)
	if err != nil {
		return nil, err
	}
	return b.GetRunIterations(a.ctx, containerKey, testKey)
}

// SaveTestDataset replaces a test's dataset locally and queues it for commit
// (#277 W1). The editor calls it; Jira hears nothing until Commit, like every
// other edit in XTM.
//
// A table Xray would refuse is refused here, with the reason, and neither
// cached nor queued. The check lives in the repository so no path can skip it.
func (a *App) SaveTestDataset(profileID, testKey string, ds testrepo.Dataset) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.repo.EditTestDataset(profileID, testKey, ds)
}

// RenameDatasetPlaceholder rewrites ${oldName} to ${newName} in the test's
// steps and definition, and reports how many fields changed (#277 W2).
//
// The editor calls it after a parameter rename, once the user has agreed:
// renaming a parameter leaves every reference to the old name resolving to
// nothing, and a test then runs with the literal ${OLD} in place of a value.
// It is a separate call rather than part of the save because the rewrite is a
// choice, and because each rewritten field is its own pending change.
func (a *App) RenameDatasetPlaceholder(profileID, testKey, oldName, newName string) (int, error) {
	if err := a.requireStore(); err != nil {
		return 0, err
	}
	return a.repo.RenameDatasetPlaceholder(profileID, testKey, oldName, newName)
}

// ImportDatasetCSV reads a dataset out of a CSV the user picked and hands it
// back for the editor to show (#277 W2). Nothing is stored: the user sees the
// table first and saves it like any other edit.
//
// current is the table being replaced. A CSV carries names, the combinatorial
// asterisk and values, and has nowhere to put a parameter's id or the list
// behind it, so those are matched back by name. Without that an import would
// rebuild every parameter on the next write.
func (a *App) ImportDatasetCSV(content string, current testrepo.Dataset) (testrepo.Dataset, error) {
	parsed := fromJiraDataset(jira.ParseDatasetCSV([]byte(content)))
	if len(parsed.Parameters) == 0 {
		return testrepo.Dataset{}, fmt.Errorf("that file has no parameter row; the first row names the parameters")
	}
	merged := testrepo.MergeDatasetParameters(parsed, current)
	if err := testrepo.ValidateDataset(merged); err != nil {
		return testrepo.Dataset{}, err
	}
	return merged, nil
}

// ExportDatasetCSV writes the table the editor is showing to a file the user
// picks, in the layout Xray's own export uses, and returns the path (empty if
// they cancelled).
//
// It exports what is on screen rather than what is cached, so an export taken
// mid-edit is the table the user is looking at.
func (a *App) ExportDatasetCSV(testKey string, ds testrepo.Dataset) (string, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export dataset",
		DefaultFilename: exportFilename(testKey + "-dataset.csv"),
		Filters:         []runtime.FileFilter{{DisplayName: "CSV", Pattern: "*.csv"}},
	})
	if err != nil {
		return "", fmt.Errorf("save dialog: %w", err)
	}
	if path == "" {
		return "", nil
	}
	if err := os.WriteFile(path, jira.DatasetCSV(toJiraDataset(ds)), 0o644); err != nil {
		return "", fmt.Errorf("write dataset: %w", err)
	}
	return path, nil
}

// fromJiraDataset / toJiraDataset map the client's table onto the stored one.
// The CSV layout is Xray's, so its reader and writer live in the jira package
// with the rest of that format.
func fromJiraDataset(d jira.Dataset) testrepo.Dataset {
	params := make([]testrepo.DatasetParameter, len(d.Parameters))
	for i, p := range d.Parameters {
		params[i] = testrepo.DatasetParameter{Name: p.Name, Combinatorial: p.Combinatorial}
	}
	return testrepo.Dataset{Parameters: params, Rows: d.Rows}
}

func toJiraDataset(ds testrepo.Dataset) jira.Dataset {
	params := make([]jira.DatasetParameter, len(ds.Parameters))
	for i, p := range ds.Parameters {
		params[i] = jira.DatasetParameter{Name: p.Name, Combinatorial: p.Combinatorial}
	}
	return jira.Dataset{Parameters: params, Rows: ds.Rows}
}

// toRepoDataset maps the neutral backend DTO onto the stored shape.
func toRepoDataset(d backend.Dataset) testrepo.Dataset {
	params := make([]testrepo.DatasetParameter, len(d.Parameters))
	for i, p := range d.Parameters {
		params[i] = testrepo.DatasetParameter{
			Name: p.Name, Combinatorial: p.Combinatorial, ID: p.ID,
			Type: p.Type, Options: p.Options, ListScope: p.ListScope, ListID: p.ListID,
		}
	}
	return testrepo.Dataset{Parameters: params, Rows: d.Rows}
}
