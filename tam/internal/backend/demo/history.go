package demo

import (
	"context"
	"strings"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
)

// Nothing calls this method outside a test yet; its future caller reaches
// it through a type assertion, so drift here would silently drop the
// offline sprint report before that caller exists to notice. This line
// fails the build instead.
var _ backend.HistoryBackend = (*Backend)(nil)

// curatedHistory is the changelog behind Sprint 11, the demo's one closed
// sprint, keyed by the dataset's own PLAT keys (canonicalKey below maps a
// rekeyed profile key back to them). It models what a real instance's
// changelog read actually sees, per the design's section 3, rather than
// what a naive "every sprint an issue ever carried" reading would show:
// `sprint = 11` only ever returns whoever is in the sprint right now, so a
// card the changelog shows leaving mid-sprint and never coming back is not
// merely unreported, it is never fetched at all. Four cards exercise that
// end to end:
//
//   - PLAT-331 finishes before the sprint even starts (2026-08-04), which
//     is what a "done before the start" case checks for rather than
//     assuming every card's history begins empty at the sprint boundary.
//     Its cached sprint is still 11.
//   - PLAT-385 leaves the sprint and comes back, which a scope count has
//     to charge once, not twice, and not zero times because it ended up
//     back where it started. Its cached sprint is still 11.
//   - PLAT-347 is added to the sprint after it has already started, a
//     scope increase, and stays. Its cached sprint is still 11, which is
//     what keeps it demonstrable at all: a card the search cannot see
//     cannot show a scope increase either.
//   - PLAT-401 is re-estimated mid-sprint (2 points to 3, the value the
//     dataset already carries for it) and also stays. Same reason.
//   - PLAT-398 is added to the sprint after it starts and then moves on to
//     Sprint 13, its cached sprint today, and never returns. It is left in
//     this map on purpose and SearchIssuesWithHistory below does not find
//     it: that absence is the blind spot section 3 describes, not a bug in
//     this dataset.
//
// This mapping, PLAT-398 included, is the documented assumption behind
// section 3 rather than a measured fact: probe 3 in
// docs/superpowers/plans/assets/2026-09-11-report-wire-probe.md, which
// checks whether `sprint = N` on a real Data Center truly cannot see a
// card that left, has not been run yet.
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
	},
	"PLAT-401": {
		{At: "2026-08-05T09:00:00.000+0000", Field: "sprint", From: "", To: "Sprint 11"},
		{At: "2026-08-09T09:00:00.000+0000", Field: "storyPoints", From: "2", To: "3"},
	},
	"PLAT-398": {
		{At: "2026-08-07T09:00:00.000+0000", Field: "sprint", From: "", To: "Sprint 11"},
		{At: "2026-08-13T09:00:00.000+0000", Field: "sprint", From: "Sprint 11", To: "Sprint 13"},
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

// SearchIssuesWithHistory answers exactly the scope SearchIssuesPage does
// (see its own comment on why "sprint = N" is the one this dataset
// honours): an issue is in scope only when its cached sprint is the one
// asked for, right now, never because its history once passed through it.
// A card the changelog shows leaving mid-sprint and never coming back is
// therefore not in the result at all, the same blind spot a real instance
// has and section 3 of the design names. Each hit carries its curated
// changelog, empty for the issues the dataset carries no history for.
func (b *Backend) SearchIssuesWithHistory(_ context.Context, jql string, startAt, maxResults int) ([]backend.IssueHistory, int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	sprintID, bySprint := sprintScope(jql)
	var all []backend.Issue
	for _, iss := range b.issues() {
		if bySprint && iss.SprintID != sprintID {
			continue
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
