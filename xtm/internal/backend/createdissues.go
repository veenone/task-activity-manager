package backend

import (
	"context"
	"time"
)

// CreateKindTest, CreateKindPrecondition and CreateKindIssue name what a
// create made, for CreatedIssueFinder. The container kinds (KindTestSet,
// KindTestPlan, KindTestExec) are used as they are. CreateKindIssue is any
// other issue, whose type the caller names.
const (
	CreateKindTest         = "test"
	CreateKindPrecondition = "precondition"
	CreateKindIssue        = "issue"
)

// IssueRef is an issue's key and summary.
type IssueRef struct {
	Key     string
	Summary string
}

// CreatedIssueFinder is an optional capability: listing the issues of one
// kind that the connected user created in a project since a given time.
//
// The commit engine asks it after a create got no answer, to find out whether
// Jira made the issue anyway before sending the create again (#607). It is
// kept off Backend because only a Jira-style tracker can answer it with one
// search; a backend without it gets the create sent again, as before.
type CreatedIssueFinder interface {
	FindCreatedIssues(ctx context.Context, projectKey, kind, issueType string, since time.Time) ([]IssueRef, error)
}
