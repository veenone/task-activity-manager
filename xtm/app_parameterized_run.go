package main

import (
	"fmt"
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
		return backend.ParameterizedRun{}, err
	}
	if err := a.repo.ReplaceParameterizedRun(profileID, run); err != nil {
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
