package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

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

var _ backend.FamilyWorklogBackend = (*Backend)(nil)

// subtaskPage is how many parents one search asks about. The JQL names
// them all, and a very long list is what makes Jira refuse a query
// outright, so the parents are asked for in batches.
const subtaskPage = 50

// SubtaskWorklogs reads what a parent's sub-tasks logged, in one search
// per batch of parents rather than one per issue.
//
// It asks for the parent field beside the worklog so each entry can be
// filed under the issue that owns it. An issue whose worklogs the search
// cut short is read again on its own, the same rule the history search
// follows: a search caps the field at 20 entries and still reports the
// real total.
func (b *Backend) SubtaskWorklogs(ctx context.Context, parentKeys []string) (map[string][]backend.Worklog, error) {
	out := map[string][]backend.Worklog{}
	for start := 0; start < len(parentKeys); start += subtaskPage {
		end := start + subtaskPage
		if end > len(parentKeys) {
			end = len(parentKeys)
		}
		if err := b.subtaskWorklogBatch(ctx, parentKeys[start:end], out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (b *Backend) subtaskWorklogBatch(ctx context.Context, keys []string, into map[string][]backend.Worklog) error {
	quoted := make([]string, 0, len(keys))
	for _, k := range keys {
		quoted = append(quoted, `"`+k+`"`)
	}
	jql := "parent in (" + strings.Join(quoted, ", ") + ")"
	for startAt := 0; ; {
		page, err := b.c.SearchIssues(ctx, jql, []string{"parent", fieldWorklog}, nil, startAt, subtaskPage)
		if err != nil {
			return fmt.Errorf("read the sub-task worklogs: %w", err)
		}
		for _, raw := range page.Issues {
			var parent keyed
			if err := json.Unmarshal(raw.Fields["parent"], &parent); err != nil || parent.Key == "" {
				continue
			}
			logs, whole := parseWorklogs(raw.Fields[fieldWorklog])
			if !whole {
				if full, err := b.Worklogs(ctx, raw.Key); err == nil {
					logs = full
				} else {
					log.Printf("tam: %s has more worklogs than the search returned, so its parent's burndown is short by the rest: %v", raw.Key, err)
				}
			}
			if len(logs) > 0 {
				into[parent.Key] = append(into[parent.Key], logs...)
			}
		}
		startAt += len(page.Issues)
		if len(page.Issues) == 0 || startAt >= page.Total {
			return nil
		}
	}
}
