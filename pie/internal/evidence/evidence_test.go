package evidence

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	corejira "agile-suite/core/jira"
	"agile-suite/core/store"
	"agile-suite/pie/internal/cases"
	"agile-suite/pie/internal/saved"
)

type fixture struct {
	ev    *Store
	cs    *cases.Store
	sv    *saved.Store
	id    int64
	mu    sync.Mutex
	asked []string
}

const sourceIssue = `{"key":"A-1","fields":{"project":{"key":"A"},"issuetype":{"name":"Bug"},
	"components":[{"name":"Reset"},{"name":"Ghost"}],"fixVersions":[{"name":"2.4"}],
	"issuelinks":[
		{"type":{"inward":"is cloned by","outward":"clones"},"outwardIssue":{"key":"B-7","fields":{"summary":"Reset again","status":{"name":"Open"}}}},
		{"type":{"inward":"relates to","outward":"relates to"},"inwardIssue":{"key":"A-2","fields":{"summary":"Same project","status":{"name":"Open"}}}}]}}`

func setup(t *testing.T) (*fixture, *corejira.Client) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pie.db"), saved.Schema())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	f := &fixture{ev: New(db.DB()), cs: cases.New(db.DB()), sv: saved.New(db.DB())}
	if err := f.cs.Open("p1", []string{"A-1"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	list, _ := f.cs.List("p1")
	f.id = list[0].ID

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/2/issue/A-1" {
			_, _ = w.Write([]byte(sourceIssue))
			return
		}
		jql := r.URL.Query().Get("jql")
		f.mu.Lock()
		f.asked = append(f.asked, jql)
		f.mu.Unlock()
		issue := func(key, summary string) string {
			p, _, _ := strings.Cut(key, "-")
			return `{"key":"` + key + `","fields":{"project":{"key":"` + p + `"},"summary":"` + summary + `"}}`
		}
		var hits []string
		switch {
		case strings.Contains(jql, `component = "Ghost"`):
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errorMessages":["The value 'Ghost' does not exist for the field 'component'."]}`))
			return
		case strings.Contains(jql, `component = "Reset"`):
			hits = []string{issue("A-5", "x"), issue("B-1", "x"), issue("B-2", "x"), issue("C-1", "x")}
		case strings.Contains(jql, `fixVersion = "2.4"`):
			hits = []string{issue("D-1", "x")}
		case strings.Contains(jql, `text ~`):
			hits = []string{issue("E-3", "PIN retry lost")}
		}
		_, _ = w.Write([]byte(`{"total":` + strconv.Itoa(len(hits)) + `,"issues":[` + strings.Join(hits, ",") + `]}`))
	}))
	t.Cleanup(srv.Close)
	return f, corejira.NewClient(srv.URL, "t")
}

type seen struct{ Project, Kind, Ref string }

func summarise(items []Item) []seen {
	var out []seen
	for _, it := range items {
		out = append(out, seen{it.Project, it.Kind, it.Ref})
	}
	return out
}

func TestFindSuggestsLinksNamesAndPhrases(t *testing.T) {
	f, client := setup(t)
	n, err := Find(context.Background(), client, f.ev, "p1", f.id, "A-1", []string{" PIN retry "})
	if err != nil {
		t.Fatal(err)
	}
	items, _ := f.ev.List("p1", f.id)
	want := []seen{
		{"B", Link, "B-7"}, {"B", Component, "Reset"}, {"C", Component, "Reset"},
		{"D", Version, "2.4"}, {"E", Text, "E-3"},
	}
	if got := summarise(items); !reflect.DeepEqual(got, want) || n != len(want) {
		t.Fatalf("found %d: %+v", n, got)
	}
	for _, it := range items {
		if it.State != Suggested || it.Weak != (it.Kind != Link) {
			t.Errorf("%+v", it)
		}
	}
	if items[0].Detail != "clones B-7: Reset again (Open)" || items[1].Detail != `2 issues with component "Reset"` {
		t.Errorf("details %q, %q", items[0].Detail, items[1].Detail)
	}
	var text string
	for _, q := range f.asked {
		if strings.Contains(q, "text ~") {
			text = q
		}
	}
	if text != `text ~ "\"PIN retry\"" AND issuetype = "Bug" AND key != A-1` {
		t.Errorf("text search sent %q", text)
	}
}

