package issuerepo_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/backend"
)

func names(users []backend.User) []string {
	out := make([]string, len(users))
	for i, u := range users {
		out[i] = u.Name
	}
	return out
}

func TestCacheAndSearchUsers(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.CacheUsers(ctx, "p1", []backend.User{
		{Name: "ranand", DisplayName: "R. Anand"},
		{Name: "skim", DisplayName: "S. Kim"},
		{Name: "mortiz", DisplayName: "M. Ortiz"},
	}); err != nil {
		t.Fatalf("cache: %v", err)
	}
	// A different profile's people are not this profile's.
	if err := r.CacheUsers(ctx, "p2", []backend.User{{Name: "other", DisplayName: "Someone Else"}}); err != nil {
		t.Fatalf("cache p2: %v", err)
	}

	all, err := r.SearchCachedUsers(ctx, "p1", "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	// A blank query is the picker's opening list, ordered by display name.
	wantKeys(t, names(all), "mortiz", "ranand", "skim")

	// Either half of a person matches: the username or the display name.
	byUser, _ := r.SearchCachedUsers(ctx, "p1", "ran")
	wantKeys(t, names(byUser), "ranand")
	byDisplay, _ := r.SearchCachedUsers(ctx, "p1", "kim")
	wantKeys(t, names(byDisplay), "skim")

	n, err := r.CountCachedUsers(ctx, "p1")
	if err != nil || n != 3 {
		t.Errorf("count = %d, %v", n, err)
	}
}

// A second search must update what it knows rather than replace the cache,
// or a narrow query would empty the picker for the next one.
func TestCacheUsersIsAdditiveAndUpdatesDisplayNames(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.CacheUsers(ctx, "p1", []backend.User{{Name: "ranand", DisplayName: "R. Anand"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.CacheUsers(ctx, "p1", []backend.User{
		{Name: "ranand", DisplayName: "Ravi Anand"},
		{Name: "skim", DisplayName: "S. Kim"},
	}); err != nil {
		t.Fatal(err)
	}
	all, _ := r.SearchCachedUsers(ctx, "p1", "")
	wantKeys(t, names(all), "ranand", "skim")
	if all[0].DisplayName != "Ravi Anand" {
		t.Errorf("a renamed person keeps one row: %+v", all[0])
	}
}

// A query is matched literally: LIKE's own wildcards must not silently widen
// what the user asked for.
func TestSearchCachedUsersEscapesWildcards(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.CacheUsers(ctx, "p1", []backend.User{
		{Name: "ranand", DisplayName: "R. Anand"},
		{Name: "a%b", DisplayName: "Percent Person"},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := r.SearchCachedUsers(ctx, "p1", "%")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	wantKeys(t, names(got), "a%b")
}

// Deleting a profile takes its people with it.
func TestPurgeProfileClearsTheUserCache(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.CacheUsers(ctx, "p1", []backend.User{{Name: "ranand", DisplayName: "R. Anand"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.PurgeProfile(ctx, "p1"); err != nil {
		t.Fatalf("purge: %v", err)
	}
	n, err := r.CountCachedUsers(ctx, "p1")
	if err != nil || n != 0 {
		t.Errorf("count after purge = %d, %v", n, err)
	}
}
