package main

import "agile-suite/xtm/internal/testrepo"

// RenameLabel queues renaming a label on every synced test carrying it, or
// merging it into an existing label. The Labels view calls it.
func (a *App) RenameLabel(profileID, oldLabel, newLabel string) (result testrepo.BulkEditResult, err error) {
	defer recoverToError("RenameLabel", &err)
	if err := a.requireStore(); err != nil {
		return testrepo.BulkEditResult{Succeeded: []string{}, Failed: []testrepo.BulkFailure{}}, err
	}
	return a.repo.RenameLabel(profileID, oldLabel, newLabel)
}

// DeleteLabel queues removing a label from every synced test carrying it.
func (a *App) DeleteLabel(profileID, label string) (result testrepo.BulkEditResult, err error) {
	defer recoverToError("DeleteLabel", &err)
	if err := a.requireStore(); err != nil {
		return testrepo.BulkEditResult{Succeeded: []string{}, Failed: []testrepo.BulkFailure{}}, err
	}
	return a.repo.DeleteLabel(profileID, label)
}
