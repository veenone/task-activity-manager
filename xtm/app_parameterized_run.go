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

// SetIterationStatus writes an iteration's result through the backend and then
// brings the stored run back in line with it (#336).
//
// The backend write comes first: a store updated before a refused write would
// show a status Xray never accepted. On demo the backend write is a no-op and
// the store is the only record, which is what it has always been.
//
// The run is re-read rather than patched locally, because Xray derives the
// run's own status from its iterations by a rule XTM does not own: writing one
// iteration moves the run to EXECUTING on its own. A re-read costs a round trip
// per write and is what keeps the view from showing a derived value XTM made up.
func (a *App) SetIterationStatus(profileID, execKey, testKey, iterationID, status string) error {
	if err := validateParameterizedRunWrite(profileID, execKey, testKey, iterationID, status); err != nil {
		return err
	}
	if err := a.requireParameterizedRunWrites(profileID); err != nil {
		return err
	}
	b, err := a.backendFor(profileID)
	if err != nil {
		return err
	}
	if err := b.SetIterationStatus(a.ctx, execKey, testKey, iterationID, status); err != nil {
		return err
	}
	if err := a.repo.SetIterationStatus(profileID, execKey, testKey, iterationID, status); err != nil {
		return err
	}
	if b.IsDemo() {
		return nil
	}
	// A failed refresh is not a failed write: the status reached Xray, and the
	// store already carries it. The run status is what stays stale, and the next
	// read fixes it, so this is logged rather than returned.
	run, err := b.GetParameterizedRun(a.ctx, execKey, testKey)
	if err != nil {
		log.Printf("xtm: iteration written but the run could not be re-read: %v", err)
		return nil
	}
	if err := a.repo.ReplaceParameterizedRun(profileID, run); err != nil {
		log.Printf("xtm: iteration written but the run could not be stored: %v", err)
	}
	return nil
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