func TestFindAgainKeepsWhatTheAnalystDecided(t *testing.T) {
	f, client := setup(t)
	_, _ = Find(context.Background(), client, f.ev, "p1", f.id, "A-1", nil)
	items, _ := f.ev.List("p1", f.id)
	if err := f.ev.SetState("p1", f.id, items[0].ID, Accepted); err != nil {
		t.Fatal(err)
	}
	if err := f.ev.SetState("p1", f.id, items[1].ID, Dismissed); err != nil {
		t.Fatal(err)
	}
	n, err := Find(context.Background(), client, f.ev, "p1", f.id, "A-1", nil)
	if err != nil || n != 0 {
		t.Fatalf("second run added %d, %v", n, err)
	}
	again, _ := f.ev.List("p1", f.id)
	if again[0].State != Accepted || again[1].State != Dismissed || len(again) != len(items) {
		t.Fatalf("after a re-run %+v", again)
	}
}

func TestAPhraseWithAQuoteIsRefusedBeforeJiraIsAsked(t *testing.T) {
	f, client := setup(t)
	if _, err := Find(context.Background(), client, f.ev, "p1", f.id, "A-1", []string{`say "hi"`}); err == nil {
		t.Fatal("searched a phrase with a quote")
	}
	if len(f.asked) != 0 {
		t.Fatalf("asked Jira %v", f.asked)
	}
}

func TestEvidenceAddedByHand(t *testing.T) {
	f, _ := setup(t)
	if err := f.ev.AddNote("p1", f.id, "B", "  Field team saw it too. "); err != nil {
		t.Fatal(err)
	}
	if err := f.ev.AddIssue("p1", f.id, "C", "C-40"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []error{
		f.ev.AddNote("p1", f.id, "B", " "),
		f.ev.AddNote("p1", f.id, "bad", "x"),
		f.ev.AddIssue("p1", f.id, "C", "nope"),
		f.ev.AddIssue("p2", f.id, "C", "C-41"),
	} {
		if bad == nil {
			t.Error("stored bad manual evidence")
		}
	}
	items, _ := f.ev.List("p1", f.id)
	if len(items) != 2 || items[0].Detail != "Field team saw it too." || items[0].State != Accepted || items[1].Ref != "C-40" {
		t.Fatalf("items %+v", items)
	}
	if err := f.ev.SetState("p1", f.id, items[0].ID, Dismissed); err == nil {
		t.Error("dismissed a note added by hand")
	}
	if err := f.ev.Remove("p1", f.id, items[0].ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.ev.List("p1", f.id); len(got) != 1 {
		t.Fatalf("after remove %+v", got)
	}
}

func TestASuggestionCannotBeRemoved(t *testing.T) {
	f, client := setup(t)
	_, _ = Find(context.Background(), client, f.ev, "p1", f.id, "A-1", nil)
	items, _ := f.ev.List("p1", f.id)
	if err := f.ev.Remove("p1", f.id, items[0].ID); err == nil {
		t.Fatal("removed a suggestion; dismiss it instead")
	}
}

// The design rests on this: evidence informs a verdict and never sets one.
func TestNoEvidenceActionChangesAVerdict(t *testing.T) {
	f, client := setup(t)
	_ = f.cs.AddProjects("p1", f.id, []string{"B", "C"})
	_ = f.cs.SetVerdict("p1", f.id, "B", cases.NotAffected, "checked", time.Now())
	before, _ := f.cs.Get("p1", f.id)

	_, _ = Find(context.Background(), client, f.ev, "p1", f.id, "A-1", []string{"PIN retry"})
	items, _ := f.ev.List("p1", f.id)
	_ = f.ev.SetState("p1", f.id, items[0].ID, Accepted)
	_ = f.ev.SetState("p1", f.id, items[1].ID, Dismissed)
	_ = f.ev.AddNote("p1", f.id, "C", "affected for sure")
	_ = f.ev.AddIssue("p1", f.id, "C", "C-9")
	all, _ := f.ev.List("p1", f.id)
	_ = f.ev.Remove("p1", f.id, all[len(all)-1].ID)

	after, _ := f.cs.Get("p1", f.id)
	if !reflect.DeepEqual(before.Projects, after.Projects) {
		t.Fatalf("verdicts moved:\n%+v\n%+v", before.Projects, after.Projects)
	}
}

func TestPurgeTakesEvidence(t *testing.T) {
	f, _ := setup(t)
	_ = f.ev.AddNote("p1", f.id, "B", "x")
	if err := f.sv.Purge("p1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.ev.List("p1", f.id); len(got) != 0 {
		t.Fatalf("purge left %+v", got)
	}
}

func TestEvidenceIsChangedOnlyThroughItsOwnCase(t *testing.T) {
	f, _ := setup(t)
	_ = f.ev.AddNote("p1", f.id, "B", "x")
	items, _ := f.ev.List("p1", f.id)
	if err := f.ev.Remove("p1", f.id+1, items[0].ID); err == nil {
		t.Fatal("removed evidence through another case")
	}
	if got, _ := f.ev.List("p1", f.id); len(got) != 1 {
		t.Fatalf("evidence gone: %+v", got)
	}
}
