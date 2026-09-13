package ritualrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
		status string
		remark string
	}{
		{12, "published", "Sprint 12 is the reference cycle for the demo walkthrough."},
		{13, "draft", "Use this draft to prepare the next sprint ceremony."},
	} {
		for _, typ := range types {
			keys, err := json.Marshal(demoIssueKeys(projectKey, sprint.id, typ))
			if err != nil {
				return fmt.Errorf("encode demo %s issues: %w", typ, err)
			}
			title := fmt.Sprintf("Sprint %d · %s", sprint.id, ritualTitle(typ))
			body := demoBody(projectKey, sprint.id, typ)
			_, err = r.db.ExecContext(ctx, `
				INSERT OR IGNORE INTO ritual_document (
					profile_id, board_id, sprint_id, ritual_type, title, remark, body,
					issue_keys_json, confluence_page_id, confluence_version, status,
					updated_at, published_at
				) VALUES (?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				profileID, sprint.id, typ, title, sprint.remark, body, string(keys),
				fmt.Sprintf("demo-ritual-%s-%d", typ, sprint.id), 1, sprint.status,
				"2026-09-13T09:00:00Z", map[string]string{"published": "2026-09-12T16:00:00Z"}[sprint.status],
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

func demoBody(projectKey string, sprint int, typ string) string {
	return fmt.Sprintf("<h2>%s · Sprint %d</h2><p>Use this workspace to capture the team's notes, decisions, and follow-ups.</p><h3>Checklist</h3><ul><li>Review the sprint goal and current progress.</li><li>Record decisions and owners while the conversation is fresh.</li><li>Link follow-up work before closing the ritual.</li></ul><h3>Related Jira issues</h3><p>%s</p>", ritualTitle(typ), sprint, joinIssueLinks(projectKey, sprint, typ))
}

func joinIssueLinks(projectKey string, sprint int, typ string) string {
	keys := demoIssueKeys(projectKey, sprint, typ)
	return strings.Join(keys, ", ")
}
