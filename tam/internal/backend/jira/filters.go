package jira

import (
	"context"

	"agile-suite/tam/internal/backend"
)

// Nothing calls these outside the Dashboards view, which reaches them
// through a type assertion, so drift here would fail that view against a
// real Jira with nothing in the build to say so. This line fails the
// build instead.
var _ backend.FilterBackend = (*Backend)(nil)

// Filters is the saved filters this user can pin, with the JQL each one
// stands for and who wrote it.
//
// Data Center has no route that lists the filters a user owns. /filter/my
// is Cloud only and 404s there, which #346 built on and #348 took back
// out. What Data Center does answer is the owner of each filter in the
// starred list, so the scope is read per filter rather than per list.
//
// This leaves one gap, recorded rather than papered over: a filter nobody
// starred is in no list Data Center will enumerate, so it cannot be
// offered. #349 covers reaching those by id.
func (b *Backend) Filters(ctx context.Context) ([]backend.Filter, error) {
	starred, err := b.c.FavouriteFilters(ctx)
	if err != nil {
		return nil, err
	}

	// Who is reading decides the scope, and failing to find out costs the
	// labels rather than the list. The filters themselves read fine, so
	// dropping the view over a second read would lose more than it
	// protects, and an unlabelled list is what this view showed before.
	reader := ""
	if me, err := b.c.Myself(ctx); err == nil {
		reader = me.Name
	}

	out := make([]backend.Filter, 0, len(starred))
	for _, f := range starred {
		out = append(out, backend.Filter{
			ID:    f.ID,
			Name:  f.Name,
			JQL:   f.JQL,
			Scope: filterScope(reader, f.Owner.Name),
		})
	}
	return out, nil
}

// filterScope names a filter by who wrote it. Both names have to be known:
// an unread owner and an unread reader are both the empty string, and
// treating that as a match would label every filter on an instance that
// names its users another way as this reader's own.
func filterScope(reader, owner string) string {
	if reader == "" || owner == "" {
		return ""
	}
	if owner == reader {
		return backend.FilterScopeOwned
	}
	return backend.FilterScopeShared
}

// SearchByJQL runs a filter's own words and maps the hits onto rows.
//
// The JQL is sent unchanged. Everywhere else in TAM a search is scoped to
// the profile's project, because everywhere else the question is about
// that project; a saved filter is the user's own question and may well
// span projects, so narrowing it here would answer something they did not
// ask. Each hit resolves its own project's types for that reason too.
func (b *Backend) SearchByJQL(ctx context.Context, jql string, startAt, maxResults int) ([]backend.Issue, int, error) {
	ids := b.discover(ctx)
	fields := append(append([]string{}, baseFields...), ids.list()...)
	page, err := b.c.SearchIssues(ctx, jql, fields, nil, startAt, maxResults)
	if err != nil {
		return nil, 0, err
	}
	out := make([]backend.Issue, 0, len(page.Issues))
	for _, raw := range page.Issues {
		pt := b.typesOrEmpty(ctx, projectOf(raw.Key))
		out = append(out, parseIssue(raw, ids, b.requirementType, pt))
	}
	return out, page.Total, nil
}
