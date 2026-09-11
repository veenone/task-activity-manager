package backend

import "context"

// Change is one field change read off an issue's changelog, normalised to
// the logical field names the rest of TAM already uses for the same field:
// "storyPoints" matches the name UpdateIssue's field map takes, and
// "sprint" matches issuerepo's EntitySprint. "status" names a transition
// target, not a field: Jira moves an issue's status through a transition,
// never through a field write, so UpdateIssue's map has no entry for it and
// none is expected here. A backend that speaks raw Jira ids
// (customfield_NNNNN, which differs per instance) is responsible for that
// translation itself; nothing downstream of HistoryBackend ever sees a raw
// id.
type Change struct {
	// At is the changelog entry's own timestamp, left exactly as the wire
	// sent it. Jira's changelog timestamps are not RFC 3339 (the offset
	// carries no colon), so parsing one is sprintdate's job, not this
	// type's.
	At    string `json:"at"`
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// IssueHistory is one issue with the changes a sprint report needs to
// reconstruct scope and effort over time, alongside the row the grid
// already knows how to draw.
type IssueHistory struct {
	Issue Issue
	// Changes is the issue's changelog, oldest first, narrowed to the
	// fields a normaliser recognises. A field this task's normaliser does
	// not know about is left out rather than passed through unlabelled: a
	// report built on named fields cannot do anything useful with a field
	// it cannot recognise, and carrying it along would only invite a
	// caller to guess.
	Changes []Change
	// Truncated is true when the issue's changelog carries more entries
	// than this page of the search returned: a search expansion caps how
	// many histories one request answers with, per issue, and a truncated
	// changelog decodes exactly like a complete one except for the
	// startAt/maxResults/total mismatch this flag is computed from. A
	// caller has to check it before trusting a count built from Changes,
	// since presenting a partial history's numbers as exact ones is
	// exactly what this field exists to prevent.
	Truncated bool
}

// HistoryBackend is the changelog capability, kept off IssueBackend the way
// BoardBackend is: only a backend that can walk an issue's history answers
// for it, reached by a type assertion, so a backend that cannot is skipped
// or refused with a sentence rather than forced to grow a no-op
// implementation of a method eight other stubs would then have to carry
// for nothing.
type HistoryBackend interface {
	// SearchIssuesWithHistory runs jql and returns one page of the matching
	// issues, each with its own changelog, plus the total match count for
	// paging the search itself. An issue whose changelog exceeded what came
	// back on this page is reported through its own Truncated rather than
	// failing the whole call, so a report can still show what it does know
	// and say plainly which issues it could not see all of.
	SearchIssuesWithHistory(ctx context.Context, jql string, startAt, maxResults int) ([]IssueHistory, int, error)
}
