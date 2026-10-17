package backend

import "context"

// Status is one of the instance's statuses with the bucket Jira files it
// under: "new", "indeterminate" or "done", the same three keys everywhere.
// Category is empty for a key Jira does not define.
type Status struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

// StatusBackend lists the instance's statuses, reached by a type assertion
// like HistoryBackend. A changelog entry names a status but not its
// category, so the sprint report's People section reads this to know which
// stretches of a card's history were in progress.
type StatusBackend interface {
	Statuses(ctx context.Context) ([]Status, error)
}
