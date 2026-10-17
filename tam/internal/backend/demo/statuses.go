package demo

import (
	"context"
	"sort"

	"agile-suite/tam/internal/backend"
)

var _ backend.StatusBackend = (*Backend)(nil)

// demoStatusCategories is the bucket of each status in demoStatusIDs.
var demoStatusCategories = map[string]string{
	"To Do":       "new",
	"Draft":       "new",
	"In Progress": "indeterminate",
	"Done":        "done",
	"Approved":    "done",
}

// Statuses is the demo's statuses with their categories, in id order.
func (b *Backend) Statuses(_ context.Context) ([]backend.Status, error) {
	out := make([]backend.Status, 0, len(demoStatusIDs))
	for name, id := range demoStatusIDs {
		out = append(out, backend.Status{ID: id, Name: name, Category: demoStatusCategories[name]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
