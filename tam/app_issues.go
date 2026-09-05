package main

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	corejira "agile-suite/core/jira"
	"agile-suite/core/profile"
	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
	jirabackend "agile-suite/tam/internal/backend/jira"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/suiteprofiles"
	"agile-suite/tam/internal/syncer"
)

const (
	// settingRequirementType is the per-profile key for the Jira issue type
	// name TAM treats as a requirement.
	settingRequirementType = "requirement_issue_type"
	// detailFreshFor is how long a cached detail is served without asking
	// Jira again.
	detailFreshFor = 10 * time.Minute
	// syncProgressEvent carries syncer.Progress frames to the frontend.
	syncProgressEvent = "tam:sync-progress"
)

// requireProfile is requireStore plus the profile row, so every issue
// method starts from a profile that exists.
func (a *App) requireProfile(profileID string) (profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return profile.Profile{}, err
	}
	if strings.TrimSpace(profileID) == "" {
		return profile.Profile{}, errors.New("no profile selected")
	}
	p, err := a.profiles.Get(profileID)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("profile %s: %w", profileID, err)
	}
	return p, nil
}

// backendFor returns the profile's backend, building it on first use. A
// demo URL gets the offline dataset; anything else gets a Jira client with
// the profile's TLS settings and the PAT from the credential store. The
// token goes into the client and nowhere else.
func (a *App) backendFor(p profile.Profile) (backend.IssueBackend, error) {
	a.backendMu.Lock()
	defer a.backendMu.Unlock()
	if b, ok := a.backends[p.ID]; ok {
		return b, nil
	}
	var b backend.IssueBackend
	if suiteprofiles.IsDemoURL(p.JiraURL) {
		b = demobackend.New(p.ProjectKey)
	} else {
		token, err := a.creds.Load(p.ID)
		if err != nil {
			return nil, fmt.Errorf("read the token for %s: %w", p.Name, err)
		}
		reqType, err := a.repo.ProfileSetting(a.ctx, p.ID, settingRequirementType)
		if err != nil {
			return nil, err
		}
		client := corejira.NewClient(p.JiraURL, token, tlsOptions(p)...)
		b = jirabackend.New(client, reqType)
	}
	a.backends[p.ID] = b
	return b, nil
}

// forgetBackend drops a cached backend so the next call rebuilds it, which
// is what changing the requirement type needs.
func (a *App) forgetBackend(profileID string) {
	a.backendMu.Lock()
	delete(a.backends, profileID)
	a.backendMu.Unlock()
}

// tlsOptions derives the client options from a profile's TLS settings.
func tlsOptions(p profile.Profile) []corejira.Option {
	var opts []corejira.Option
	if strings.TrimSpace(p.CACert) != "" {
		opts = append(opts, corejira.WithCACert(p.CACert))
	}
	if p.AllowUntrustedTLS {
		opts = append(opts, corejira.WithInsecureTLS(true))
	}
	return opts
}

func (a *App) emitProgress(p syncer.Progress) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, syncProgressEvent, p)
	}
}

// SyncIssues pulls the profile's issues, incrementally or in full, and
// emits tam:sync-progress frames while it runs. It returns when the sync
// has finished or failed.
func (a *App) SyncIssues(profileID string, full bool) (syncer.Summary, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return syncer.Summary{}, err
	}
	b, err := a.backendFor(p)
	if err != nil {
		return syncer.Summary{}, err
	}
	sum, err := syncer.New(b, a.repo).Sync(a.ctx, p.ID, p.ProjectKey, p.ScopeJQL, full, a.emitProgress)
	if err != nil {
		log.Printf("tam: sync %s (%s) failed: %v", p.Name, p.ProjectKey, err)
		return sum, err
	}
	log.Printf("tam: synced %s (%s): %d fetched, %d upserted, %d skipped in %s", p.Name, p.ProjectKey, sum.Fetched, sum.Upserted, sum.Skipped, sum.Elapsed)
	return sum, nil
}

// GetSyncState reports when the profile last synced and how many issues
// are cached.
func (a *App) GetSyncState(profileID string) (issuerepo.SyncState, error) {
	if err := a.requireStore(); err != nil {
		return issuerepo.SyncState{}, err
	}
	return a.repo.SyncState(a.ctx, profileID)
}

// ListIssues is the Backlog grid's page.
func (a *App) ListIssues(profileID string, q issuerepo.IssueQuery) (issuerepo.IssuePage, error) {
	if err := a.requireStore(); err != nil {
		return issuerepo.IssuePage{}, err
	}
	return a.repo.ListIssues(a.ctx, profileID, q)
}

// GetIssueDetail returns the cached detail when it is fresh, otherwise
// fetches it from the backend and caches it.
func (a *App) GetIssueDetail(profileID, key string) (backend.IssueDetail, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return backend.IssueDetail{}, err
	}
	cached, fetchedAt, ok, err := a.repo.ReadDetail(a.ctx, p.ID, key)
	if err != nil {
		return backend.IssueDetail{}, err
	}
	if ok && time.Since(fetchedAt) < detailFreshFor {
		return cached, nil
	}
	b, err := a.backendFor(p)
	if err != nil {
		return backend.IssueDetail{}, err
	}
	d, err := b.GetIssueDetail(a.ctx, key)
	if err != nil {
		return backend.IssueDetail{}, err
	}
	if err := a.repo.WriteDetail(a.ctx, p.ID, key, d, time.Now()); err != nil {
		return backend.IssueDetail{}, err
	}
	return d, nil
}

// ListLinkedTests returns the tests linked to key through the suite's
// requirement link type.
func (a *App) ListLinkedTests(profileID, key string) ([]issuerepo.LinkedTest, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	s, err := a.settings.Get()
	if err != nil {
		return nil, err
	}
	return a.repo.ListLinkedTests(a.ctx, profileID, key, s.RequirementLinkType)
}

// ListSprints returns the sprints seen in the cached issues.
func (a *App) ListSprints(profileID string) ([]issuerepo.SprintRef, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	sprints, err := a.repo.ListSprints(a.ctx, profileID)
	if err != nil {
		return nil, err
	}
	if sprints == nil {
		sprints = []issuerepo.SprintRef{}
	}
	return sprints, nil
}

// GetProfileSetting reads a per-profile TAM setting; "" when unset.
func (a *App) GetProfileSetting(profileID, key string) (string, error) {
	if err := a.requireStore(); err != nil {
		return "", err
	}
	if strings.TrimSpace(key) == "" {
		return "", errors.New("setting key is empty")
	}
	return a.repo.ProfileSetting(a.ctx, profileID, key)
}

// SetProfileSetting writes a per-profile TAM setting and drops the
// profile's cached backend, since the requirement type feeds into it.
func (a *App) SetProfileSetting(profileID, key, value string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" {
		return errors.New("setting key is empty")
	}
	if err := a.repo.SetProfileSetting(a.ctx, profileID, key, strings.TrimSpace(value)); err != nil {
		return err
	}
	a.forgetBackend(profileID)
	return nil
}
