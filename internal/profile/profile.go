// Package profile manages connection profiles and their credentials.
//
// A profile binds the application to one Jira project and connection (FR-5).
// Each profile owns an isolated local dataset; the user switches the active
// profile to switch projects.
package profile

import (
	"errors"
	"time"

	"xray-test-manager/internal/store"
)

// ErrNotImplemented marks Phase 0 stubs that are wired but not yet built out.
var ErrNotImplemented = errors.New("not implemented")

// Profile is one Jira project connection. Credentials are stored separately in
// the OS credential manager (see CredentialStore) — never in this struct or the
// database.
type Profile struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	JiraURL    string    `json:"jiraUrl"`
	ProjectKey string    `json:"projectKey"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Manager is the profile CRUD service backed by the local store (FR-5.1).
type Manager struct {
	store *store.Store
}

// NewManager returns a profile manager backed by the given store.
func NewManager(s *store.Store) *Manager {
	return &Manager{store: s}
}

// List returns all profiles, ordered by name.
//
// TODO(xtm): implement against the profiles table (FR-5.1).
func (m *Manager) List() ([]Profile, error) {
	return nil, ErrNotImplemented
}

// Create persists a new profile, generating its ID and CreatedAt.
//
// TODO(xtm): implement (FR-5.1).
func (m *Manager) Create(p Profile) (Profile, error) {
	return Profile{}, ErrNotImplemented
}

// Delete removes a profile and its isolated local dataset.
//
// TODO(xtm): implement, including per-profile data cleanup (FR-5.3).
func (m *Manager) Delete(id string) error {
	return ErrNotImplemented
}
