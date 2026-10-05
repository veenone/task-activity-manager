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

// Filters is the saved filters this user has starred, with the JQL each
// one stands for.
func (b *Backend) Filters(ctx context.Context) ([]backend.Filter, error) {
	raw, err := b.c.FavouriteFilters(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]backend.Filter, 0, len(raw))
	for _, f := range raw {
		out = append(out, backend.Filter{ID: f.ID, Name: f.Name, JQL: f.JQL})
	}
	return out, nil
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
