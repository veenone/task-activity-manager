package sprints

import (
	"errors"
	"strings"

	"agile-suite/tam/internal/backend"
)

// errDraftSprint is what every write to a real sprint answers for a draft
// sprint's negative id. The id means nothing to Jira, and the bound methods are
// reachable without the menus that disable these actions for a draft.
var errDraftSprint = errors.New("this sprint is a draft in TAM; Commit creates it in Jira first")

// DraftSprint checks a sprint about to be drafted and converts its dates the
// way a start or an edit converts them: a name is required, and a date that
// is not a date or an end before its start is refused here, where the
// message can name it. It is a package function and not a Service method
// because it writes nothing and reaches nothing: the draft is journaled by
// issuerepo, and Commit creates it.
func DraftSprint(d backend.SprintDraft) (backend.SprintDraft, error) {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		return d, errors.New("a sprint needs a name")
	}
	var err error
	if d.StartDate, d.EndDate, err = dates(d.StartDate, d.EndDate); err != nil {
		return d, err
	}
	return d, nil
}
