package backend

import "context"

// Filter is one saved Jira filter: the id it is pinned by, the name its
// owner gave it, and the JQL it stands for. The JQL travels with it
// because a dashboard built on a filter has to be able to say what it
// counts without asking Jira again.
type Filter struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	JQL  string `json:"jql"`
	// Scope is FilterScopeOwned, FilterScopeShared, or empty when the
	// instance cannot say. Empty is a real answer, not a default: a Jira
	// with no owned-filter route still lists the starred ones, and a
	// picker that groups them by a scope nobody read would be inventing
	// the label. The view draws one flat list in that case.
	Scope string `json:"scope"`
}

// The two scopes a filter can carry. A filter is owned when it came back
// from the read of what this user owns, and shared when the only list it
// appeared in was their starred one, which means somebody else wrote it.
const (
	FilterScopeOwned  = "owned"
	FilterScopeShared = "shared"
)

// FilterBackend is the pair of reads the Dashboards view needs. It is an
// optional interface rather than part of IssueBackend, the way
// WorklogBackend is: a backend that cannot answer says so once, and the
// view reports that instead of pretending there are no filters.
//
// Both are reads. Nothing in this interface writes to Jira, which is what
// keeps a dashboard inside the local-first rule the rest of TAM follows.
type FilterBackend interface {
	// Filters is the saved filters this user has starred.
	Filters(ctx context.Context) ([]Filter, error)
	// SearchByJQL is one page of whatever a JQL matches, plus the total
	// match count for paging it. It is not scoped to the profile's
	// project: a saved filter is not either.
	SearchByJQL(ctx context.Context, jql string, startAt, maxResults int) ([]Issue, int, error)
}
