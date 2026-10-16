package jira

import (
	"context"

	"agile-suite/tam/internal/backend"
)

var _ backend.StatusBackend = (*Backend)(nil)

// Statuses reads every status on the instance with its category.
func (b *Backend) Statuses(ctx context.Context) ([]backend.Status, error) {
	var raw []struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		StatusCategory struct {
			Key string `json:"key"`
		} `json:"statusCategory"`
	}
	if err := b.c.Get(ctx, "/rest/api/2/status", &raw); err != nil {
		return nil, err
	}
	out := make([]backend.Status, 0, len(raw))
	for _, s := range raw {
		st := backend.Status{ID: s.ID, Name: s.Name}
		// I1: only the three keys Jira defines are kept, the rule parseIssue
		// applies to an issue's own category.
		switch s.StatusCategory.Key {
		case "new", "indeterminate", "done":
			st.Category = s.StatusCategory.Key
		}
		out = append(out, st)
	}
	return out, nil
}
