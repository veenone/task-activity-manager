package cycles

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"agile-suite/core/store"
	"agile-suite/pie/internal/cases"
	"agile-suite/pie/internal/saved"
)

var at = time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)

func open(t *testing.T) (*Store, *cases.Store, *saved.Store, *sql.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pie.db"), saved.Schema())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	cs := cases.New(db.DB())
	return New(db.DB(), cs), cs, saved.New(db.DB()), db.DB()
}

// caseWith opens a case on key and sets a verdict per project.
func caseWith(t *testing.T, cs *cases.Store, profile, key string, verdicts map[string]string) int64 {
	t.Helper()
	if err := cs.Open(profile, []string{key}, at); err != nil {
		t.Fatal(err)
	}
	list, _ := cs.List(profile)
	var id int64
	for _, c := range list {
		if c.Key == key {
			id = c.ID
		}
	}
	for p, v := range verdicts {
		if err := cs.AddProjects(profile, id, []string{p}); err != nil {
			t.Fatal(err)
		}
		if err := cs.SetVerdict(profile, id, p, v, "because", at); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func publish(t *testing.T, db *sql.DB, id int64) {
	t.Helper()
	if _, err := db.Exec(`UPDATE report_cycle SET published_at = ? WHERE id = ?`, at.Format(time.RFC3339), id); err != nil {
		t.Fatal(err)
	}
}

func TestTakeKeepsTheCasesAsTheyWere(t *testing.T) {
	s, cs, _, _ := open(t)
	id := caseWith(t, cs, "p1", "ACME-1", map[string]string{"ALPHA": cases.Affected})
	c, err := s.Take("p1", " Week 45 ", at)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "Week 45" || c.TakenAt != "2026-11-07T10:00:00Z" || c.PublishedAt != "" {
		t.Fatalf("cycle = %+v", c)
	}
	if err := cs.SetVerdict("p1", id, "ALPHA", cases.NotAffected, "", at); err != nil {
		t.Fatal(err)
	}
	d, err := s.Get("p1", c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Cases[0].Projects[0]; got.Verdict != cases.Affected || got.Reason != "because" {
		t.Fatalf("the cycle followed the live verdict: %+v", got)
	}

	if err := s.Retake("p1", c.ID, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	d, _ = s.Get("p1", c.ID)
	if got := d.Cases[0].Projects[0].Verdict; got != cases.NotAffected || d.TakenAt != "2026-11-07T11:00:00Z" {
		t.Fatalf("Take again kept %q taken %s", got, d.TakenAt)
	}
}

func TestTakeRefusesAnEmptyOrRepeatedName(t *testing.T) {
	s, _, _, _ := open(t)
	if _, err := s.Take("p1", "  ", at); err == nil {
		t.Fatal("an empty name was taken")
	}
	if _, err := s.Take("p1", "Week 45", at); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Take("p1", "Week 45", at); err == nil {
		t.Fatal("a repeated name was taken")
	}
	if _, err := s.Take("p2", "Week 45", at); err != nil {
		t.Fatalf("another profile could not use the name: %v", err)
	}
}

func TestAPublishedCycleCannotBeTakenAgainOrDeleted(t *testing.T) {
	s, _, _, db := open(t)
	c, _ := s.Take("p1", "Week 45", at)
	publish(t, db, c.ID)
	if err := s.Retake("p1", c.ID, at); err == nil {
		t.Fatal("a published cycle was taken again")
	}
	if err := s.Delete("p1", c.ID); err == nil {
		t.Fatal("a published cycle was deleted")
	}
	draft, _ := s.Take("p1", "Week 46", at)
	if err := s.Delete("p1", draft.ID); err != nil {
		t.Fatal(err)
	}
	list, _ := s.List("p1")
	if len(list) != 1 || list[0].Name != "Week 45" {
		t.Fatalf("cycles left = %+v", list)
	}
}

func TestChangesCompareWithTheLastPublishedCycleBefore(t *testing.T) {
	s, cs, _, db := open(t)
	one := caseWith(t, cs, "p1", "ACME-1", map[string]string{"ALPHA": cases.Investigating, "BETA": cases.NotAffected})
	caseWith(t, cs, "p1", "ACME-2", nil)
	first, _ := s.Take("p1", "Week 44", at)
	if d, _ := s.Get("p1", first.ID); d.Changes.Since != "" || !reflect.DeepEqual(d.Changes.New, []string{"ACME-1", "ACME-2"}) {
		t.Fatalf("with nothing published, changes = %+v", d.Changes)
	}
	publish(t, db, first.ID)
	unpublished, _ := s.Take("p1", "Week 45 draft", at)

	if err := cs.SetVerdict("p1", one, "ALPHA", cases.Affected, "", at); err != nil {
		t.Fatal(err)
	}
	if err := cs.AddProjects("p1", one, []string{"GAMMA"}); err != nil {
		t.Fatal(err)
	}
	if err := cs.SetVerdict("p1", one, "GAMMA", cases.Affected, "", at); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM case_item WHERE source_key = 'ACME-2'`); err != nil {
		t.Fatal(err)
	}
	caseWith(t, cs, "p1", "ACME-3", nil)
	now, _ := s.Take("p1", "Week 46", at)

	d, err := s.Get("p1", now.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := Changes{
		Since: "Week 44",
		New:   []string{"ACME-3"},
		Gone:  []string{"ACME-2"},
		Verdicts: []Change{
			{Case: "ACME-1", Project: "ALPHA", From: cases.Investigating, To: cases.Affected},
			{Case: "ACME-1", Project: "GAMMA", From: cases.NotAssessed, To: cases.Affected},
		},
	}
	if !reflect.DeepEqual(d.Changes, want) {
		t.Fatalf("changes = %+v, want %+v", d.Changes, want)
	}
	if d, _ := s.Get("p1", unpublished.ID); d.Changes.Since != "Week 44" {
		t.Fatalf("an unpublished cycle was compared with %q", d.Changes.Since)
	}
}

func TestListIsNewestFirstAndPurgeTakesOnlyItsProfile(t *testing.T) {
	s, _, sv, _ := open(t)
	_, _ = s.Take("p1", "Week 44", at)
	_, _ = s.Take("p1", "Week 45", at.Add(time.Hour))
	_, _ = s.Take("p2", "Other", at)
	list, _ := s.List("p1")
	if len(list) != 2 || list[0].Name != "Week 45" {
		t.Fatalf("list = %+v", list)
	}
	if err := sv.Purge("p1"); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.List("p1"); len(list) != 0 {
		t.Fatalf("purge left %+v", list)
	}
	if list, _ := s.List("p2"); len(list) != 1 {
		t.Fatalf("purge took another profile's cycles: %+v", list)
	}
}
