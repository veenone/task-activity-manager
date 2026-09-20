package jira

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"

	"agile-suite/tam/internal/backend"
)

// editwrite.go is the push half of an edit: the journal's text values turned
// into Jira's field shapes, guarded by the screen editscreen.go reads. The
// create half lives in writes.go.

// jiraFields turns the journal's text values into Jira's field shapes. An
// empty priority, assignee, or points clears the field with null.
//
// screen is the set of TAM's names the issue's edit screen carries, and nil
// when Jira could not be asked. A field the screen does not carry is refused
// here, in words, rather than sent: Jira answers such a write with "Field
// cannot be set. It is not on the appropriate screen, or unknown", which
// names an id and leaves the user to guess why TAM sent it at all. The fields
// are walked in name order so two refusals always read the same way.
func jiraFields(fields map[string]string, ids fieldIDs, pointsID string, screen map[string]bool) (map[string]any, error) {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	out := map[string]any{}
	for _, name := range names {
		v := fields[name]
		if label := editFieldLabels[name]; label != "" && screen != nil && !screen[name] {
			return nil, fmt.Errorf(
				"%s is not on the edit screen of this issue in Jira, so TAM will not send it. A Jira administrator has to put the field on that screen; until then the change stays here until you discard it.",
				label)
		}
		switch name {
		case "summary":
			out["summary"] = v
		case "description":
			out["description"] = v
		case "priority":
			out["priority"] = nameOrNull(v)
		case "assignee":
			out["assignee"] = nameOrNull(v)
		case "labels":
			out["labels"] = backend.SplitLabels(v)
		case "storyPoints":
			p, err := backend.ParsePoints(v)
			if err != nil {
				return nil, err
			}
			if p == nil {
				out[pointsID] = nil
			} else {
				out[pointsID] = *p
			}
		case "parentKey":
			if ids.EpicLink == "" {
				return nil, errors.New("this Jira has no Epic Link field, so the epic cannot be pushed")
			}
			if v == "" {
				out[ids.EpicLink] = nil
			} else {
				out[ids.EpicLink] = v
			}
		default:
			return nil, fmt.Errorf("field %q cannot be sent to Jira", name)
		}
	}
	return out, nil
}

func nameOrNull(v string) any {
	if v == "" {
		return nil
	}
	return map[string]string{"name": v}
}

// UpdateIssue PUTs the edited fields. Jira answers 204 on success and a
// 400 with a per-field message otherwise; the client's error carries it.
func (b *Backend) UpdateIssue(ctx context.Context, key string, fields map[string]string) error {
	ids := b.discover(ctx)
	// Resolved only when the edit carries an estimate: an unresolvable
	// points field is no reason to refuse an edit that never mentions it.
	pointsID := ""
	if _, edited := fields["storyPoints"]; edited {
		var err error
		if pointsID, err = b.pointsField(projectOf(key), ids); err != nil {
			return err
		}
	}
	// Read now rather than trusting what the panel drew: the panel's answer
	// is a stored one, and a screen that changed between that read and this
	// Commit is exactly the case this guard is here for.
	//
	// Read only when the edit carries something a screen was ever found to
	// refuse, which is the rule the create path follows for the same reason:
	// a Commit of a hundred summary edits would otherwise pay a hundred
	// GETs. The estimate and the epic are the two fields an instance was
	// seen to leave off a screen, and the two TAM finds by name rather than
	// by a fixed id; the other five are on every edit screen seen in the
	// field. An edit of those five keeps the behaviour it had before this
	// guard existed, with Jira's own refusal as the backstop.
	var screen map[string]bool
	_, editsPoints := fields["storyPoints"]
	_, editsParent := fields["parentKey"]
	if editsPoints || editsParent {
		screen = b.editScreen(ctx, key)
	}
	jf, err := jiraFields(fields, ids, pointsID, screen)
	if err != nil {
		return err
	}
	if err := b.c.Put(ctx, "/rest/api/2/issue/"+url.PathEscape(key), map[string]any{"fields": jf}); err != nil {
		return b.humanizeFieldError(ctx, err, false)
	}
	return nil
}
