package main

import (
	"errors"
	"strings"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// ListWorklogs is what the Work log section shows: the entries Jira holds for
// the issue, then the ones the journal holds. It runs when the section is
// expanded rather than on every sync, because most rows are never asked about
// and Jira's own entries belong to Jira; nothing here is cached in the store.
//
// A draft has no issue in Jira to read, so only its journalled entries come
// back rather than an error about a key Jira has never heard of.
func (a *App) ListWorklogs(profileID, key string) ([]backend.Worklog, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("issue key is empty")
	}
	p, b, err := a.backendForProfile(profileID)
	if err != nil {
		return nil, err
	}
	logs := []backend.Worklog{}
	if !strings.HasPrefix(key, issuerepo.DraftPrefix) {
		w, ok := b.(backend.WorklogBackend)
		if !ok {
			return nil, errors.New("this connection cannot read worklogs")
		}
		remote, err := w.Worklogs(a.ctx, key)
		if err != nil {
			return nil, err
		}
		logs = append(logs, remote...)
	}
	pending, err := a.repo.PendingWorklogs(a.ctx, p.ID, key)
	if err != nil {
		return nil, err
	}
	return append(logs, pending...), nil
}

// LogWork journals an entry against the issue. time.Now() carries the
// machine's own zone, and that offset is what dates the entry in Jira: the
// user logging an hour at 01:00 logged it today, not yesterday evening in
// UTC. Nothing reaches Jira here; Commit sends it.
func (a *App) LogWork(profileID, key, timeSpent, comment string) error {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return err
	}
	d, err := backend.NewWorklogDraft(timeSpent, comment, time.Now())
	if err != nil {
		return err
	}
	return a.repo.LogWork(a.ctx, p.ID, strings.TrimSpace(key), d)
}

// CheckWorkDuration is the duration rule on its own, so the entry form can
// refuse what the user typed where they typed it instead of waiting for
// Commit to bring back Jira's 400. It is the same function LogWork puts the
// value through, which is what keeps the form and the journal from
// disagreeing about what a duration is.
func (a *App) CheckWorkDuration(timeSpent string) error {
	_, err := backend.ParseWorkSeconds(timeSpent)
	return err
}
