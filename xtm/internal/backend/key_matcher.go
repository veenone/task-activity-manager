package backend

import "context"

// KeyMatcher narrows a list of issue keys to those that also match a JQL
// query. Only a Jira-side backend has JQL, so it is a side interface rather
// than another method every adapter would have to refuse, the same shape as
// RunStepResultManager. Callers type-assert, and treat a backend without it
// as having no JQL (#446).
type KeyMatcher interface {
	KeysMatching(ctx context.Context, keys []string, jql string) ([]string, error)
}
