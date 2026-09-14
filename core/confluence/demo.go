package confluence

import "strings"

// DemoPages returns the deterministic, offline ritual documents used by TAM's
// demo profiles. The project key is included in titles so demo variants remain
// recognizable without contacting Confluence.
func DemoPages(projectKey string) ([]Page, []ChildPage) {
	key := strings.ToUpper(strings.TrimSpace(projectKey))
	if key == "" {
		key = "DEMO"
	}
	pages := []Page{
		{ID: "demo-ritual-planning", Title: key + " Sprint planning", Space: PageSpace{Key: "DEMO"}, Body: PageBody{View: PageView{Value: "<h1>Sprint planning</h1><p>Confirm the sprint goal, capacity, and top priorities.</p><h3>Related Jira issues</h3><p>" + key + "-412 · " + key + "-409 · " + key + "-414</p><p><strong>Remark:</strong> Keep scope clear and call out risks before work starts.</p>"}}},
		{ID: "demo-ritual-standup", Title: key + " Daily standup", Space: PageSpace{Key: "DEMO"}, Body: PageBody{View: PageView{Value: "<h1>Daily standup</h1><p>Share progress, plans, and blockers. Keep the conversation focused.</p><h3>Related Jira issues</h3><p>" + key + "-412 · " + key + "-409 · " + key + "-414</p><p><strong>Remark:</strong> Ask for help early when an issue is blocked.</p>"}}},
		{ID: "demo-ritual-review", Title: key + " Sprint review", Space: PageSpace{Key: "DEMO"}, Body: PageBody{View: PageView{Value: "<h1>Sprint review</h1><p>Demonstrate completed work and capture stakeholder feedback.</p><h3>Related Jira issues</h3><p>" + key + "-398 · " + key + "-390</p><p><strong>Remark:</strong> Turn feedback into a named follow-up before closing.</p>"}}},
		{ID: "demo-ritual-retro", Title: key + " Retrospective", Space: PageSpace{Key: "DEMO"}, Body: PageBody{View: PageView{Value: "<h1>Retrospective</h1><p>Keep, improve, and try: record one practical action for the next sprint.</p><h3>Related Jira issues</h3><p>" + key + "-398 · " + key + "-390</p><p><strong>Remark:</strong> Leave with one small action and an owner.</p>"}}},
	}
	children := make([]ChildPage, 0, len(pages))
	for _, page := range pages {
		children = append(children, ChildPage{ID: page.ID, Title: page.Title, Type: "page", Status: "current"})
	}
	return pages, children
}
