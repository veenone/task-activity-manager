package committer_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/committer"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/tamstore"
)

// fake is a Jira that remembers its rows and every write.
type fake struct {
	rows      map[string]backend.Issue
	desc      map[string]string
	updates   []string // "KEY field=value,..." in field order
	creates   []backend.IssueDraft
	nextKey   int
	updateErr map[string]error
	createErr error
	getErr    map[string]error
}

func newFake() *fake {
	return &fake{rows: map[string]backend.Issue{}, desc: map[string]string{}, nextKey: 501, updateErr: map[string]error{}, getErr: map[string]error{}}
}

func (f *fake) TestConnection(context.Context) (backend.User, error) {
	return backend.User{Name: "f"}, nil
}
func (f *fake) IsDemo() bool { return false }
func (f *fake) SearchIssuesPage(context.Context, string, string, string, []string, int, int) ([]backend.Issue, int, error) {
	return nil, 0, errors.New("not used")
}
func (f *fake) IssueTypes(context.Context, string) ([]backend.IssueType, error) { return nil, nil }
func (f *fake) CreateFields(context.Context, string, string) ([]backend.FieldSpec, error) {
	return []backend.FieldSpec{}, nil
}
func (f *fake) GetIssueDetail(_ context.Context, key string) (backend.IssueDetail, error) {
	return backend.IssueDetail{Key: key, Description: f.desc[key], Links: []backend.Link{}, Fields: map[string]any{}}, nil
}
func (f *fake) GetIssue(_ context.Context, key string) (backend.Issue, error) {
	if err := f.getErr[key]; err != nil {
		return backend.Issue{}, err
	}
	iss, ok := f.rows[key]
	if !ok {
		return backend.Issue{}, fmt.Errorf("no issue %s", key)
	}
	return iss, nil
}
func (f *fake) UpdateIssue(_ context.Context, key string, fields map[string]string) error {
	if err := f.updateErr[key]; err != nil {
		return err
	}
	iss := f.rows[key]
	parts := []string{}
	for _, name := range issuerepo.EditableFields {
		v, ok := fields[name]
		if !ok {
			continue
		}
		parts = append(parts, name+"="+v)
		switch name {
		case "summary":
			iss.Summary = v
		case "priority":
			iss.Priority = v
		case "assignee":
			iss.Assignee = v
		case "labels":
			iss.Labels = backend.SplitLabels(v)
		case "storyPoints":
			iss.StoryPoints, _ = backend.ParsePoints(v)
		case "description":
			f.desc[key] = v
		}
	}
	iss.Updated = "2026-09-07T00:00:00Z"
	f.rows[key] = iss
	f.updates = append(f.updates, key+" "+strings.Join(parts, ","))
	return nil
}
func (f *fake) CreateIssue(_ context.Context, projectKey string, d backend.IssueDraft) (string, error) {
	if f.createErr != nil {
		return "", f.createErr
	}
	f.creates = append(f.creates, d)
	key := fmt.Sprintf("%s-%d", projectKey, f.nextKey)
	f.nextKey++
	f.rows[key] = backend.Issue{Key: key, ID: "9", Project: projectKey, Type: d.Type, Summary: d.Summary, Status: "To Do", Labels: d.Labels, StoryPoints: d.StoryPoints, Updated: "2026-09-07T00:00:00Z"}
	return key, nil
}

