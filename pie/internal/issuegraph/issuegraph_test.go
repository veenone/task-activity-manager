package issuegraph

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"agile-suite/core/store"
	"agile-suite/pie/internal/query"
	"agile-suite/pie/internal/saved"
)

func open(t *testing.T) *Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pie.db"), saved.Schema())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(db.DB())
}

// jira is a fake Jira holding these issues; down makes it unreachable.
type jira struct {
	issues map[string]query.Issue
	down   bool
	asked  [][]string
}

func (j *jira) fetch(_ context.Context, keys []string) ([]query.Issue, error) {
	j.asked = append(j.asked, keys)
	if j.down {
		return nil, &url.Error{Op: "Get", URL: "https://jira.example.com", Err: errors.New("connection refused")}
	}
	var out []query.Issue
	for _, k := range keys {
		if is, ok := j.issues[k]; ok {
			out = append(out, is)
		}
	}
	return out, nil
}

var at = time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)

func world() *jira {
	return &jira{issues: map[string]query.Issue{
		"A-1": {Key: "A-1", Links: []query.Link{{Key: "B-1", Type: "Blocks", Outward: true}}, Subtasks: []string{"A-2"}},
		"A-2": {Key: "A-2", Parent: "A-1"},
		"B-1": {Key: "B-1", Links: []query.Link{{Key: "A-1", Type: "Blocks"}}},
	}}
}

func keysOf(n Neighbourhood) []string {
	var out []string
	for _, c := range n.Issues {
		out = append(out, c.Issue.Key)
	}
	return out
}

func TestExpandFetchesTheIssueAndItsNeighboursAndKeepsThem(t *testing.T) {
	s, j := open(t), world()
	n, err := Expand(context.Background(), j.fetch, s, "p1", "A-1", at)
	if err != nil {
		t.Fatal(err)
	}
	// The issue first, then its links, parent and sub-tasks, each once.
	if got := keysOf(n); !slices.Equal(got, []string{"A-1", "B-1", "A-2"}) || n.Offline {
		t.Fatalf("got %v offline %v", got, n.Offline)
	}
	if n.Issues[0].FetchedAt != at.Format(time.RFC3339) {
		t.Fatalf("fetched at %q", n.Issues[0].FetchedAt)
	}
	kept, err := s.Get("p1", []string{"A-1", "A-2", "B-1", "Z-9"})
	if err != nil || len(kept) != 3 {
		t.Fatalf("kept %d, %v", len(kept), err)
	}
	// Another profile sees none of it.
	if other, _ := s.Get("p2", []string{"A-1"}); len(other) != 0 {
		t.Fatal("another profile read p1's issues")
	}
}

func TestExpandFallsBackToWhatIsStoredWhenJiraIsUnreachable(t *testing.T) {
	s, j := open(t), world()
	if _, err := Expand(context.Background(), j.fetch, s, "p1", "B-1", at); err != nil {
		t.Fatal(err)
	}
	// B-1's neighbour A-1 is stored; A-1's own sub-task A-2 never was.
	j.down = true
	n, err := Expand(context.Background(), j.fetch, s, "p1", "A-1", at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !n.Offline || !slices.Equal(keysOf(n), []string{"A-1", "B-1"}) || !slices.Equal(n.Missing, []string{"A-2"}) {
		t.Fatalf("offline %v, keys %v, missing %v", n.Offline, keysOf(n), n.Missing)
	}
	// What is shown offline carries the time it was fetched, not now.
	if n.Issues[0].FetchedAt != at.Format(time.RFC3339) {
		t.Fatalf("fetched at %q", n.Issues[0].FetchedAt)
	}
}

func TestExpandSaysSoWhenOfflineAndTheIssueWasNeverStored(t *testing.T) {
	s, j := open(t), world()
	j.down = true
	_, err := Expand(context.Background(), j.fetch, s, "p1", "A-1", at)
	if err == nil || !strings.Contains(err.Error(), "A-1") || !IsOffline(err) {
		t.Fatalf("err %v", err)
	}
}

func TestExpandRefusesAKeyJiraDoesNotHave(t *testing.T) {
	s, j := open(t), world()
	_, err := Expand(context.Background(), j.fetch, s, "p1", "Q-1", at)
	if err == nil || IsOffline(err) {
		t.Fatalf("err %v", err)
	}
}

func TestPutReplacesAnIssueFetchedAgain(t *testing.T) {
	s := open(t)
	if err := s.Put("p1", []query.Issue{{Key: "A-1", Status: "Open"}}, at); err != nil {
		t.Fatal(err)
	}
	if err := s.Put("p1", []query.Issue{{Key: "A-1", Status: "Closed"}}, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get("p1", []string{"A-1"})
	if got["A-1"].Issue.Status != "Closed" || got["A-1"].FetchedAt != at.Add(time.Hour).Format(time.RFC3339) {
		t.Fatalf("got %+v", got["A-1"])
	}
}
