// Package issuerepo is the store layer over tam.db for issues: the cached
// rows the Backlog reads, the per-issue detail cache, issue links, sync
// state, and per-profile settings. Every method takes the profile id
// because every table is scoped by it.
package issuerepo

import (
	"database/sql"
	"errors"

	"agile-suite/tam/internal/backend"
)

// ErrNotFound is returned when a key is not cached for the profile.
var ErrNotFound = errors.New("issuerepo: not found")

// Repository runs the queries. It holds no state beyond the handle.
type Repository struct {
	db *sql.DB
}

// New wraps an open tam.db handle.
func New(db *sql.DB) *Repository { return &Repository{db: db} }

// IssueQuery is the Backlog's filter and page. Text matches key, summary,
// and labels; the match is case-insensitive for ASCII only, since it runs
// through SQLite's default LIKE, which does not fold non-ASCII case. An
// empty Types list means every type. Limit defaults to 25 and is capped
// at 500.
type IssueQuery struct {
	Text     string   `json:"text"`
	Types    []string `json:"types"`
	SprintID string   `json:"sprintId"`
	// Statuses and Assignees are the filter bar's own two lists: empty
	// means the field is not filtered, and several entries are an OR
	// within the field while the fields themselves are an AND. Both
	// compare the names the sync stored, which is what the grid shows and
	// what ListFilterOptions offers.
	//
	// Assignees holds display names, not usernames, and so is a different
	// filter from AssigneeName below: that one is the "assigned to me" tab
	// matching the connected user, this one is a name the user picked off a
	// list. An empty entry here means unassigned, which is why the field is
	// a list rather than a string that would read as no filter at all.
	Statuses  []string `json:"statuses"`
	Assignees []string `json:"assignees"`
	Offset   int      `json:"offset"`
	Limit    int      `json:"limit"`
	// GroupSubtasks pages parent issues and includes their subtasks directly
	// after them. Total, Limit and Offset then count parent groups.
	GroupSubtasks bool `json:"groupSubtasks"`
	// Sort names the column the grid is ordered by: one of the keys in
	// sortColumns, or "" for the default rank order. Desc reverses it.
	// Sorting happens here rather than in the frontend because the grid is
	// paged: the frontend only ever holds one page, so a client-side sort
	// would order 25 rows out of a project's thousands.
	Sort string `json:"sort"`
	Desc bool   `json:"desc"`
	// AssigneeName filters to issues assigned to this Jira username,
	// matched against the assignee_name column, case-insensitively. Empty
	// means no assignee filter. AssigneeDisplayName is the display name
	// that goes with it, used only as a fallback for rows cached before
	// schema 14, whose assignee_name is still empty; the caller supplies
	// it (the connected user's display name), since this repository does
	// no settings lookups of its own.
	AssigneeName        string `json:"assigneeName"`
	AssigneeDisplayName string `json:"assigneeDisplayName"`
}

// IssuePage is one page of rows plus the total the filter matches.
type IssuePage struct {
	Issues []backend.Issue `json:"issues"`
	Total  int             `json:"total"`
}

// SprintRef is a sprint seen in the cached issues, for the filter dropdown.
type SprintRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