func setup(t *testing.T) (*committer.Engine, *issuerepo.Repository, *fake) {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := issuerepo.New(db.DB())
	f := newFake()
	rows := []backend.Issue{
		{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "To Do", Priority: "Medium", Labels: []string{"a"}, StoryPoints: pts(3), Updated: "2026-09-01T00:00:00Z"},
		{Key: "PLAT-2", ID: "2", Project: "PLAT", Type: backend.TypeStory, Summary: "two", Status: "To Do", Labels: []string{}, Updated: "2026-09-01T00:00:00Z"},
	}
	for _, r := range rows {
		f.rows[r.Key] = r
	}
	f.desc["PLAT-1"] = "remote text"
	if err := repo.UpsertPage(context.Background(), "p1", rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	return committer.New(f, repo), repo, f
}

func pts(v float64) *float64 { return &v }

func TestCommitPushesEditsRefreshesRowsAndClearsTheJournal(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	_ = repo.EditField(ctx, "p1", "PLAT-1", "summary", "uno")
	_ = repo.EditField(ctx, "p1", "PLAT-1", "labels", "a, b")
	_ = repo.EditField(ctx, "p1", "PLAT-2", "storyPoints", "5")
	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if strings.Join(res.Committed, ",") != "PLAT-1,PLAT-2" || len(res.Conflicts) != 0 || len(res.Failures) != 0 || res.Remaining != 0 {
		t.Errorf("result: %+v", res)
	}
	if len(f.updates) != 2 || f.updates[0] != "PLAT-1 summary=uno,labels=a, b" || f.updates[1] != "PLAT-2 storyPoints=5" {
		t.Errorf("updates: %v", f.updates)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.Pending || iss.Updated != "2026-09-07T00:00:00Z" || iss.Summary != "uno" {
		t.Errorf("row refreshed from the fake: %+v", iss)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	commits := 0
	for _, a := range act {
		if a.Action == "commit" {
			commits++
		}
	}
	if commits != 2 {
		t.Errorf("two commit entries: %+v", act)
	}
}

func TestCommitHoldsBackAConflictWithBaseMineRemote(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	_ = repo.EditField(ctx, "p1", "PLAT-1", "storyPoints", "8")
	_ = repo.EditField(ctx, "p1", "PLAT-1", "description", "mine text")
	_ = repo.EditField(ctx, "p1", "PLAT-2", "summary", "dos")
	remote := f.rows["PLAT-1"]
	remote.Updated = "2026-09-05T00:00:00Z"
	remote.StoryPoints = pts(13)
	f.rows["PLAT-1"] = remote

	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Conflicts) != 1 || strings.Join(res.Committed, ",") != "PLAT-2" || res.Remaining != 2 {
		t.Fatalf("result: %+v", res)
	}
	c := res.Conflicts[0]
	if c.Key != "PLAT-1" || c.RemoteVersion != "2026-09-05T00:00:00Z" || c.Summary != "one" || len(c.Fields) != 2 {
		t.Errorf("conflict: %+v", c)
	}
	byField := map[string]committer.FieldConflict{}
	for _, fc := range c.Fields {
		byField[fc.Field] = fc
	}
	if p := byField["storyPoints"]; p.Base != "3" || p.Mine != "8" || p.Remote != "13" {
		t.Errorf("points: %+v", p)
	}
	if d := byField["description"]; d.Base != "" || d.Mine != "mine text" || d.Remote != "remote text" {
		t.Errorf("description base is what the cache held when edited: %+v", d)
	}
	for _, u := range f.updates {
		if strings.HasPrefix(u, "PLAT-1") {
			t.Errorf("a held issue must not be pushed: %v", f.updates)
		}
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if !iss.Pending || *iss.StoryPoints != 8 {
		t.Errorf("local edit stays: %+v", iss)
	}

	// Override rebases and the next commit pushes.
	if err := eng.ResolveOverride(ctx, "p1", "PLAT-1", c.RemoteVersion); err != nil {
		t.Fatal(err)
	}
	res, _ = eng.Commit(ctx, "p1", "PLAT")
	if strings.Join(res.Committed, ",") != "PLAT-1" || res.Remaining != 0 {
		t.Errorf("after override: %+v", res)
	}
	if last := f.updates[len(f.updates)-1]; last != "PLAT-1 description=mine text,storyPoints=8" {
		t.Errorf("pushed both fields: %s", last)
	}
}

func TestKeepRemoteDropsTheEditsAndTakesJirasRow(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	_ = repo.EditField(ctx, "p1", "PLAT-1", "summary", "mine")
	remote := f.rows["PLAT-1"]
	remote.Summary = "theirs"
	remote.Updated = "2026-09-05T00:00:00Z"
	f.rows["PLAT-1"] = remote
	if err := eng.ResolveKeepRemote(ctx, "p1", "PLAT-1"); err != nil {
		t.Fatal(err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.Pending || iss.Summary != "theirs" || iss.Updated != "2026-09-05T00:00:00Z" {
		t.Errorf("row: %+v", iss)
	}
	if len(f.updates) != 0 {
		t.Errorf("nothing pushed: %v", f.updates)
	}
}

func TestCommitCreatesDraftsFirstAndRekeysThem(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	temp, _ := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "New bug", Labels: []string{"x"}, StoryPoints: pts(2)})
	_ = repo.EditField(ctx, "p1", temp, "summary", "New bug, renamed")
	_ = repo.EditField(ctx, "p1", "PLAT-2", "summary", "dos")
	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) != 1 || res.Created[0].TempKey != temp || res.Created[0].Key != "PLAT-501" || strings.Join(res.Committed, ",") != "PLAT-2" || res.Remaining != 0 {
		t.Errorf("result: %+v", res)
	}
	if len(f.creates) != 1 || f.creates[0].Summary != "New bug, renamed" || f.creates[0].Type != backend.TypeBug {
		t.Errorf("posted draft: %+v", f.creates)
	}
	if _, err := repo.GetIssue(ctx, "p1", temp); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("temp key gone: %v", err)
	}
	iss, err := repo.GetIssue(ctx, "p1", "PLAT-501")
	if err != nil || iss.Pending || iss.Draft || iss.Status != "To Do" || iss.Summary != "New bug, renamed" {
		t.Errorf("created row refreshed from Jira: %+v %v", iss, err)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-501", 0)
	if len(act) < 3 || act[0].Action != "commit" {
		t.Errorf("audit trail followed the key: %+v", act)
	}
}

func TestFailuresKeepTheRowsForNextTime(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	_ = repo.EditField(ctx, "p1", "PLAT-1", "summary", "uno")
	_ = repo.EditField(ctx, "p1", "PLAT-2", "summary", "dos")
	temp, _ := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "d"})
	f.updateErr["PLAT-1"] = errors.New("PUT failed: 400 summary too long")
	f.createErr = errors.New("POST failed: 400 Severity is required")
	f.getErr["PLAT-2"] = errors.New("GET failed: 502")
	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 3 || len(res.Committed) != 0 || len(res.Created) != 0 || res.Remaining != 3 {
		t.Errorf("result: %+v", res)
	}
	keys := map[string]string{}
	for _, fl := range res.Failures {
		keys[fl.Key] = fl.Error
	}
	if !strings.Contains(keys["PLAT-1"], "summary too long") || !strings.Contains(keys[temp], "Severity") || !strings.Contains(keys["PLAT-2"], "502") {
		t.Errorf("failures: %v", keys)
	}
	if pend, _ := repo.ListPendingChanges(ctx, "p1"); len(pend) != 3 {
		t.Errorf("all rows kept: %+v", pend)
	}
	if _, err := repo.GetIssue(ctx, "p1", temp); err != nil {
		t.Errorf("draft kept: %v", err)
	}
}

func TestCommitWithNothingPendingIsEmpty(t *testing.T) {
	eng, _, f := setup(t)
	res, err := eng.Commit(context.Background(), "p1", "PLAT")
	if err != nil || len(res.Committed)+len(res.Created)+len(res.Conflicts)+len(res.Failures) != 0 || res.Remaining != 0 {
		t.Errorf("empty: %+v %v", res, err)
	}
	if res.Committed == nil || res.Created == nil || res.Conflicts == nil || res.Failures == nil {
		t.Error("slices are non-nil so the frontend sees [] not null")
	}
	if len(f.updates) != 0 {
		t.Error("no writes")
	}
}
