package main

import (
	"errors"
	"strings"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/dashboardrepo"
)

// The Dashboards view's bindings. Every one of them is a read against Jira
// or a write to the local store; nothing here writes to Jira, which is
// what keeps a dashboard inside the rule the rest of TAM follows.

// ListJiraFilters is the saved filters the connected user has starred,
// which is what a dashboard is made from.
func (a *App) ListJiraFilters(profileID string) ([]backend.Filter, error) {
	_, b, err := a.backendForProfile(profileID)
	if err != nil {
		return nil, err
	}
	f, ok := b.(backend.FilterBackend)
	if !ok {
		return nil, errors.New("this connection cannot read saved filters")
	}
	return f.Filters(a.ctx)
}

// ListDashboards is the profile's dashboards with their last figures. It
// reads the store alone, so it answers with Jira unreachable, which is the
// whole reason a dashboard holds a snapshot.
func (a *App) ListDashboards(profileID string) ([]dashboardrepo.Dashboard, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return nil, err
	}
	return a.dashboards.List(a.ctx, p.ID)
}

// CreateDashboard pins a filter, then fills it in. A dashboard that opened
// empty and waited to be refreshed would make the reader do the one step
// the create already knows it needs.
//
// A failed first refresh leaves the dashboard in place with no figures and
// reports why: the pin is the user's decision and Jira being unreachable
// does not undo it.
func (a *App) CreateDashboard(profileID, name, filterID, jql string) (dashboardrepo.Dashboard, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return dashboardrepo.Dashboard{}, err
	}
	d, err := a.dashboards.Create(a.ctx, p.ID, name, filterID, jql)
	if err != nil {
		return dashboardrepo.Dashboard{}, err
	}
	filled, err := a.refreshDashboard(p.ID, d.ID)
	if err != nil {
		return d, err
	}
	return filled, nil
}

// RefreshDashboard re-runs one dashboard's filter and stores what came
// back.
func (a *App) RefreshDashboard(profileID, id string) (dashboardrepo.Dashboard, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return dashboardrepo.Dashboard{}, err
	}
	return a.refreshDashboard(p.ID, strings.TrimSpace(id))
}

// DeleteDashboard unpins one.
func (a *App) DeleteDashboard(profileID, id string) error {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return err
	}
	return a.dashboards.Delete(a.ctx, p.ID, strings.TrimSpace(id))
}

// refreshDashboard is the half CreateDashboard and RefreshDashboard share:
// the backend lookup, the capability check, and the stamp the snapshot is
// written under, which is this machine's clock because nothing in Jira's
// answer says when it was taken.
func (a *App) refreshDashboard(profileID, id string) (dashboardrepo.Dashboard, error) {
	_, b, err := a.backendForProfile(profileID)
	if err != nil {
		return dashboardrepo.Dashboard{}, err
	}
	f, ok := b.(backend.FilterBackend)
	if !ok {
		return dashboardrepo.Dashboard{}, errors.New("this connection cannot run a filter")
	}
	return a.dashboards.Refresh(a.ctx, f, profileID, id, time.Now().UTC().Format(time.RFC3339))
}
