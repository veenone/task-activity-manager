// Package suiteprofiles holds the rules for how Task Activity Manager sees the
// profiles it shares with Xray Test Manager.
package suiteprofiles

import (
	"errors"
	"strings"

	"agile-suite/core/profile"
)

// Backend is the value TAM passes to core when it creates a profile. Core
// normalizes every non-kiwi value to "xray" when it writes the row, so the
// stored backend is "xray", not "jira". The constant's job is only to mean
// "a Jira profile, not Kiwi", so the same row is usable from XTM too.
const Backend = "jira"

// Visible drops the profiles TAM cannot use. Kiwi TCMS is not Jira.
func Visible(ps []profile.Profile) []profile.Profile {
	out := make([]profile.Profile, 0, len(ps))
	for _, p := range ps {
		if strings.EqualFold(p.Backend, "kiwi") {
			continue
		}
		out = append(out, p)
	}
	return out
}

// IsDemoURL reports whether a Jira URL selects the offline demo dataset:
// "demo" on its own, or a "demo:" or "demo-" variant. It mirrors XTM's rule so
// a demo profile made in either app is a demo profile in both.
func IsDemoURL(url string) bool {
	u := strings.ToLower(strings.TrimSpace(url))
	return u == "demo" || strings.HasPrefix(u, "demo:") || strings.HasPrefix(u, "demo-")
}

// ValidateNew checks the fields of a profile about to be created. A demo
// profile needs no token; a live one does.
func ValidateNew(name, jiraURL, projectKey, token string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("a profile needs a name")
	}
	if strings.TrimSpace(jiraURL) == "" {
		return errors.New("a profile needs a Jira URL (or \"demo\")")
	}
	if strings.TrimSpace(projectKey) == "" {
		return errors.New("a profile needs a project key")
	}
	if !IsDemoURL(jiraURL) && strings.TrimSpace(token) == "" {
		return errors.New("a live Jira profile needs a personal access token")
	}
	return nil
}
