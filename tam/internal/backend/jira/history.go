package jira

import (
	"context"
	"strings"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
)

// changelogExpand is what SearchIssuesWithHistory asks Jira to expand,
// beside the fields the row itself is built from. The sync path's own
// search passes nil instead: expand=changelog costs no second request, it
// makes the one search response carry every row's full history, which
// Jira has to assemble at real cost and which a sync never reads, so
// asking for it there would only make every page slower for nothing.
var changelogExpand = []string{"changelog"}

// Nothing calls this method outside a test yet; its future caller reaches
// it through a type assertion, so drift here would silently fail the
// report open to a real Jira before that caller exists to notice. This
// line fails the build instead.
var _ backend.HistoryBackend = (*Backend)(nil)

// SearchIssuesWithHistory runs jql with the changelog expanded and maps
// each hit to its row plus its changes, normalised to the logical field
// names other backends and the rest of TAM use for the same fields.
func (b *Backend) SearchIssuesWithHistory(ctx context.Context, jql string, startAt, maxResults int) ([]backend.IssueHistory, int, error) {
	ids := b.discover(ctx)
	fields := append(append([]string{}, baseFields...), ids.list()...)
	page, err := b.c.SearchIssues(ctx, jql, fields, changelogExpand, startAt, maxResults)
	if err != nil {
		return nil, 0, err
	}
	// Each hit resolves its own project's types rather than assuming a
	// single call's worth: a jql spanning more than one project (a
	// programme-wide scope JQL, for instance) still needs each hit's own
	// instance-specific task and sub-task names. resolveTypes already
	// caches per project on the backend itself, so a jql that stays inside
	// one project costs nothing extra here, and a second cache in front of
	// it would only be duplicate bookkeeping.
	out := make([]backend.IssueHistory, 0, len(page.Issues))
	for _, raw := range page.Issues {
		pt := b.typesOrEmpty(ctx, projectOf(raw.Key))
		out = append(out, backend.IssueHistory{
			Issue:     parseIssue(raw, ids, b.requirementType, pt),
			Changes:   normalizeChanges(raw.Changelog.Histories, ids),
			Truncated: raw.Changelog.Total > len(raw.Changelog.Histories),
		})
	}
	return out, page.Total, nil
}

// normalizeChanges flattens a raw changelog into the changes a sprint
// report can read, dropping any item logicalChangeField does not
// recognise: a report built on named fields cannot do anything with a
// field it cannot name, and carrying it along unlabelled would only invite
// a caller to guess what it was.
func normalizeChanges(histories []corejira.RawHistory, ids fieldIDs) []backend.Change {
	var out []backend.Change
	for _, h := range histories {
		for _, item := range h.Items {
			field := logicalChangeField(item, ids)
			if field == "" {
				continue
			}
			out = append(out, backend.Change{
				At:    h.Created,
				Field: field,
				From:  stringOrRaw(item.FromString, item.From),
				To:    stringOrRaw(item.ToString, item.To),
			})
		}
	}
	return out
}

// stringOrRaw is the readable half of a changelog item's from/to pair,
// falling back to the raw half when Jira left the readable one empty: core's
// RawHistoryItem keeps both because which pair a field populates depends on
// the field, and a caller that only ever read the string pair would see two
// empty values on a field that only sent the raw one, indistinguishable from
// a change that genuinely carried no value on that side.
func stringOrRaw(readable, raw string) string {
	if readable != "" {
		return readable
	}
	return raw
}

// logicalChangeField maps one changelog item's raw field identity to the
// logical name the rest of TAM uses for it, or "" for a field this task's
// normaliser does not read.
//
// status carries the stable field id "status" on every Jira instance, so
// it is matched on the id directly. Sprint and Story Points are custom
// fields: their ids are customfield_NNNNN and differ per instance, the
// same reason fields.go resolves them once per instance instead of
// hardcoding them, so they are matched against the ids this backend has
// already discovered for the instance it is talking to. The display name
// is a fallback for the two custom fields only, and only when discovery
// could not resolve an id (the instance renamed the field, or this
// backend's field lookup failed): a normaliser keyed on literal
// customfield_NNNNN ids copied from one instance would still match status,
// whose id really is a stable literal, and would never match sprint or
// story points on any instance whose discovered ids differ from those
// literals, which is every instance but the one they were copied from.
func logicalChangeField(item corejira.RawHistoryItem, ids fieldIDs) string {
	switch {
	case item.FieldID == "status":
		return "status"
	case ids.Sprint != "" && item.FieldID == ids.Sprint:
		return "sprint"
	case ids.Points != "" && item.FieldID == ids.Points:
		return "storyPoints"
	case ids.Sprint == "" && strings.EqualFold(item.Field, "Sprint"):
		return "sprint"
	case ids.Points == "" && strings.EqualFold(item.Field, "Story Points"):
		return "storyPoints"
	default:
		return ""
	}
}
