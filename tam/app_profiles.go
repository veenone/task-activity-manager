package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	corejira "agile-suite/core/jira"
	"agile-suite/core/profile"
	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
	jirabackend "agile-suite/tam/internal/backend/jira"
	"agile-suite/tam/internal/suiteprofiles"
)

// app_profiles.go holds the profile manager's bound methods: the CRUD the
// Manage Profiles dialog needs, the connection test behind its Test button,
// and the credential-free export/import pair. They mirror XTM's own profile
// methods so the two dialogs are one feature, with one difference: the fields
// TAM's form does not show (bug issue type, bug project, backend) are read off
// the saved row and written back unchanged, so editing a profile here never
// resets what XTM configured on it.

// ListProfiles returns the shared profiles TAM can use.
func (a *App) ListProfiles() ([]profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	ps, err := a.profiles.List()
	if err != nil {
		return nil, err
	}
	return suiteprofiles.Visible(ps), nil
}

// CreateProfile adds a profile to the shared store and saves its token in the
// OS credential manager. A demo profile ("demo" as the URL) needs no token.
// scopeJQL narrows what syncs; caCert and allowUntrustedTLS configure the TLS
// trust used to reach this Jira instance.
func (a *App) CreateProfile(name, jiraURL, projectKey, scopeJQL, token, caCert string, allowUntrustedTLS bool) (profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return profile.Profile{}, err
	}
	if err := suiteprofiles.ValidateNew(name, jiraURL, projectKey, token); err != nil {
		return profile.Profile{}, err
	}
	p, err := a.profiles.Create(
		strings.TrimSpace(name), strings.TrimSpace(jiraURL), strings.TrimSpace(projectKey),
		strings.TrimSpace(scopeJQL), "", "", "", strings.TrimSpace(caCert), allowUntrustedTLS,
		suiteprofiles.Backend,
	)
	if err != nil {
		return profile.Profile{}, err
	}
	if strings.TrimSpace(token) != "" {
		if err := a.creds.Save(p.ID, strings.TrimSpace(token)); err != nil {
			if delErr := a.profiles.Delete(p.ID); delErr != nil {
				log.Printf("tam: rollback profile %s after credential save failure: %v", p.ID, delErr)
			}
			return profile.Profile{}, fmt.Errorf("save credentials: %w", err)
		}
	}
	return p, nil
}

// CreateProfileReusingToken creates a profile that borrows the credential
// already stored for another one, for the common case of several projects on
// one Jira instance. The token is copied inside the OS credential manager and
// never reaches the frontend.
func (a *App) CreateProfileReusingToken(name, jiraURL, projectKey, scopeJQL, sourceProfileID string) (profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return profile.Profile{}, err
	}
	if err := suiteprofiles.ValidateFields(name, jiraURL, projectKey); err != nil {
		return profile.Profile{}, err
	}
	token, err := a.creds.Load(sourceProfileID)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("read the token from the selected profile: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return profile.Profile{}, errors.New("the selected profile has no stored token to reuse")
	}
	return a.CreateProfile(name, jiraURL, projectKey, scopeJQL, token, "", false)
}

