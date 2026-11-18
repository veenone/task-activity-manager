package main

import (
	"errors"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/worklogweek"
)

// WorklogWeek is what the Worklog view draws for the days fromDay to toDay
// ("2026-11-16"): the connected user's open issues, the other issues they
// logged on, and their own entries, Jira's and the journal's together.
//
// The issues come from the cache and the entries from Jira, read each time
// like the panel's Work log section: nothing here is stored. Who "the
// connected user" is comes from the username a sync or a connection test
// recorded, and without one there is nobody to ask Jira about.
func (a *App) WorklogWeek(profileID, fromDay, toDay string) (worklogweek.View, error) {
	if err := worklogweek.CheckRange(fromDay, toDay); err != nil {
		return worklogweek.View{}, err
	}
	p, b, err := a.backendForProfile(profileID)
	if err != nil {
		return worklogweek.View{}, err
	}
	me, err := a.repo.ProfileSetting(a.ctx, p.ID, settingJiraUsername)
	if err != nil {
		return worklogweek.View{}, err
	}
	if me == "" {
		return worklogweek.View{}, errors.New("TAM does not know your Jira user yet. Sync once or test the connection")
	}
	own, ok := b.(backend.OwnWorklogBackend)
	if !ok {
		return worklogweek.View{}, errors.New("this connection cannot read worklogs")
	}
	displayName, err := a.repo.ProfileSetting(a.ctx, p.ID, settingJiraDisplayName)
	if err != nil {
		return worklogweek.View{}, err
	}
	assigned, err := a.repo.ListIssuesForExport(a.ctx, p.ID, issuerepo.IssueQuery{AssigneeName: me, AssigneeDisplayName: displayName})
	if err != nil {
		return worklogweek.View{}, err
	}
	remote, err := own.WorklogsBy(a.ctx, me, fromDay, toDay)
	if err != nil {
		return worklogweek.View{}, err
	}
	pending, err := a.repo.AllPendingWorklogs(a.ctx, p.ID)
	if err != nil {
		return worklogweek.View{}, err
	}
	lookup := func(key string) (backend.Issue, bool) {
		iss, err := a.repo.GetIssue(a.ctx, p.ID, key)
		return iss, err == nil
	}
	return worklogweek.Build(assigned, lookup, remote, pending, fromDay, toDay), nil
}
