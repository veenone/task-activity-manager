// Package pieprofile holds the rules for PIE's own profiles. They use core's
// profile tables, but in a file of their own: a PIE profile has no project key,
// and TAM and XTM read every row of the shared database.
package pieprofile

import (
	"errors"
	"net/url"
	"path/filepath"
	"strings"

	"agile-suite/core/shareddb"
	"agile-suite/pie/internal/demo"
)

// DefaultPath is PIE's database, beside the suite's shared one.
func DefaultPath() (string, error) {
	shared, err := shareddb.DefaultPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(shared), "pie.db"), nil
}

// ValidateNew checks the fields of a profile about to be created.
func ValidateNew(name, jiraURL, token string) error {
	if err := ValidateFields(name, jiraURL); err != nil || demo.Is(jiraURL) {
		return err // invented data: no server, so no token
	}
	if strings.TrimSpace(token) == "" {
		return errors.New("a profile needs a personal access token")
	}
	return nil
}

// ValidateFields checks a profile's name and Jira URL, for a profile file
// that carries no token.
func ValidateFields(name, jiraURL string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("a profile needs a name")
	}
	if demo.Is(jiraURL) {
		return nil
	}
	u, err := url.Parse(strings.TrimSpace(jiraURL))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("a profile needs a Jira URL starting with http:// or https://")
	}
	return nil
}
