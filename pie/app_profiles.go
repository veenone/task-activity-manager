package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"agile-suite/core/profile"
	"agile-suite/pie/internal/demo"
	"agile-suite/pie/internal/pieprofile"
	"agile-suite/pie/internal/workbook"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// The bindings behind Manage Profiles (#453), shaped like TAM's and XTM's.

// UpdateProfile changes a profile's name and Jira URL in place, so its saved
// queries, cases and cycles stay. An empty token keeps the stored one.
func (a *App) UpdateProfile(id, name, jiraURL, token string) (profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return profile.Profile{}, err
	}
	name, jiraURL, token = strings.TrimSpace(name), strings.TrimSpace(jiraURL), strings.TrimSpace(token)
	was, err := a.profiles.Get(id)
	if err != nil {
		return profile.Profile{}, err
	}
	// The stored token belongs to the stored Jira, so a new Jira needs its own
	// and the old one is never sent there. A profile moving off the demo has
	// no stored token at all.
	moved := !strings.EqualFold(strings.TrimRight(jiraURL, "/"), strings.TrimRight(was.JiraURL, "/"))
	if moved && token == "" && !demo.Is(jiraURL) {
		return profile.Profile{}, errors.New("enter the personal access token for the new Jira URL")
	}
	have := token
	if have == "" {
		have, _ = a.creds.Load(id)
	}
	if err := pieprofile.ValidateNew(name, jiraURL, have); err != nil {
		return profile.Profile{}, err
	}
	if err := a.profiles.Update(id, name, jiraURL, "", "", "", "", "", "", false, ""); err != nil {
		return profile.Profile{}, err
	}
	if token != "" && !demo.Is(jiraURL) {
		if err := a.creds.Save(id, token); err != nil {
			return profile.Profile{}, fmt.Errorf("save credentials: %w", err)
		}
	}
	return a.profiles.Get(id)
}

// DefaultProfile is the profile PIE opens at launch, "" for none.
func (a *App) DefaultProfile() string {
	if a.settings == nil {
		return ""
	}
	s, err := a.settings.Get()
	if err != nil {
		return ""
	}
	return s.DefaultProfileID
}

// SetDefaultProfile records the profile PIE opens at launch. "" clears it.
func (a *App) SetDefaultProfile(id string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.settings.SetDefaultProfileID(id)
}

// profileConfig is a profile file: everything but the tokens.
type profileConfig struct {
	Name                 string `json:"name"`
	JiraURL              string `json:"jiraUrl"`
	ConfluenceURL        string `json:"confluenceUrl,omitempty"`
	ConfluenceSpace      string `json:"confluenceSpace,omitempty"`
	ConfluenceRootPageID string `json:"confluenceRootPageId,omitempty"`
}

func profileFile(p profile.Profile, cc profile.ConfluenceConfig) ([]byte, error) {
	return json.MarshalIndent(profileConfig{
		Name: p.Name, JiraURL: p.JiraURL,
		ConfluenceURL: cc.BaseURL, ConfluenceSpace: cc.SpaceKey, ConfluenceRootPageID: cc.RootPageID,
	}, "", "  ")
}

// readProfileFile parses and checks a profile file. The file came from
// outside PIE, so it is checked like typed input.
func readProfileFile(data []byte) (profileConfig, error) {
	var cfg profileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("not a profile file: %w", err)
	}
	cfg.Name, cfg.JiraURL = strings.TrimSpace(cfg.Name), strings.TrimSpace(cfg.JiraURL)
	if err := pieprofile.ValidateFields(cfg.Name, cfg.JiraURL); err != nil {
		return cfg, fmt.Errorf("profile file: %w", err)
	}
	return cfg, nil
}

// ExportProfile writes a profile, without its tokens, to a file the analyst
// picks. It returns the path, or "" when the dialog was cancelled.
func (a *App) ExportProfile(id string) (string, error) {
	if err := a.requireStore(); err != nil {
		return "", err
	}
	p, err := a.profiles.Get(id)
	if err != nil {
		return "", err
	}
	cc, err := a.profiles.ConfluenceConfig(id)
	if err != nil {
		return "", err
	}
	data, err := profileFile(p, cc)
	if err != nil {
		return "", err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export profile",
		DefaultFilename: workbook.FileName(p.Name) + "-profile.json",
		Filters:         []runtime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, data, 0o644)
}

// ImportProfile creates a profile from a file the analyst picks. It has no
// token yet: the analyst opens it and enters one. A zero profile means the
// dialog was cancelled.
func (a *App) ImportProfile() (profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return profile.Profile{}, err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Import profile",
		Filters: []runtime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return profile.Profile{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return profile.Profile{}, err
	}
	cfg, err := readProfileFile(data)
	if err != nil {
		return profile.Profile{}, err
	}
	p, err := a.profiles.Create(cfg.Name, cfg.JiraURL, "", "", "", "", "", "", false, "")
	if err != nil {
		return profile.Profile{}, err
	}
	if cfg.ConfluenceURL != "" {
		cc := profile.ConfluenceConfig{BaseURL: cfg.ConfluenceURL, SpaceKey: cfg.ConfluenceSpace, RootPageID: cfg.ConfluenceRootPageID}
		if err := a.profiles.SetConfluenceConfig(p.ID, cc); err != nil {
			return profile.Profile{}, err
		}
	}
	return p, nil
}
