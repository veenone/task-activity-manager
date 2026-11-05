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
	if strings.TrimSpace(name) == "" {
		return errors.New("a profile needs a name")
	}
	u, err := url.Parse(strings.TrimSpace(jiraURL))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("a profile needs a Jira URL starting with http:// or https://")
	}
	if strings.TrimSpace(token) == "" {
		return errors.New("a profile needs a personal access token")
	}
	return nil
}
