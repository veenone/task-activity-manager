package boardrepo

import "agile-suite/tam/internal/backend"

// Subtask drafts have no Jira membership yet. Their parent's membership is
// authoritative, so they only appear in that parent's board/sprint scope.
func withSubtaskDrafts(cards, drafts []backend.Issue) []backend.Issue {
	parents := map[string]backend.Issue{}
	seen := map[string]bool{}
	for _, c := range cards {
		seen[c.Key] = true
		if c.Type != backend.TypeSubtask && c.Type != backend.TypeEpic {
			parents[c.Key] = c
		}
	}
	for _, d := range drafts {
		if d.Type != backend.TypeSubtask || seen[d.Key] {
			continue
		}
		if parent, ok := parents[d.ParentKey]; ok {
			d.SprintID, d.SprintName = parent.SprintID, parent.SprintName
			cards = append(cards, d)
			seen[d.Key] = true
		}
	}
	return cards
}
