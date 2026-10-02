package jira

import (
	"context"
	"encoding/json"

	corejira "agile-suite/core/jira"
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
	return mapWorklogs(raw), nil
}

// AddWorklog logs the draft against the issue.
func (b *Backend) AddWorklog(ctx context.Context, key string, d backend.WorklogDraft) error {
	return b.c.AddWorklog(ctx, key, d.Started, d.TimeSpent, d.Comment)
}

// fieldWorklog is the field name a search asks the worklogs for, and the
// key they come back under.
const fieldWorklog = "worklog"

// worklogField is the envelope a search answers the field with: the same
// paged shape the endpoint uses, so total says how many entries exist
// where the list says how many came back.
type worklogField struct {
	Total    int                   `json:"total"`
	Worklogs []corejira.RawWorklog `json:"worklogs"`
}

// parseWorklogs reads the worklogs a search carried, and reports whether
// they are all of them. A search caps an issue's worklogs at 20 and still
// states the real total, which is the only way to tell an issue worked on
// for a week from one worked on for a month.
func parseWorklogs(raw json.RawMessage) ([]backend.Worklog, bool) {
	var field worklogField
	if err := json.Unmarshal(raw, &field); err != nil {
		return nil, true
	}
	return mapWorklogs(field.Worklogs), len(field.Worklogs) >= field.Total
}

// mapWorklogs is the wire shape onto TAM's, shared by the search and the
// per-issue read so neither can word an entry differently.
func mapWorklogs(raw []corejira.RawWorklog) []backend.Worklog {
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
	return out
}
