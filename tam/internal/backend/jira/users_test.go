package jira_test

import (
	"context"
	"strings"
	"testing"
)

func TestSearchUsersAsksTheAssignableEndpointAndDropsWhoCannotBeAssigned(t *testing.T) {
	b, f := newBackend(t, twoFields)
	users, err := b.SearchUsers(context.Background(), "PLAT", "an")
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	// The deactivated account is dropped: it can be searched but cannot hold
	// an issue, so offering it would only produce a failed Commit.
	if len(users) != 2 || users[0].Name != "ranand" || users[0].DisplayName != "R. Anand" {
		t.Fatalf("users = %+v", users)
	}
	// Somebody with no display name is still pickable, under their username.
	if users[1].Name != "nodisplay" || users[1].DisplayName != "nodisplay" {
		t.Errorf("display name falls back to the username: %+v", users[1])
	}
	var q string
	for _, s := range f.searches {
		if strings.HasPrefix(s, "users ") {
			q = s
		}
	}
	// Assignable, scoped to the project: Jira's plain user search answers with
	// people who have no permission on it.
	for _, want := range []string{"project=PLAT", "username=an", "maxResults="} {
		if !strings.Contains(q, want) {
			t.Errorf("query lacks %s: %s", want, q)
		}
	}
}

// A blank query is what seeds the cache, and Jira DC rejects an empty
// username on the assignable endpoint, so it goes out as the wildcard.
func TestSearchUsersSendsAWildcardForABlankQuery(t *testing.T) {
	b, f := newBackend(t, twoFields)
	if _, err := b.SearchUsers(context.Background(), "PLAT", ""); err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	var q string
	for _, s := range f.searches {
		if strings.HasPrefix(s, "users ") {
			q = s
		}
	}
	if !strings.Contains(q, "username=%25") {
		t.Errorf("blank query must go out as the wildcard: %s", q)
	}
}

func TestPrioritiesKeepsJirasOrderAndDropsBlanks(t *testing.T) {
	b, _ := newBackend(t, twoFields)
	got, err := b.Priorities(context.Background())
	if err != nil {
		t.Fatalf("Priorities: %v", err)
	}
	if strings.Join(got, ",") != "Highest,High,Low" {
		t.Errorf("priorities = %v", got)
	}
}
