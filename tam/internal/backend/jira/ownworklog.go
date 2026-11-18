package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agile-suite/tam/internal/backend"
)

var _ backend.OwnWorklogBackend = (*Backend)(nil)

// ownWorklogPage is how many issues one search for the reader's work asks for.
const ownWorklogPage = 50

// WorklogsBy reads what the connected user logged on the days asked for.
//
// The search finds the issues and the entries are then narrowed here. Jira
// reads worklogDate on the zone of the user's Jira profile, which need not be
// the zone the entries were logged in, so the search asks for a day either
// side and the range is applied to each entry's own started stamp. A search
// also returns every worklog on a matching issue, other people's included,
// which is why author is compared again entry by entry.
//
// An issue whose worklogs the search cut short is read whole. Skipping it
// would show a day as short when the missing hours were logged.
func (b *Backend) WorklogsBy(ctx context.Context, author, fromDay, toDay string) ([]backend.LoggedWork, error) {
	from, err := time.Parse(backend.DayLayout, fromDay)
	if err != nil {
		return nil, fmt.Errorf("%q is not a day", fromDay)
	}
	to, err := time.Parse(backend.DayLayout, toDay)
	if err != nil {
		return nil, fmt.Errorf("%q is not a day", toDay)
	}
	jql := fmt.Sprintf(`worklogAuthor = currentUser() AND worklogDate >= "%s" AND worklogDate <= "%s"`,
		from.AddDate(0, 0, -1).Format(backend.DayLayout), to.AddDate(0, 0, 1).Format(backend.DayLayout))

	out := []backend.LoggedWork{}
	for startAt := 0; ; {
		page, err := b.c.SearchIssues(ctx, jql, []string{"summary", fieldWorklog}, nil, startAt, ownWorklogPage)
		if err != nil {
			return nil, fmt.Errorf("read your worklogs: %w", err)
		}
		for _, raw := range page.Issues {
			logs, whole := parseWorklogs(raw.Fields[fieldWorklog])
			if !whole {
				if logs, err = b.Worklogs(ctx, raw.Key); err != nil {
					return nil, fmt.Errorf("read the worklogs of %s: %w", raw.Key, err)
				}
			}
			var summary string
			_ = json.Unmarshal(raw.Fields["summary"], &summary)
			for _, l := range logs {
				if strings.EqualFold(l.Author, author) && backend.OnDays(l.Started, fromDay, toDay) {
					out = append(out, backend.LoggedWork{Key: raw.Key, Summary: summary, Log: l})
				}
			}
		}
		startAt += len(page.Issues)
		if len(page.Issues) == 0 || startAt >= page.Total {
			return out, nil
		}
	}
}
