package main

import "agile-suite/tam/internal/reports"

// BoardAllocation is the board's open work per assignee (#288), read from
// the cache alone: no Jira call, so the Reports view can show it beside a
// report without holding the profile's lock. Its rows are reports.Person,
// the People section's shape, with logged and in-progress time left at zero
// because the cache carries no changelog.
func (a *App) BoardAllocation(profileID string, boardID int) ([]reports.Person, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	issues, err := a.boards.BoardIssues(a.ctx, a.repo, profileID, boardID)
	if err != nil {
		return nil, err
	}
	return reports.Allocation(issues), nil
}