// UpdateProfile edits a profile's name, URL, project key, scope, and TLS
// settings, and replaces its token when one is given (a blank token keeps the
// stored one). The bug-filing fields and the backend are carried over from the
// saved row: TAM's form does not show them, so an edit here leaves XTM's
// settings alone. Changing the project key or the URL purges the rows TAM
// cached for the old project, so the next sync starts clean.
func (a *App) UpdateProfile(id, name, jiraURL, projectKey, scopeJQL, token, caCert string, allowUntrustedTLS bool) (profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return profile.Profile{}, err
	}
	old, err := a.profiles.Get(id)
	if err != nil {
		return profile.Profile{}, err
	}
	if err := suiteprofiles.ValidateFields(name, jiraURL, projectKey); err != nil {
		return profile.Profile{}, err
	}
	name, jiraURL = strings.TrimSpace(name), strings.TrimSpace(jiraURL)
	projectKey, scopeJQL = strings.TrimSpace(projectKey), strings.TrimSpace(scopeJQL)
	if err := a.profiles.Update(
		id, name, jiraURL, projectKey, scopeJQL,
		old.BugIssueType, old.BugProjectMode, old.BugProjectKey,
		strings.TrimSpace(caCert), allowUntrustedTLS, old.Backend,
	); err != nil {
		return profile.Profile{}, err
	}
	if strings.TrimSpace(token) != "" {
		if err := a.creds.Save(id, strings.TrimSpace(token)); err != nil {
			return profile.Profile{}, fmt.Errorf("save credentials: %w", err)
		}
	}
	// The cached backend holds the old URL, TLS settings, and token.
	a.forgetBackend(id)
	if old.ProjectKey != projectKey || old.JiraURL != jiraURL {
		if err := a.repo.PurgeProfile(a.ctx, id); err != nil {
			return profile.Profile{}, fmt.Errorf("clear the cached rows of the old project: %w", err)
		}
	}
	return a.profiles.Get(id)
}

// DeleteProfile removes a profile from the shared store, so it disappears
// from XTM as well, drops its credential, and purges the rows TAM cached
// for it locally.
func (a *App) DeleteProfile(id string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if err := a.profiles.Delete(id); err != nil {
		return err
	}
	a.forgetBackend(id)
	if err := a.repo.PurgeProfile(a.ctx, id); err != nil {
		// The shared profile row is already gone, so a failed purge leaves
		// stale local rows rather than a half-deleted profile. Log it.
		log.Printf("tam: purge local rows for %s: %v", id, err)
	}
	// The board tables are boardrepo's, not issuerepo's, so they are purged
	// beside it rather than by it.
	if err := a.boards.PurgeProfile(a.ctx, id); err != nil {
		log.Printf("tam: purge local board rows for %s: %v", id, err)
	}
	if err := a.creds.Delete(id); err != nil {
		log.Printf("tam: delete credentials for %s: %v", id, err)
	}
	if s, err := a.settings.Get(); err == nil && s.DefaultProfileID == id {
		if err := a.settings.SetDefaultProfileID(""); err != nil {
			log.Printf("tam: clear default profile after delete: %v", err)
		}
	}
	return nil
}

// TestConnection verifies a URL and token that are not saved yet, returning
// the display name of the authenticated user. It does not touch the store, so
// it works while a profile is still being typed.
func (a *App) TestConnection(jiraURL, token, caCert string, allowUntrustedTLS bool) (string, error) {
	user, err := testBackend(jiraURL, token, caCert, allowUntrustedTLS).TestConnection(a.ctx)
	if err != nil {
		return "", err
	}
	return user.DisplayName, nil
}

// TestProfileConnection verifies a saved profile with its stored token, so the
// Test button works while editing, where the token field is blank on purpose.
// The URL and TLS settings come from the form, not the saved row, so an
// unsaved edit is what gets tested.
func (a *App) TestProfileConnection(profileID, jiraURL, caCert string, allowUntrustedTLS bool) (string, error) {
	if err := a.requireStore(); err != nil {
		return "", err
	}
	token := ""
	if !suiteprofiles.IsDemoURL(jiraURL) {
		var err error
		if token, err = a.creds.Load(profileID); err != nil {
			return "", fmt.Errorf("read the stored token: %w", err)
		}
	}
	user, err := testBackend(jiraURL, token, caCert, allowUntrustedTLS).TestConnection(a.ctx)
	if err != nil {
		return "", err
	}
	return user.DisplayName, nil
}

