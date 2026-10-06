package issuerepo

import (
	"context"
	"sort"
	"strings"

	"agile-suite/tam/internal/backend"
)

// What a draft's type may be.
//
// Three things used to disagree about this and only one of them refused.
// The New issue dialog offers every type the project has, under the
// project's own name for it, which is the rule #67 set; the Jira backend
// accepts such a name on a create, because the project's type list has it;
// and this gate checked a hardcoded map of TAM's six logical types, so a
// type like Improvement was offered and then refused on save (#137).
//
// So the gate asks the project. A type is draftable when it is one of
// TAM's six, which every Jira has something answering to, or a name the
// profile's own synced type list carries. There is no setting: a list of
// type names kept by hand is a second answer to a question Jira already
// answers, and it is wrong from the first time somebody adds a type in
// Jira until somebody else notices here.

// draftable is the set of type names one profile may draft, and the words
// a refusal lists when something else arrives.
type draftable struct {
	// own is what the project itself offers, empty for a profile no sync
	// has recorded types for.
	own []backend.IssueType
}

// draftableTypes reads the profile's recorded project types. A profile
// that has never synced has none, and the six logical types are then the
// whole answer, which is also what the dialog offers there.
func (r *Repository) draftableTypes(ctx context.Context, profileID string) (draftable, error) {
	own, err := r.ProjectTypes(ctx, profileID)
	if err != nil {
		return draftable{}, err
	}
	return draftable{own: own}, nil
}

// has reports whether a draft may carry this type. The comparison on the
// project's own names is case-insensitive, because the name travels
// through a spreadsheet column on the import path and "improvement" is
// the same type there as "Improvement".
func (d draftable) has(name string) bool {
	if draftTypes[name] {
		return true
	}
	for _, t := range d.own {
		if strings.EqualFold(strings.TrimSpace(name), t.Name) {
			return true
		}
	}
	return false
}

// words is what a refusal lists: the project's own type names when a sync
// has recorded them, since those are what this project actually takes, and
// TAM's six otherwise.
func (d draftable) words() string {
	if len(d.own) == 0 {
		return "tasks, stories, bugs, requirements, epics, and subtasks"
	}
	names := make([]string, 0, len(d.own))
	for _, t := range d.own {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
