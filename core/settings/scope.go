package settings

import (
	"errors"
	"strings"
)

// Scope reads and writes the settings one app owns. Two apps share the same
// app_setting table, so an app-specific key is stored as "<app>.<key>" and
// cannot overwrite the other app's preference of the same name. Settings both
// apps read (the theme, the default profile, the requirement link type) stay
// on Manager without a prefix.
type Scope struct {
	m   *Manager
	app string
}

// Scope returns the settings namespace for app, a short fixed identifier such
// as "tam". It must be non-empty and must not contain a dot.
func (m *Manager) Scope(app string) (*Scope, error) {
	if app == "" || strings.Contains(app, ".") {
		return nil, errors.New("settings scope: app id must be non-empty and contain no dot")
	}
	return &Scope{m: m, app: app}, nil
}

// Get returns the value stored for key in this scope, "" when unset.
func (s *Scope) Get(key string) (string, error) {
	return s.m.value(s.app + "." + key)
}

// Set stores value for key in this scope.
func (s *Scope) Set(key, value string) error {
	return s.m.setValue(s.app+"."+key, value)
}
