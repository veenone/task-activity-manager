// Package ritualrepo stores locally editable ritual documents in tam.db.
package ritualrepo

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// Issue is one Jira issue chosen for a ritual, with the remark the team made
// about it. The slice order is the order the published table uses, so it is
// preserved exactly as stored and never re-sorted from a query result.
type Issue struct {
	Key    string `json:"key"`
	Remark string `json:"remark"`
}

// DecodeIssues reads stored issues. An empty column decodes as an empty slice.
func DecodeIssues(encoded string) ([]Issue, error) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return []Issue{}, nil
	}
	var issues []Issue
	if err := json.Unmarshal([]byte(trimmed), &issues); err != nil {
		return nil, fmt.Errorf("decode ritual issues: %w", err)
	}
	if issues == nil {
		issues = []Issue{}
	}
	return issues, nil
}

// Repository runs ritual document queries against an open tam.db handle.
type Repository struct {
	db *sql.DB
}

// New wraps an open tam.db handle.
func New(db *sql.DB) *Repository { return &Repository{db: db} }

// normalizeRitualType matches ritual_type's case-insensitive treatment
// everywhere in this package: the column carries no COLLATE NOCASE, so a
// caller passing "Review" against a stored "review" row would otherwise
// match nothing, and a write that follows such a miss would silently blank
// a real row's publication fields instead of updating it. Every key-taking
// method normalizes here rather than trusting a caller that has already
// validated the type against knownRitualType, so the repository's methods
// can never disagree about which row a given type names.
func normalizeRitualType(ritualType string) string {
	return strings.TrimSpace(strings.ToLower(ritualType))
}