// testBackend builds a throwaway backend for one connection test. A demo URL
// answers offline; anything else gets a Jira client with the given TLS
// settings. The requirement type is irrelevant to a test, so it stays blank.
func testBackend(jiraURL, token, caCert string, allowUntrustedTLS bool) backend.IssueBackend {
	if suiteprofiles.IsDemoURL(jiraURL) {
		return demobackend.New("")
	}
	return jirabackend.New(
		corejira.NewClient(strings.TrimSpace(jiraURL), strings.TrimSpace(token),
			tlsOptionsFor(caCert, allowUntrustedTLS)...),
		"",
	)
}

// profileConfig is the shareable shape of a profile: everything but the
// credential, which never leaves the OS credential manager. The field names
// match XTM's exporter, so a file written by either app imports into the other.
type profileConfig struct {
	Name           string `json:"name"`
	JiraURL        string `json:"jiraUrl"`
	ProjectKey     string `json:"projectKey"`
	ScopeJQL       string `json:"scopeJql"`
	BugIssueType   string `json:"bugIssueType"`
	BugProjectMode string `json:"bugProjectMode"`
	BugProjectKey  string `json:"bugProjectKey"`
	Backend        string `json:"backend"`
}

// ExportProfile writes a profile's configuration, without its token, to a
// file the user picks. It returns the path written, or "" when the dialog was
// cancelled.
func (a *App) ExportProfile(id string) (string, error) {
	if err := a.requireStore(); err != nil {
		return "", err
	}
	p, err := a.profiles.Get(id)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(profileConfig{
		Name: p.Name, JiraURL: p.JiraURL, ProjectKey: p.ProjectKey, ScopeJQL: p.ScopeJQL,
		BugIssueType: p.BugIssueType, BugProjectMode: p.BugProjectMode,
		BugProjectKey: p.BugProjectKey, Backend: p.Backend,
	}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode profile: %w", err)
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export profile",
		DefaultFilename: exportFilename(sanitizeFilename(p.Name) + "-profile.json"),
		Filters:         []runtime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil {
		return "", fmt.Errorf("save dialog: %w", err)
	}
	if path == "" {
		return "", nil // cancelled
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write profile: %w", err)
	}
	return path, nil
}

// ImportProfile creates a profile from a JSON config file the user picks. The
// new profile has no credential yet, so open it and enter one before syncing.
// A zero profile (empty id) means the dialog was cancelled.
func (a *App) ImportProfile() (profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return profile.Profile{}, err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Import profile",
		Filters: []runtime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil {
		return profile.Profile{}, fmt.Errorf("open dialog: %w", err)
	}
	if path == "" {
		return profile.Profile{}, nil // cancelled
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("read file: %w", err)
	}
	var cfg profileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return profile.Profile{}, fmt.Errorf("not a valid profile file: %w", err)
	}
	if err := suiteprofiles.ValidateFields(cfg.Name, cfg.JiraURL, cfg.ProjectKey); err != nil {
		return profile.Profile{}, fmt.Errorf("profile file: %w", err)
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Backend), "kiwi") {
		return profile.Profile{}, errors.New("that is a Kiwi TCMS profile; Task Activity Manager talks to Jira only")
	}
	return a.profiles.Create(
		strings.TrimSpace(cfg.Name), strings.TrimSpace(cfg.JiraURL), strings.TrimSpace(cfg.ProjectKey),
		strings.TrimSpace(cfg.ScopeJQL), cfg.BugIssueType, cfg.BugProjectMode, cfg.BugProjectKey,
		"", false, suiteprofiles.Backend,
	)
}

// sanitizeFilename strips the characters a profile name may carry that a
// filename may not, so it can seed the export dialog's default name.
func sanitizeFilename(name string) string {
	repl := strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-", "*", "-", "?", "-",
		"\"", "-", "<", "-", ">", "-", "|", "-", " ", "_",
	)
	out := strings.TrimSpace(repl.Replace(name))
	if out == "" {
		return "profile"
	}
	return out
}

// exportFilename prefixes a default filename with a local YYYYMMDDHHMM stamp,
// so exports sort chronologically and a second one does not silently
// overwrite the first.
func exportFilename(name string) string {
	return time.Now().Format("200601021504") + "_" + name
}
