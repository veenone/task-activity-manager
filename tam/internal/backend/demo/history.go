package demo

import (
	"context"
	"strings"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
)

// The only consumer is a type assertion, so drift here would silently drop
// the offline sprint report; this fails the build instead.
var _ backend.HistoryBackend = (*Backend)(nil)

// curatedHistory is the changelog behind Sprint 11, the demo's one closed
// sprint, keyed by the dataset's own PLAT keys (canonicalKey below maps a
// rekeyed profile key back to them). It carries one of each event a
// sprint report has to tell apart, so the report built on top of this task
// can be exercised end to end without a real instance:
//
//   - PLAT-331 finishes before the sprint even starts (2026-08-04), which
//     is what a "done before the start" case checks for rather than
//     assuming every card's history begins empty at the sprint boundary.
//   - PLAT-385 leaves the sprint and comes back, which a scope count has
//     to charge once, not twice, and not zero times because it ended up
//     back where it started.
//   - PLAT-347 is added to the sprint after it has already started, a
//     scope increase, and later moves on to Sprint 13, the sprint the
//     dataset already has it in today.
//   - PLAT-401 is re-estimated mid-sprint (2 points to 3, the value the
//     dataset already carries for it) before it too moves on to Sprint 12,
//     its current sprint.
//
// PLAT-347 and PLAT-401 end up outside Sprint 11 in the dataset's own
// current fields, which is deliberate and not a mismatch: Jira's Sprint
// field is a list of every sprint an issue has ever carried, so `sprint =
// 11` matches both of them even after they moved on, and a sprint report
// has to see that scope change to report it. SearchIssuesWithHistory
// mirrors that below rather than filtering on the issue's current sprint
// alone.
var curatedHistory = map[string][]backend.Change{
	"PLAT-331": {
		{At: "2026-07-21T09:00:00.000+0000", Field: "sprint", From: "", To: "Sprint 11"},
		{At: "2026-08-01T09:00:00.000+0000", Field: "status", From: "In Progress", To: "Done"},
	},
	"PLAT-385": {
		{At: "2026-08-04T09:00:00.000+0000", Field: "sprint", From: "", To: "Sprint 11"},
		{At: "2026-08-08T09:00:00.000+0000", Field: "sprint", From: "Sprint 11", To: ""},
		{At: "2026-08-11T09:00:00.000+0000", Field: "sprint", From: "", To: "Sprint 11"},
		{At: "2026-08-15T09:00:00.000+0000", Field: "status", From: "In Progress", To: "Done"},
	},
	"PLAT-347": {
		{At: "2026-08-06T09:00:00.000+0000", Field: "sprint", From: "", To: "Sprint 11"},
		{At: "2026-08-17T09:00:00.000+0000", Field: "sprint", From: "Sprint 11", To: "Sprint 13"},
	},
	"PLAT-401": {
		{At: "2026-08-05T09:00:00.000+0000", Field: "sprint", From: "", To: "Sprint 11"},
		{At: "2026-08-09T09:00:00.000+0000", Field: "storyPoints", From: "2", To: "3"},
		{At: "2026-08-17T09:00:00.000+0000", Field: "sprint", From: "Sprint 11", To: "Sprint 12"},
	},
}

// canonicalKey reverses the rekeying demo.Issues does on the way out, so a
// profile key can be looked up in curatedHistory, which is written against
// the dataset's own PLAT keys.
func canonicalKey(key, projectKey string) string {
	if projectKey != "" && projectKey != demo.ProjectKey && strings.HasPrefix(key, projectKey+"-") {
		return demo.ProjectKey + strings.TrimPrefix(key, projectKey)
	}
	return key
}

// changesFor is a copy of key's curated changes, empty for a key the
// dataset carries no history for. Copying rather than returning the map's
// own slice means a caller can never mutate curatedHistory through it.
func changesFor(key, projectKey string) []backend.Change {
	cs, ok := curatedHistory[canonicalKey(key, projectKey)]
	if !ok {
		return nil
	}
	out := make([]backend.Change, len(cs))
	copy(out, cs)
	return out
}

// touchedSprint reports whether changes ever moved the issue into or out of
// sprintName, which is what a "sprint = N" scope means on Jira's own Sprint
// field: it is a list of every sprint the issue has ever carried, not just
// the last one, so `sprint = 11` keeps matching a card long after it moved
// on to Sprint 12.
func touchedSprint(changes []backend.Change, sprintName string) bool {
	for _, c := range changes {
		if c.Field == "sprint" && (c.From == sprintName || c.To == sprintName) {
			return true
		}
	}
	return false
}

// SearchIssuesWithHistory answers the scope SearchIssuesPage does (see its
// own comment on why "sprint = N" is the one this dataset honours), widened
// to include an issue whose curated history ever passed through the
// queried sprint even though its current sprint has since moved on, and
// attaches each hit's curated changelog, empty for the issues the dataset
// carries no history for.
func (b *Backend) SearchIssuesWithHistory(_ context.Context, jql string, startAt, maxResults int) ([]backend.IssueHistory, int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	sprintID, bySprint := sprintScope(jql)
	wantSprintName := "Sprint " + sprintID
	var all []backend.Issue
	for _, iss := range b.issues() {
		if bySprint {
			changes := changesFor(iss.Key, b.project)
			if iss.SprintID != sprintID && !touchedSprint(changes, wantSprintName) {
				continue
			}
		}
		all = append(all, iss)
	}
	total := len(all)
	if startAt >= total || maxResults <= 0 {
		return []backend.IssueHistory{}, total, nil
	}
	end := startAt + maxResults
	if end > total {
		end = total
	}
	out := make([]backend.IssueHistory, 0, end-startAt)
	for _, iss := range all[startAt:end] {
		out = append(out, backend.IssueHistory{Issue: iss, Changes: changesFor(iss.Key, b.project)})
	}
	return out, total, nil
}
