// Package settings manages global (non-profile) application preferences
// (FR-12.2), stored as key-value rows in the local database.
package settings

import (
	"database/sql"
	"errors"
	"fmt"

	"xray-test-manager/internal/store"
)

const keyDefaultProfileID = "default_profile_id"

// Settings holds the global application preferences.
type Settings struct {
	DefaultProfileID string `json:"defaultProfileId"`
}

// Manager reads and writes global settings.
type Manager struct {
	db *sql.DB
}

// NewManager returns a settings manager backed by the given store.
func NewManager(s *store.Store) *Manager {
	return &Manager{db: s.DB()}
}

// Get returns the current settings, with zero values for anything unset.
func (m *Manager) Get() (Settings, error) {
	var s Settings
	v, err := m.value(keyDefaultProfileID)
	if err != nil {
		return Settings{}, err
	}
	s.DefaultProfileID = v
	return s, nil
}

// Set persists the given settings.
func (m *Manager) Set(s Settings) error {
	return m.setValue(keyDefaultProfileID, s.DefaultProfileID)
}

func (m *Manager) value(key string) (string, error) {
	var v string
	err := m.db.QueryRow(`SELECT value FROM app_setting WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read setting %q: %w", key, err)
	}
	return v, nil
}

func (m *Manager) setValue(key, value string) error {
	if _, err := m.db.Exec(
		`INSERT INTO app_setting (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	); err != nil {
		return fmt.Errorf("write setting %q: %w", key, err)
	}
	return nil
}
