package cases

import (
	"path/filepath"
	"testing"
	"time"

	"agile-suite/core/store"
	"agile-suite/pie/internal/saved"
)

func open(t *testing.T) (*Store, *saved.Store) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pie.db"), saved.Schema())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(db.DB()), saved.New(db.DB())
}

var at = time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

func keys(cs []Case) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Key)
	}
	return out
}

func TestOpenKeepsTheFirstCaseForAnIssue(t *testing.T) {
	s, _ := open(t)
	if err := s.Open("p1", []string{"B-2", "A-1"}, at); err != nil {
		t.Fatal(err)
	}
	if err := s.Open("p1", []string{"A-1", "C-3"}, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := s.List("p1")
	if err != nil {
		t.Fatal(err)
	}
	if k := keys(got); len(k) != 3 || k[0] != "A-1" || k[1] != "B-2" || k[2] != "C-3" {
		t.Fatalf("keys %v", k)
	}
	if got[0].OpenedAt != "2026-11-06T10:00:00Z" || got[0].State != "open" || got[0].Notes != "" {
		t.Fatalf("A-1 = %+v", got[0])
	}
}

func TestOpenRefusesAnythingButIssueKeys(t *testing.T) {
	s, _ := open(t)
	for _, bad := range [][]string{{""}, {"a-1"}, {"A-"}, {"A-1; DROP"}, {"A-1", "nope"}, {}} {
		if err := s.Open("p1", bad, at); err == nil {
			t.Errorf("opened %q", bad)
		}
	}
	if got, _ := s.List("p1"); len(got) != 0 {
		t.Fatalf("a refused call stored %v", keys(got))
	}
	if err := s.Open("p1", []string{"RND_P_X2-17"}, at); err != nil {
		t.Fatalf("a key with digits and underscores: %v", err)
	}
}

func TestCasesBelongToTheirProfileAndGoWithIt(t *testing.T) {
	s, sv := open(t)
	_ = s.Open("p1", []string{"A-1"}, at)
	_ = s.Open("p2", []string{"B-1"}, at)
	if err := sv.Purge("p1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.List("p1"); len(got) != 0 {
		t.Fatalf("purge left %v", keys(got))
	}
	if got, _ := s.List("p2"); len(got) != 1 {
		t.Fatalf("purge reached p2: %v", keys(got))
	}
}
