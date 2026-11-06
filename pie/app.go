package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	corejira "agile-suite/core/jira"
	"agile-suite/core/profile"
	"agile-suite/core/store"
	"agile-suite/pie/internal/cases"
	"agile-suite/pie/internal/demo"
	"agile-suite/pie/internal/evidence"
	"agile-suite/pie/internal/pieprofile"
	"agile-suite/pie/internal/query"
	"agile-suite/pie/internal/saved"
)

// App is the backend bound to the React frontend. Every exported method here
// is callable from JavaScript, so it only validates and delegates.
type App struct {
	ctx        context.Context
	db         *store.DB
	profiles   *profile.Manager
	creds      profile.CredentialStore
	queries    *saved.Store
	cases      *cases.Store
	evidence   *evidence.Store
	startupErr string
}

// startup records a failure rather than returning it: the Windows build has no
// console, so it has to reach the window as data, through StartupError.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	path, err := pieprofile.DefaultPath()
	if err == nil {
		// Core's profile tables and PIE's own, in PIE's file. See pieprofile.
		a.db, err = store.Open(path, saved.Schema())
	}
	if err != nil {
		a.startupErr = fmt.Sprintf("open the database: %v", err)
		log.Printf("pie: %s", a.startupErr)
		return
	}
	a.profiles = profile.NewManager(a.db.DB())
	a.creds = profile.NewCredentialStore()
	a.queries = saved.New(a.db.DB())
	a.cases = cases.New(a.db.DB())
	a.evidence = evidence.New(a.db.DB())
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
	// Saved runs go first: if this fails the profile is still there to retry.
	if err := a.queries.Purge(id); err != nil {
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

// ListQueries returns a profile's saved queries.
func (a *App) ListQueries(profileID string) ([]saved.Query, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.queries.List(profileID)
}

// SaveQuery saves JQL under a name.
func (a *App) SaveQuery(profileID, name, jql string) (saved.Query, error) {
	if err := a.requireStore(); err != nil {
		return saved.Query{}, err
	}
	return a.queries.Save(profileID, name, jql)
}

// DeleteQuery removes a saved query and its stored run.
func (a *App) DeleteQuery(profileID string, id int64) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.queries.Delete(profileID, id)
}

// LastRun is a saved query's stored run. It does not contact Jira.
func (a *App) LastRun(profileID string, id int64) (saved.Run, error) {
	if err := a.requireStore(); err != nil {
		return saved.Run{}, err
	}
	return a.queries.Last(profileID, id)
}

// Fields returns the names of the profile's Jira custom fields.
func (a *App) Fields(profileID string) (saved.Fields, error) {
	if err := a.requireStore(); err != nil {
		return saved.Fields{}, err
	}
	return a.queries.Fields(profileID)
}

// SetFields stores the profile's field names after checking each named one
// against Jira's field list. If any is refused, none is stored. Clearing every
// name needs no Jira.
func (a *App) SetFields(profileID string, f saved.Fields) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if f != (saved.Fields{}) {
		client, err := a.client(profileID)
		if err != nil {
			return err
		}
		if _, err := fieldIDs(a.ctx, client, f); err != nil {
			return err
		}
	}
	return a.queries.SetFields(profileID, f)
}

// ListCases returns the profile's cases.
func (a *App) ListCases(profileID string) ([]cases.Case, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.cases.List(profileID)
}

// OpenCases opens a case for each issue key and returns the profile's cases.
func (a *App) OpenCases(profileID string, keys []string) ([]cases.Case, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	if err := a.cases.Open(profileID, keys, time.Now()); err != nil {
		return nil, err
	}
	return a.cases.List(profileID)
}

// JiraFilters lists the saved filters the profile's account has starred.
func (a *App) JiraFilters(profileID string) ([]corejira.RawFilter, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	client, err := a.client(profileID)
	if err != nil {
		return nil, err
	}
	return query.Starred(a.ctx, client)
}

// JiraFilter reads one saved filter by id, which reaches a filter nobody starred.
func (a *App) JiraFilter(profileID, id string) (corejira.RawFilter, error) {
	if err := a.requireStore(); err != nil {
		return corejira.RawFilter{}, err
	}
	client, err := a.client(profileID)
	if err != nil {
		return corejira.RawFilter{}, err
	}
	return query.Filter(a.ctx, client, id)
}

// client is the Jira connection for a profile: demo's, or the instance's with
// the token from the credential store.
func (a *App) client(profileID string) (*corejira.Client, error) {
	p, err := a.profiles.Get(profileID)
	if err != nil {
		return nil, err
	}
	if demo.Is(p.JiraURL) {
		return demo.Client(), nil
	}
	token, err := a.creds.Load(p.ID)
	if err != nil {
		return nil, fmt.Errorf("load the profile's token: %w", err)
	}
	return corejira.NewClient(p.JiraURL, token), nil
}

// fieldIDs resolves each named field to its id, in words an analyst can act
// on. An empty name stays an empty id.
func fieldIDs(ctx context.Context, client *corejira.Client, f saved.Fields) (query.Fields, error) {
	var ids query.Fields
	for _, x := range []struct {
		name, what string
		id         *string
	}{
		{f.ProductBase, "product base", &ids.Base},
		{f.ClientVisibility, "client visibility", &ids.Visibility},
		{f.ClientImpact, "client impact", &ids.Impact},
	} {
		name := strings.TrimSpace(x.name)
		if name == "" {
			continue
		}
		id, err := client.CustomFieldID(ctx, name)
		switch {
		case errors.Is(err, corejira.ErrFieldNotFound):
			return query.Fields{}, fmt.Errorf("Jira has no custom field named %q. Check the %s field's name", name, x.what)
		case errors.Is(err, corejira.ErrFieldAmbiguous):
			return query.Fields{}, fmt.Errorf("Jira has more than one custom field named %q, so PIE cannot tell which is the %s", name, x.what)
		case err != nil:
			return query.Fields{}, fmt.Errorf("read Jira's field list: %w", err)
		}
		*x.id = id
	}
	return ids, nil
}

// RunQuery runs JQL with a profile's connection and follows its links. With a
// saved query's id the run is stored on it and compared with the last one; 0
// runs the JQL without saving anything. confirmed says the analyst has agreed
// to a fetch larger than query.AskAbove.
func (a *App) RunQuery(profileID string, queryID int64, jql string, confirmed bool) (saved.Run, error) {
	if err := a.requireStore(); err != nil {
		return saved.Run{}, err
	}
	client, err := a.client(profileID)
	if err != nil {
		return saved.Run{}, err
	}
	names, err := a.queries.Fields(profileID)
	if err != nil {
		return saved.Run{}, err
	}
	ids, err := fieldIDs(a.ctx, client, names)
	if err != nil {
		return saved.Run{}, err
	}
	res, err := query.Explore(a.ctx, client, jql, confirmed, ids)
	if err != nil || queryID == 0 || res.NeedsConfirm {
		return saved.Run{Result: res}, err
	}
	return a.queries.Record(profileID, queryID, jql, res, time.Now())
}
