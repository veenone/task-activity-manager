package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	corejira "agile-suite/core/jira"
	"agile-suite/core/profile"
	"agile-suite/core/shareddb"
	"agile-suite/core/store"
	"agile-suite/pie/internal/demo"
	"agile-suite/pie/internal/pieprofile"
	"agile-suite/pie/internal/query"
)

// App is the backend bound to the React frontend. Every exported method here
// is callable from JavaScript, so it only validates and delegates.
type App struct {
	ctx        context.Context
	db         *store.DB
	profiles   *profile.Manager
	creds      profile.CredentialStore
	startupErr string
}

// startup records a failure rather than returning it: the Windows build has no
// console, so it has to reach the window as data, through StartupError.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	path, err := pieprofile.DefaultPath()
	if err == nil {
		// Core's profile tables, in PIE's own file. See pieprofile.
		a.db, err = shareddb.Open(path)
	}
	if err != nil {
		a.startupErr = fmt.Sprintf("open the database: %v", err)
		log.Printf("pie: %s", a.startupErr)
		return
	}
	a.profiles = profile.NewManager(a.db.DB())
	a.creds = profile.NewCredentialStore()
}

func (a *App) shutdown(context.Context) {
	if a.db != nil {
		if err := a.db.Close(); err != nil {
			log.Printf("pie: close database: %v", err)
		}
	}
}

// StartupError is why the app cannot work, or "" when it can.
func (a *App) StartupError() string { return a.startupErr }

func (a *App) requireStore() error {
	if a.profiles == nil {
		return errors.New("the database is not available: " + a.startupErr)
	}
	return nil
}

// ListProfiles returns PIE's profiles.
func (a *App) ListProfiles() ([]profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.profiles.List()
}

// CreateProfile saves a profile and puts its token in the OS credential
// manager. The project key is empty on purpose: scope comes from the query.
func (a *App) CreateProfile(name, jiraURL, token string) (profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return profile.Profile{}, err
	}
	if err := pieprofile.ValidateNew(name, jiraURL, token); err != nil {
		return profile.Profile{}, err
	}
	p, err := a.profiles.Create(strings.TrimSpace(name), strings.TrimSpace(jiraURL), "", "", "", "", "", "", false, "")
	if err != nil {
		return profile.Profile{}, err
	}
	if demo.Is(p.JiraURL) {
		return p, nil // no token to keep
	}
	if err := a.creds.Save(p.ID, strings.TrimSpace(token)); err != nil {
		if delErr := a.profiles.Delete(p.ID); delErr != nil {
			log.Printf("pie: rollback profile %s after credential save failure: %v", p.ID, delErr)
		}
		return profile.Profile{}, fmt.Errorf("save credentials: %w", err)
	}
	return p, nil
}

// DeleteProfile removes a profile and its token.
func (a *App) DeleteProfile(id string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	p, err := a.profiles.Get(id)
	if err != nil {
		return err
	}
	if err := a.profiles.Delete(id); err != nil {
		return err
	}
	if demo.Is(p.JiraURL) {
		return nil
	}
	if err := a.creds.Delete(id); err != nil {
		log.Printf("pie: delete credentials for %s: %v", id, err)
	}
	return nil
}

// RunQuery runs JQL with a profile's connection.
func (a *App) RunQuery(profileID, jql string) (query.Result, error) {
	if err := a.requireStore(); err != nil {
		return query.Result{}, err
	}
	p, err := a.profiles.Get(profileID)
	if err != nil {
		return query.Result{}, err
	}
	if demo.Is(p.JiraURL) {
		return query.Run(a.ctx, demo.Client(), jql)
	}
	token, err := a.creds.Load(p.ID)
	if err != nil {
		return query.Result{}, fmt.Errorf("load the profile's token: %w", err)
	}
	return query.Run(a.ctx, corejira.NewClient(p.JiraURL, token), jql)
}
