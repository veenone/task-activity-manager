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
	Offset   int      `json:"offset"`
	Limit    int      `json:"limit"`
	// Sort names the column the grid is ordered by: one of the keys in
	// sortColumns, or "" for the default rank order. Desc reverses it.
	// Sorting happens here rather than in the frontend because the grid is
	// paged: the frontend only ever holds one page, so a client-side sort
	// would order 25 rows out of a project's thousands.
	Sort string `json:"sort"`
	Desc bool   `json:"desc"`
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
