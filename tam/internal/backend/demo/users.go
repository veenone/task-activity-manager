package demo

import (
	"context"
	"sort"
	"strings"

	"agile-suite/tam/internal/backend"
)

// demoUsers is the Acme Platform team, the names the curated dataset already
// assigns issues to plus a few more so the picker has something to filter.
var demoUsers = []backend.User{
	{Name: "ranand", DisplayName: "R. Anand"},
	{Name: "skim", DisplayName: "S. Kim"},
	{Name: "po", DisplayName: "Product Owner"},
	{Name: "mortiz", DisplayName: "M. Ortiz"},
	{Name: "jdoe", DisplayName: "J. Doe"},
	{Name: "lchen", DisplayName: "L. Chen"},
	{Name: "araghu", DisplayName: "A. Raghunathan"},
	{Name: "bnovak", DisplayName: "B. Novak"},
}

// demoPriorities are Jira's own defaults, in Jira's order.
var demoPriorities = []string{"Highest", "High", "Medium", "Low", "Lowest"}

// SearchUsers filters the demo team by username or display name, the same
// substring match the cached search does, so the offline dataset behaves like
// the live one.
func (b *Backend) SearchUsers(_ context.Context, _ string, query string) ([]backend.User, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	out := []backend.User{}
	for _, u := range demoUsers {
		if q == "" || q == "%" ||
			strings.Contains(strings.ToLower(u.Name), q) ||
			strings.Contains(strings.ToLower(u.DisplayName), q) {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DisplayName < out[j].DisplayName })
	return out, nil
}

// Priorities answers with Jira's defaults so the demo profile's forms offer a
// list rather than falling back to free text.
func (b *Backend) Priorities(context.Context) ([]string, error) {
	return append([]string{}, demoPriorities...), nil
}

// DemoSubtaskType is what the offline dataset calls its sub-task level. It is
// deliberately not "Sub-task": the name is per-instance, and a demo that used
// the default would hide the fact that TAM discovers it.
const DemoSubtaskType = "Technical task"

// SubtaskTypeName is the demo project's sub-task type.
func (b *Backend) SubtaskTypeName(context.Context, string) (string, error) {
	return DemoSubtaskType, nil
}
