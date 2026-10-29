package main

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"agile-suite/xtm/internal/backend"
)

func (a *App) GetParameterizedRun(profileID, execKey, testKey string, forceRefresh bool) (backend.ParameterizedRun, error) {
	if err := a.requireStore(); err != nil {
		return backend.ParameterizedRun{}, err
	}
	if err := validateParameterizedRunIdentity(profileID, execKey, testKey); err != nil {
		return backend.ParameterizedRun{}, err
	}
	if !forceRefresh {
		if cached, found, err := a.repo.ParameterizedRun(profileID, execKey, testKey); err != nil {
			return backend.ParameterizedRun{}, err
		} else if found {
			return cached, nil
		}
	}
	b, err := a.backendFor(profileID)
	if err != nil {
		return backend.ParameterizedRun{}, err
	}
	if !b.Capabilities().SupportsParameterizedRunDetail {
		return backend.ParameterizedRun{}, backend.ErrUnsupported
	}
	run, err := b.GetParameterizedRun(a.ctx, execKey, testKey)
	if err != nil {
		// ErrUnsupported is the ordinary answer for a test with no dataset,
		// and every member row of every execution asks, so logging that would
		// bury the failures worth reading. Anything else is a real one (#362).
		if !errors.Is(err, backend.ErrUnsupported) {
			log.Printf("xtm: could not read the parameterized run of %s in %s: %v", testKey, execKey, err)
		}
		return backend.ParameterizedRun{}, err
	}
	if err := a.repo.ReplaceParameterizedRun(profileID, run); err != nil {
		// The run was read and then refused by the store, which is invisible
		// in the view: the member row simply shows no iterations (#362).
		log.Printf("xtm: read the parameterized run of %s in %s but could not store it: %v", testKey, execKey, err)
		return backend.ParameterizedRun{}, err
	}
	stored, found, err := a.repo.ParameterizedRun(profileID, execKey, testKey)
	if err != nil {
		return backend.ParameterizedRun{}, err
	}
	if !found {
		return backend.ParameterizedRun{}, fmt.Errorf("parameterized run was not cached")
	}
	return stored, nil
}

// SetIterationStatus records an iteration's result locally and journals it for
// Commit (#366).
//
// #336 wrote it straight to Xray on the pick, which made it the only edit in
// XTM that reached the server without a Commit: it could not be reviewed in
// Pending changes or discarded, and it broke the contract that the store is a
// cache plus a journal and Jira is the system of record. The write now goes
// where every other edit goes, and `syncer/commit_iteration.go` pushes it.
//
// Dropping the write also drops the re-read it forced. That re-read was what
// destroyed a locally set step result (#364), because refreshing the run
// replaced the whole iteration tree with what Xray reports, and Xray reports no
// per-step status.
//
// The run's own status is therefore no longer refreshed on every pick. Xray
// derives it from the iterations by a rule XTM does not own, so the Execution
// column keeps showing the value last read from Xray until the results are
// committed, rather than a value XTM invented.
func (a *App) SetIterationStatus(profileID, execKey, testKey, iterationID, status string) error {
	if err := validateParameterizedRunWrite(profileID, execKey, testKey, iterationID, status); err != nil {
		return err
	}
	if err := a.requireParameterizedRunWrites(profileID); err != nil {
		return err
	}
	return a.repo.SetIterationStatus(profileID, execKey, testKey, iterationID, status)
}

func (a *App) SetIterationStepStatus(profileID, execKey, testKey, iterationID, stepResultID, status string) error {
	if err := validateParameterizedRunWrite(profileID, execKey, testKey, iterationID, status); err != nil {
		return err
	}
	if strings.TrimSpace(stepResultID) == "" {
		return fmt.Errorf("step result id is required")
	}
	if err := a.requireParameterizedRunWrites(profileID); err != nil {
		return err
	}
	return a.repo.SetIterationStepStatus(profileID, execKey, testKey, iterationID, stepResultID, status)
}

func (a *App) requireParameterizedRunWrites(profileID string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	b, err := a.backendFor(profileID)
	if err != nil {
		return err
	}
	if !b.Capabilities().SupportsParameterizedRunWrites {
		return backend.ErrUnsupported
	}
	return nil
}

func validateParameterizedRunIdentity(profileID, execKey, testKey string) error {
	if strings.TrimSpace(profileID) == "" || strings.TrimSpace(execKey) == "" || strings.TrimSpace(testKey) == "" {
		return fmt.Errorf("profile, execution and test are required")
	}
	return nil
}

func validateParameterizedRunWrite(profileID, execKey, testKey, iterationID, status string) error {
	if err := validateParameterizedRunIdentity(profileID, execKey, testKey); err != nil {
		return err
	}
	if strings.TrimSpace(iterationID) == "" || strings.TrimSpace(status) == "" {
		return fmt.Errorf("iteration id and status are required")
	}
	return nil
}
