package componentadmin

import (
	"context"
	"fmt"
	"strings"

	"agile-suite/xtm/internal/backend"
)

// BatchRow is one row of a batch create as Check sees it: the cleaned input,
// and why it cannot be created, or "" when it can.
type BatchRow struct {
	Input   backend.ComponentInput `json:"input"`
	Problem string                 `json:"problem"`
}

// BatchFailure is a row CreateMany did not create. Row counts from 1.
type BatchFailure struct {
	Row   int    `json:"row"`
	Name  string `json:"name"`
	Error string `json:"error"`
}

// BatchResult is what a batch create did.
type BatchResult struct {
	Created []backend.Component `json:"created"`
	Failed  []BatchFailure      `json:"failed"`
}

// assigneeTypes are the default assignee values Jira accepts, in the order the
// form lists them.
var assigneeTypes = []string{"PROJECT_DEFAULT", "COMPONENT_LEAD", "PROJECT_LEAD", "UNASSIGNED"}

// Check cleans each row and says which cannot be created: a blank name, a
// default assignee Jira does not know, a name the project already has, or a
// name an earlier row already creates. Names compare case-insensitively,
// because Jira refuses two components that differ only in case.
func (s *Service) Check(ctx context.Context, rows []backend.ComponentInput) ([]BatchRow, error) {
	list, err := s.b.ProjectComponentDetails(ctx, s.projectKey)
	if err != nil {
		return nil, translate(err)
	}
	existing := map[string]string{}
	for _, c := range list {
		existing[strings.ToLower(c.Name)] = c.Name
	}
	seen := map[string]string{}
	out := make([]BatchRow, 0, len(rows))
	for _, in := range rows {
		in, err := clean(in)
		row := BatchRow{Input: in}
		key := strings.ToLower(in.Name)
		switch {
		case err != nil:
			row.Problem = err.Error()
		case !validAssignee(in.AssigneeType):
			row.Problem = "Default assignee must be Project default, Component lead, Project lead or Unassigned."
		case existing[key] != "":
			row.Problem = fmt.Sprintf("The project already has a component named %q.", existing[key])
		case seen[key] != "":
			row.Problem = fmt.Sprintf("Another row also creates %q.", seen[key])
		}
		if key != "" && seen[key] == "" {
			seen[key] = in.Name
		}
		out = append(out, row)
	}
	return out, nil
}

// CreateMany creates every row Check passes, one Jira call each, and carries
// on past a row Jira refuses. The cached component list is refreshed once at
// the end.
func (s *Service) CreateMany(ctx context.Context, rows []backend.ComponentInput) (BatchResult, error) {
	res := BatchResult{Created: []backend.Component{}, Failed: []BatchFailure{}}
	checked, err := s.Check(ctx, rows)
	if err != nil {
		return res, err
	}
	for i, row := range checked {
		problem := row.Problem
		if problem == "" {
			c, err := s.b.CreateComponent(ctx, s.projectKey, row.Input)
			if err == nil {
				res.Created = append(res.Created, c)
				continue
			}
			problem = translate(err).Error()
		}
		res.Failed = append(res.Failed, BatchFailure{Row: i + 1, Name: row.Input.Name, Error: problem})
	}
	return res, s.refresh(ctx)
}

func validAssignee(t string) bool {
	for _, v := range assigneeTypes {
		if t == v {
			return true
		}
	}
	return false
}
