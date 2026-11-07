package main

import (
	"time"

	"agile-suite/pie/internal/cycles"
)

// The bindings for reporting cycles. Each validates through internal/cycles;
// nothing here reaches Jira or Confluence.

// Cycles returns the profile's cycles, newest first.
func (a *App) Cycles(profileID string) ([]cycles.Cycle, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.cycles.List(profileID)
}

// TakeCycle stores a new cycle holding the cases as they are now.
func (a *App) TakeCycle(profileID, name string) (cycles.Cycle, error) {
	if err := a.requireStore(); err != nil {
		return cycles.Cycle{}, err
	}
	return a.cycles.Take(profileID, name, time.Now())
}

// GetCycle returns one cycle with its cases and what changed.
func (a *App) GetCycle(profileID string, id int64) (cycles.Detail, error) {
	if err := a.requireStore(); err != nil {
		return cycles.Detail{}, err
	}
	return a.cycles.Get(profileID, id)
}

// RetakeCycle replaces an unpublished cycle's cases with the cases now.
func (a *App) RetakeCycle(profileID string, id int64) (cycles.Detail, error) {
	if err := a.requireStore(); err != nil {
		return cycles.Detail{}, err
	}
	if err := a.cycles.Retake(profileID, id, time.Now()); err != nil {
		return cycles.Detail{}, err
	}
	return a.cycles.Get(profileID, id)
}

// DeleteCycle removes an unpublished cycle.
func (a *App) DeleteCycle(profileID string, id int64) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.cycles.Delete(profileID, id)
}
