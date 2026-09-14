// Package ritualdefaults decides which of a sprint's issues each ritual starts
// with, so the same four documents appear each cycle without anyone choosing.
package ritualdefaults

import (
	"sort"
	"strings"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/ritualrepo"
)

// Types is the four rituals a sprint gets, in the order they happen.
var Types = []string{"planning", "standup", "review", "retro"}

// Selected is one chosen issue. It is ritualrepo.Issue under another name so
// this package can be read without knowing where drafts are stored.
type Selected = ritualrepo.Issue

// labels name each ritual in a page title.
var labels = map[string]string{
	"planning": "Planning",
	"standup":  "Standup",
	"review":   "Review",
	"retro":    "Retrospective",
}

// Select returns the issues a ritual opens with, in the order they should be
// published. An unknown ritual type selects nothing rather than guessing.
//
// The rules are deliberately derivable from a cached issue alone. Nothing here
// reads a sprint report, so a scaffold works before a report has ever been
// built.
func Select(ritualType string, issues []backend.Issue) []Selected {
	switch strings.TrimSpace(strings.ToLower(ritualType)) {
	case "planning":
		// Everything in the sprint, with unestimated work first, because the
		// unestimated issues are what planning is for.
		return order(issues, func(a, b backend.Issue) bool {
			ae, be := a.StoryPoints == nil, b.StoryPoints == nil
			if ae != be {
				return ae
			}
			return false
		})
	case "standup":
		return pick(issues, func(i backend.Issue) bool {
			return !backend.IsDone(i.Status) && inFlight(i.Status)
		})
	case "review":
		return pick(issues, func(i backend.Issue) bool { return backend.IsDone(i.Status) })
	case "retro":
		// Unfinished work first: that is what a retrospective opens on.
		return order(issues, func(a, b backend.Issue) bool {
			ad, bd := backend.IsDone(a.Status), backend.IsDone(b.Status)
			if ad != bd {
				return !ad
			}
			return false
		})
	default:
		return []Selected{}
	}
}

// Title names a ritual's page.
func Title(ritualType, sprintName string) string {
	label, ok := labels[strings.TrimSpace(strings.ToLower(ritualType))]
	if !ok {
		return strings.TrimSpace(sprintName)
	}
	name := strings.TrimSpace(sprintName)
	if name == "" {
		return label
	}
	return name + " " + label
}

// inFlight is true for work somebody has started or is stuck on. It matches on
// the status name, the same shape backend.IsDone uses, because a cached issue
// carries the name and not the board's column.
func inFlight(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return strings.Contains(s, "progress") || strings.Contains(s, "block") ||
		strings.Contains(s, "review") || strings.Contains(s, "testing")
}

func pick(issues []backend.Issue, keep func(backend.Issue) bool) []Selected {
	out := []Selected{}
	for _, i := range issues {
		if keep(i) {
			out = append(out, Selected{Key: i.Key})
		}
	}
	return out
}

// order keeps every issue, sorted by less, preserving the incoming order within
// a group. SliceStable matters: the incoming order is the board's rank, and two
// issues that tie must not swap between runs or the render stops being
// deterministic.
func order(issues []backend.Issue, less func(a, b backend.Issue) bool) []Selected {
	sorted := make([]backend.Issue, len(issues))
	copy(sorted, issues)
	sort.SliceStable(sorted, func(i, j int) bool { return less(sorted[i], sorted[j]) })
	return pick(sorted, func(backend.Issue) bool { return true })
}
