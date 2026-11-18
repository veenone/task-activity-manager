package demo

import (
	"context"
	"sort"
	"strings"

	"agile-suite/tam/internal/backend"
)

var _ backend.OwnWorklogBackend = (*Backend)(nil)

// WorklogsBy answers from the same entries Worklogs reads: the sample's and
// whatever this run pushed. Issues are walked in key order so the answer does
// not change from one call to the next.
func (b *Backend) WorklogsBy(_ context.Context, author, fromDay, toDay string) ([]backend.LoggedWork, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	keys := make([]string, 0, len(b.worklogs))
	for k := range b.worklogs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []backend.LoggedWork{}
	for _, k := range keys {
		iss, _ := b.find(k)
		for _, l := range b.worklogs[k] {
			if strings.EqualFold(l.Author, author) && backend.OnDays(l.Started, fromDay, toDay) {
				out = append(out, backend.LoggedWork{Key: k, Summary: iss.Summary, Log: l})
			}
		}
	}
	return out, nil
}
