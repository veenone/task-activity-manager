package jira

import (
	"context"

	"agile-suite/tam/internal/backend"
)

// Worklogs reads every entry Jira holds for the issue. The transport pages
// the endpoint; this only maps the wire shape onto TAM's, keeping started
// exactly as Jira sent it so the reader can show the day the work was logged
// on rather than the day TAM's own zone makes of the instant.
func (b *Backend) Worklogs(ctx context.Context, key string) ([]backend.Worklog, error) {
	raw, err := b.c.Worklogs(ctx, key)
	if err != nil {
		return nil, err
	}
	out := make([]backend.Worklog, 0, len(raw))
	for _, w := range raw {
		out = append(out, backend.Worklog{
			ID:         w.ID,
			Author:     w.Author.Name,
			AuthorName: w.Author.DisplayName,
			Started:    w.Started,
			TimeSpent:  w.TimeSpent,
			Seconds:    w.TimeSpentSeconds,
			Comment:    w.Comment,
		})
	}
	return out, nil
}

// AddWorklog logs the draft against the issue.
func (b *Backend) AddWorklog(ctx context.Context, key string, d backend.WorklogDraft) error {
	return b.c.AddWorklog(ctx, key, d.Started, d.TimeSpent, d.Comment)
}
