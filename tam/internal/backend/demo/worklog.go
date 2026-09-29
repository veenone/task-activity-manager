package demo

import (
	"context"
	"fmt"
	"strconv"

	"agile-suite/tam/internal/backend"
)

// Worklogs answers with the entries this run pushed and nothing else. The
// dataset seeds none, because a worklog is only ever the record of work
// somebody did and a seeded one would be state no operation produced.
func (b *Backend) Worklogs(_ context.Context, key string) ([]backend.Worklog, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.find(key); !ok {
		return nil, fmt.Errorf("demo: no issue %s", key)
	}
	return append([]backend.Worklog{}, b.worklogs[key]...), nil
}

// AddWorklog records the entry so the section shows it from now on, the way
// CreateLink records a link.
func (b *Backend) AddWorklog(_ context.Context, key string, d backend.WorklogDraft) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.find(key); !ok {
		return fmt.Errorf("demo: no issue %s", key)
	}
	b.worklogs[key] = append(b.worklogs[key], backend.Worklog{
		ID:         strconv.Itoa(len(b.worklogs[key]) + 1),
		Author:     "demo",
		AuthorName: "Demo user",
		Started:    d.Started,
		TimeSpent:  d.TimeSpent,
		Seconds:    d.Seconds,
		Comment:    d.Comment,
	})
	return nil
}
