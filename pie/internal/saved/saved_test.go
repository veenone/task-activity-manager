package saved

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"agile-suite/core/shareddb"
	"agile-suite/core/store"
	"agile-suite/pie/internal/query"
)

func open(t *testing.T) *Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pie.db"), Schema())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(db.DB())
}

func result(keys ...string) query.Result {
	res := query.Result{Total: len(keys), Issues: []query.Issue{}}
	for _, k := range keys {
		res.Issues = append(res.Issues, query.Issue{Key: k, Matched: true})
	}
	return res
}

var at = time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

func TestASavedQueryIsListedByProfile(t *testing.T) {
	s := open(t)
	q, err := s.Save("p1", " Weekly ", " category = X ")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save("p2", "Other", "x"); err != nil {
		t.Fatal(err)
	}
	got, err := s.List("p1")
	if err != nil {
		t.Fatal(err)
	}
	want := []Query{{ID: q.ID, Name: "Weekly", JQL: "category = X"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSaveRefusesABlankOrRepeatedName(t *testing.T) {
	s := open(t)
	if _, err := s.Save("p1", " ", "x"); err == nil {
		t.Fatal("saved a query with no name")
	}
	if _, err := s.Save("p1", "Q", " "); err == nil {
		t.Fatal("saved a query with no JQL")
	}
	if _, err := s.Save("p1", "Q", "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save("p1", "Q", "y"); err == nil || err.Error() != `a saved query named "Q" already exists` {
		t.Fatalf("err = %v", err)
	}
	if _, err := s.Save("p2", "Q", "y"); err != nil {
		t.Fatalf("another profile cannot reuse the name: %v", err)
	}
}

func TestARunIsKeptAndComparedWithTheNext(t *testing.T) {
	s := open(t)
	q, _ := s.Save("p1", "Q", "x")

	first, err := s.Record("p1", q.ID, "x", result("A-1", "B-1"), at)
	if err != nil {
		t.Fatal(err)
	}
	if first.Added != nil || first.Dropped != nil {
		t.Fatalf("a first run has nothing to compare with: %+v", first)
	}

	// B-9 came only through a link, so it is neither added nor dropped.
	next := result("A-1", "C-1")
	next.Issues = append(next.Issues, query.Issue{Key: "B-9"})
	second, err := s.Record("p1", q.ID, " x ", next, at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second.Added, []string{"C-1"}) || !reflect.DeepEqual(second.Dropped, []string{"B-1"}) {
		t.Fatalf("added %v, dropped %v", second.Added, second.Dropped)
	}

	last, err := s.Last("p1", q.ID)
	if err != nil {
		t.Fatal(err)
	}
	if last.RunAt != "2026-11-06T11:00:00Z" || !reflect.DeepEqual(last.Result, next) {
		t.Fatalf("last run %+v", last)
	}
}

// Rows from different JQL say nothing about what changed in Jira, so an
// edited query keeps the JQL it ran and reports no comparison.
func TestEditedJQLIsKeptAndNotCompared(t *testing.T) {
	s := open(t)
	q, _ := s.Save("p1", "Q", "x")
	if _, err := s.Record("p1", q.ID, "x", result("A-1"), at); err != nil {
		t.Fatal(err)
	}
	run, err := s.Record("p1", q.ID, "x AND y", result("B-1"), at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if run.Added != nil || run.Dropped != nil {
		t.Fatalf("compared runs of different JQL: added %v, dropped %v", run.Added, run.Dropped)
	}
	list, _ := s.List("p1")
	if list[0].JQL != "x AND y" || list[0].RunAt != run.RunAt {
		t.Fatalf("the query keeps the JQL it last ran: %+v", list[0])
	}
}

func TestANeverRunQueryHasNoRun(t *testing.T) {
	s := open(t)
	q, _ := s.Save("p1", "Q", "x")
	last, err := s.Last("p1", q.ID)
	if err != nil {
		t.Fatal(err)
	}
	if last.RunAt != "" || last.Result.Issues != nil {
		t.Fatalf("got %+v", last)
	}
}

// A query id from one profile must not reach another profile's rows.
func TestAnotherProfilesQueryIsOutOfReach(t *testing.T) {
	s := open(t)
	q, _ := s.Save("p1", "Q", "x")
	if _, err := s.Last("p2", q.ID); err == nil {
		t.Fatal("read p1's query as p2")
	}
	if _, err := s.Record("p2", q.ID, "x", result("A-1"), at); err == nil {
		t.Fatal("wrote p1's query as p2")
	}
	if err := s.Delete("p2", q.ID); err == nil {
		t.Fatal("deleted p1's query as p2")
	}
}

func TestDeleteAndPurge(t *testing.T) {
	s := open(t)
	a, _ := s.Save("p1", "A", "x")
	_, _ = s.Save("p1", "B", "x")
	keep, _ := s.Save("p2", "C", "x")
	if err := s.Delete("p1", a.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.List("p1"); len(got) != 1 || got[0].Name != "B" {
		t.Fatalf("after delete: %+v", got)
	}
	if err := s.Purge("p1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.List("p1"); len(got) != 0 {
		t.Fatalf("purge left %+v", got)
	}
	if got, _ := s.List("p2"); len(got) != 1 || got[0].ID != keep.ID {
		t.Fatalf("purge reached another profile: %+v", got)
	}
}

// PIE's schema is the profile tables plus its own, so a pie.db written before
// saved queries opens and gains the table.
func TestAnOlderPieDatabaseGainsTheTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pie.db")
	old, err := shareddb.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = old.Close()
	db, err := store.Open(path, Schema())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := New(db.DB()).Save("p1", "Q", "x"); err != nil {
		t.Fatal(err)
	}
}
