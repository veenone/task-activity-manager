package ritualrepo

import (
	"context"
	"fmt"
)

// SeedDemo creates the local ritual documents used by the offline demo. It is
// idempotent and only fills missing documents, so a preview never overwrites a
// user's edits when the profile is opened again.
func (r *Repository) SeedDemo(ctx context.Context, profileID, projectKey string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if projectKey == "" {
		projectKey = "DEMO"
	}
	types := []string{"planning", "standup", "review", "retro"}
	for _, sprint := range []struct {
		id     int
		remark string
	}{
		{12, "Sprint 12 is the reference cycle for the demo walkthrough."},
		{13, "Use this draft to prepare the next sprint ceremony."},
	} {
		for _, typ := range types {
			issueKeys := demoIssueKeys(projectKey, sprint.id, typ)
			issues := make([]Issue, len(issueKeys))
			for i, key := range issueKeys {
				issues[i] = Issue{Key: key}
			}
			issuesJSON, err := EncodeIssues(issues)
			if err != nil {
				return fmt.Errorf("encode demo %s issues: %w", typ, err)
			}
			title := fmt.Sprintf("Sprint %d · %s", sprint.id, ritualTitle(typ))
			// Publication fields (body, confluence_page_id,
			// confluence_version, published_at) stay empty and status stays
			// "draft": publishing is a separate, gated part 2, and a demo
			// preview must never fabricate the state that only a real publish
			// should ever produce.
			_, err = r.db.ExecContext(ctx, `
				INSERT OR IGNORE INTO ritual_document (
					profile_id, board_id, sprint_id, ritual_type, title, remark, body,
					issues_json, confluence_page_id, confluence_version, status,
					updated_at, published_at
				) VALUES (?, 1, ?, ?, ?, ?, '', ?, '', 0, 'draft', ?, '')`,
				profileID, sprint.id, typ, title, sprint.remark, issuesJSON,
				"2026-09-13T09:00:00Z",
			)
			if err != nil {
				return fmt.Errorf("seed demo %s sprint %d: %w", typ, sprint.id, err)
			}
		}
	}
	return nil
}

func ritualTitle(typ string) string {
	switch typ {
	case "planning":
		return "Planning"
	case "standup":
		return "Standup"
	case "review":
		return "Review"
	case "retro":
		return "Retrospective"
	default:
		return typ
	}
}

func demoIssueKeys(projectKey string, sprint int, typ string) []string {
	switch sprint {
	case 12:
		return []string{projectKey + "-412", projectKey + "-409", projectKey + "-414"}
	default:
		return []string{projectKey + "-398", projectKey + "-390"}
	}
}
