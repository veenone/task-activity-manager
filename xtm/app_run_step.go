// The Wails bindings for step results on a plain execution (#371 Phase 6).
//
// It mirrors app_parameterized_run.go: the cache answers first, the backend
// fills it, and a pick is journalled locally rather than written through.
// Writing on the pick is what #336 got wrong and #366 fixed, and the same rule
// applies here: the store is a cache plus a journal, Jira is the system of
// record, and syncer/commit_run_step.go is the only thing that writes.

package main

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"agile-suite/xtm/internal/backend"
)

// GetRunStepResults reads one test's step results in one execution, from the
// cache unless a refresh is asked for.
func (a *App) GetRunStepResults(profileID, execKey, testKey string, forceRefresh bool) ([]backend.RunStepResult, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	if err := validateParameterizedRunIdentity(profileID, execKey, testKey); err != nil {
		return nil, err
	}
	if !forceRefresh {
		if cached, found, err := a.repo.RunStepResults(profileID, execKey, testKey); err != nil {
			return nil, err
		} else if found {
			return cached, nil
		}
	}
	manager, err := a.runStepManager(profileID)
	if err != nil {
		return nil, err
	}
	steps, err := manager.GetRunStepResults(a.ctx, execKey, testKey)
	if err != nil {
		// A test with no step results is the ordinary answer for a test with
		// no steps, and every member row of every execution asks, so logging
		// that would bury the failures worth reading (#362).
		if !errors.Is(err, backend.ErrUnsupported) {
			log.Printf("xtm: could not read the step results of %s in %s: %v", testKey, execKey, err)
		}
		return nil, err
	}
	if len(steps) == 0 {
		// Nothing to cache, and ReplaceRunStepResults refuses an empty set
		// rather than letting a backend blank a run. The view reads this as a
		// test with no recorded steps.
		return []backend.RunStepResult{}, nil
	}
	if err := a.repo.ReplaceRunStepResults(profileID, execKey, testKey, steps); err != nil {
		// Read and then refused by the store, which is invisible in the view:
		// the member row simply shows no steps (#362).
		log.Printf("xtm: read the step results of %s in %s but could not store them: %v", testKey, execKey, err)
		return nil, err
	}
	stored, found, err := a.repo.RunStepResults(profileID, execKey, testKey)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("the step results were not cached")
	}
	return stored, nil
}

// SetRunStepStatus records a step's result locally and journals it for Commit.
func (a *App) SetRunStepStatus(profileID, execKey, testKey, stepResultID, status string) error {
	if err := validateParameterizedRunIdentity(profileID, execKey, testKey); err != nil {
		return err
	}
	if strings.TrimSpace(stepResultID) == "" || strings.TrimSpace(status) == "" {
		return fmt.Errorf("step result id and status are required")
	}
	if _, err := a.runStepManager(profileID); err != nil {
		return err
	}
	return a.repo.SetRunStepStatus(profileID, execKey, testKey, stepResultID, status)
}

// runStepManager resolves the profile's backend and checks it can address step
// results at all. The capability is the gate and the type assertion is the
// backstop, as for every optional capability.
func (a *App) runStepManager(profileID string) (backend.RunStepResultManager, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	b, err := a.backendFor(profileID)
	if err != nil {
		return nil, err
	}
	if !b.Capabilities().SupportsRunStepResults {
		return nil, backend.ErrUnsupported
	}
	manager, ok := b.(backend.RunStepResultManager)
	if !ok {
		return nil, backend.ErrUnsupported
	}
	return manager, nil
}
