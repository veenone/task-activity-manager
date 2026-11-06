package main

import "agile-suite/pie/internal/cases"

// The bindings for the analyst's baseline mapping. Each change returns the
// mapping as it stands after it, so the dialog redraws from what was stored.

// Baselines returns the profile's baselines with their descendants.
func (a *App) Baselines(profileID string) ([]cases.Baseline, error) {
	return a.afterBaselines(profileID, func() error { return nil })
}

// AddBaseline marks a Jira project as a product baseline.
func (a *App) AddBaseline(profileID, key string) ([]cases.Baseline, error) {
	return a.afterBaselines(profileID, func() error { return a.cases.AddBaseline(profileID, key) })
}

// AddDescendants puts Jira projects under a baseline.
func (a *App) AddDescendants(profileID, baseline string, keys []string) ([]cases.Baseline, error) {
	return a.afterBaselines(profileID, func() error { return a.cases.AddDescendants(profileID, baseline, keys) })
}

// RemoveBaseline drops a baseline and its descendants from the mapping.
func (a *App) RemoveBaseline(profileID, key string) ([]cases.Baseline, error) {
	return a.afterBaselines(profileID, func() error { return a.cases.RemoveBaseline(profileID, key) })
}

// RemoveDescendant takes a project out from under its baseline.
func (a *App) RemoveDescendant(profileID, key string) ([]cases.Baseline, error) {
	return a.afterBaselines(profileID, func() error { return a.cases.RemoveDescendant(profileID, key) })
}

func (a *App) afterBaselines(profileID string, change func() error) ([]cases.Baseline, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	if err := change(); err != nil {
		return nil, err
	}
	return a.cases.Baselines(profileID)
}
