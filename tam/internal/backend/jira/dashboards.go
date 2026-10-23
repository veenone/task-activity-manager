package jira

import (
	"context"

	"agile-suite/tam/internal/backend"
)

// JiraDashboards is Jira's own dashboards, as a name and the URL that opens
// each one.
//
// That is all a signpost needs, and all Data Center will give. Reading a
// dashboard's gadgets needs /dashboard/{id}/gadget, which is Cloud only,
// and the property routes that would read the filter behind a gadget need
// an item id that only that route hands out. Deriving a TAM dashboard from
// a Jira one is out of scope on the API rather than on effort (#349).
func (b *Backend) JiraDashboards(ctx context.Context) ([]backend.JiraDashboard, error) {
	boards, err := b.c.Dashboards(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]backend.JiraDashboard, 0, len(boards))
	for _, d := range boards {
		out = append(out, backend.JiraDashboard{ID: d.ID, Name: d.Name, URL: d.View})
	}
	return out, nil
}
