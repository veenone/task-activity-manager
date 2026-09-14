package ritualrepo_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/tamstore"
)

func newDocs(t *testing.T) (*ritualrepo.Repository, context.Context) {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return ritualrepo.New(db.DB()), context.Background()
}

var planning = ritualrepo.Key{ProfileID: "p1", BoardID: 1, SprintID: 14, RitualType: "planning"}

func mustDoc(t *testing.T, r *ritualrepo.Repository, ctx context.Context, k ritualrepo.Key) ritualrepo.Document {
	t.Helper()
	d, ok, err := r.Document(ctx, k)
	if err != nil || !ok {
		t.Fatalf("document %+v: ok=%v err=%v", k, ok, err)
	}
	return d
}

func dbOf(t *testing.T, r *ritualrepo.Repository, ctx context.Context) interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
} {
	t.Helper()
	return r.DB()
}

func TestWriteTemplateCreatesALocalRowAndLeavesAWrittenOneAlone(t *testing.T) {
	r, ctx := newDocs(t)
	if err := r.WriteTemplate(ctx, planning, "Sprint 14 · Planning", "<p>template</p>", "t1"); err != nil {
		t.Fatal(err)
	}
	d := mustDoc(t, r, ctx, planning)
	if d.Status != ritualrepo.StatusLocal || d.Body != "<p>template</p>" || d.BaseBody != "" || !d.Dirty() {
		t.Fatalf("fresh = %+v", d)
	}
	if _, err := r.SaveBody(ctx, planning, "<p>mine</p>", "t2"); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteTemplate(ctx, planning, "Sprint 14 · Planning", "<p>template again</p>", "t3"); err != nil {
		t.Fatal(err)
	}
	if got := mustDoc(t, r, ctx, planning).Body; got != "<p>mine</p>" {
		t.Fatalf("a written body was overwritten: %q", got)
	}
}

func TestNeedsTemplateFindsMissingAndEmptyRowsWithTheirLegacyText(t *testing.T) {
	r, ctx := newDocs(t)
	db := dbOf(t, r, ctx)
	for _, stmt := range []string{
		`INSERT INTO ritual_document (profile_id, board_id, sprint_id, ritual_type, remark, issues_json)
		 VALUES ('p1', 1, 14, 'review', 'short sprint', '[{"key":"PLAT-1","remark":"demoed"}]')`,
		`INSERT INTO ritual_document (profile_id, board_id, sprint_id, ritual_type, body) VALUES ('p1', 1, 14, 'retro', '<p>written</p>')`,
		`INSERT INTO ritual_document (profile_id, board_id, sprint_id, ritual_type, confluence_page_id) VALUES ('p1', 1, 14, 'standup', '77')`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	need, err := r.NeedsTemplate(ctx, "p1", 1, 14, []string{"_sprint", "planning", "standup", "review", "retro"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ritualrepo.Legacy{}
	for _, l := range need {
		got[l.RitualType] = l
	}
	if len(got) != 3 {
		t.Fatalf("need = %+v", need)
	}
	if _, ok := got["retro"]; ok {
		t.Error("a row with a body needs no template")
	}
	if _, ok := got["standup"]; ok {
		t.Error("a row with a page id needs no template")
	}
	if l := got["review"]; l.Remark != "short sprint" || len(l.Issues) != 1 || l.Issues[0].Remark != "demoed" {
		t.Errorf("legacy review = %+v", l)
	}
}

func TestSaveBodyStatusFollowsThePageAndTheBase(t *testing.T) {
	r, ctx := newDocs(t)
	_ = r.WriteTemplate(ctx, planning, "T", "<p>a</p>", "t1")
	if d, _ := r.SaveBody(ctx, planning, "<p>b</p>", "t2"); d.Status != ritualrepo.StatusLocal {
		t.Fatalf("no page yet = %s", d.Status)
	}
	if err := r.ApplyCreated(ctx, planning, "42", "<p>b</p>", 1, "t3"); err != nil {
		t.Fatal(err)
	}
	if d := mustDoc(t, r, ctx, planning); d.Status != ritualrepo.StatusSynced || d.Dirty() || d.PageID != "42" {
		t.Fatalf("after create = %+v", d)
	}
	if d, _ := r.SaveBody(ctx, planning, "<p>c</p>", "t4"); d.Status != ritualrepo.StatusUnsynced || !d.Dirty() {
		t.Fatalf("edited = %+v", d)
	}
	if d, _ := r.SaveBody(ctx, planning, "<p>b</p>", "t5"); d.Status != ritualrepo.StatusSynced {
		t.Fatalf("undone back to base = %s", d.Status)
	}
	if _, err := r.SaveBody(ctx, ritualrepo.Key{ProfileID: "p1", BoardID: 1, SprintID: 99, RitualType: "planning"}, "x", "t6"); err == nil {
		t.Fatal("saving a row that does not exist should fail")
	}
}

func TestApplyPulledRefusesABodyThatChangedSinceItWasRead(t *testing.T) {
	r, ctx := newDocs(t)
	_ = r.WriteTemplate(ctx, planning, "T", "<p>read</p>", "t1")
	_, _ = r.SaveBody(ctx, planning, "<p>typed after the read</p>", "t2")
	pulled, err := r.ApplyPulled(ctx, planning, "42", "<p>read</p>", "<p>remote</p>", 2, "t3")
	if err != nil || pulled {
		t.Fatalf("pulled = %v, %v", pulled, err)
	}
	if got := mustDoc(t, r, ctx, planning).Body; got != "<p>typed after the read</p>" {
		t.Fatalf("a pull overwrote a newer save: %q", got)
	}
	pulled, _ = r.ApplyPulled(ctx, planning, "42", "<p>typed after the read</p>", "<p>remote</p>", 2, "t4")
	if d := mustDoc(t, r, ctx, planning); !pulled || d.Body != "<p>remote</p>" || d.BaseBody != "<p>remote</p>" || d.Version != 2 || d.Status != ritualrepo.StatusSynced {
		t.Fatalf("clean pull = %+v", d)
	}
}

func TestApplyPushedLeavesAKeystrokeSavedMidPushUnsynced(t *testing.T) {
	r, ctx := newDocs(t)
	_ = r.WriteTemplate(ctx, planning, "T", "<p>v1</p>", "t1")
	_ = r.ApplyCreated(ctx, planning, "42", "<p>v1</p>", 1, "t2")
	_, _ = r.SaveBody(ctx, planning, "<p>pushed</p>", "t3")
	_, _ = r.SaveBody(ctx, planning, "<p>typed during the push</p>", "t4")
	if err := r.ApplyPushed(ctx, planning, "<p>pushed</p>", 2, "t5"); err != nil {
		t.Fatal(err)
	}
	d := mustDoc(t, r, ctx, planning)
	if d.Body != "<p>typed during the push</p>" || d.BaseBody != "<p>pushed</p>" || d.Version != 2 || d.Status != ritualrepo.StatusUnsynced {
		t.Fatalf("after push = %+v", d)
	}
}

func TestResolveMineRebasesAndTheirsTakesTheRemote(t *testing.T) {
	r, ctx := newDocs(t)
	setup := func() {
		_ = r.DeleteDocument(ctx, planning)
		_ = r.WriteTemplate(ctx, planning, "T", "<p>v1</p>", "t1")
		_ = r.ApplyCreated(ctx, planning, "42", "<p>v1</p>", 1, "t2")
		_, _ = r.SaveBody(ctx, planning, "<p>mine</p>", "t3")
		_ = r.ApplyConflict(ctx, planning, "42", "<p>theirs</p>", 3)
	}
	setup()
	if d := mustDoc(t, r, ctx, planning); d.Status != ritualrepo.StatusConflict || d.ConflictVersion != 3 {
		t.Fatalf("conflict = %+v", d)
	}
	if _, err := r.SaveBody(ctx, planning, "<p>mine, more</p>", "t4"); err != nil {
		t.Fatal(err)
	}
	if d := mustDoc(t, r, ctx, planning); d.Status != ritualrepo.StatusConflict {
		t.Fatalf("a save must not clear a conflict: %s", d.Status)
	}
	if err := r.ResolveMine(ctx, planning, "t5"); err != nil {
		t.Fatal(err)
	}
	if d := mustDoc(t, r, ctx, planning); d.Body != "<p>mine, more</p>" || d.BaseBody != "<p>theirs</p>" || d.Version != 3 || d.ConflictBody != "" || d.Status != ritualrepo.StatusUnsynced {
		t.Fatalf("mine = %+v", d)
	}
	if err := r.ResolveMine(ctx, planning, "t6"); err == nil {
		t.Fatal("resolving a row with no conflict should fail")
	}

	setup()
	if err := r.ResolveTheirs(ctx, planning, "t7"); err != nil {
		t.Fatal(err)
	}
	if d := mustDoc(t, r, ctx, planning); d.Body != "<p>theirs</p>" || d.BaseBody != "<p>theirs</p>" || d.Version != 3 || d.Status != ritualrepo.StatusSynced {
		t.Fatalf("theirs = %+v", d)
	}
}

func TestForgetPageOnlyActsOnAGoneRow(t *testing.T) {
	r, ctx := newDocs(t)
	_ = r.WriteTemplate(ctx, planning, "T", "<p>v1</p>", "t1")
	_ = r.ApplyCreated(ctx, planning, "42", "<p>v1</p>", 1, "t2")
	if err := r.ForgetPage(ctx, planning, "t3"); err == nil {
		t.Fatal("forgetting a live page should fail")
	}
	_ = r.MarkGone(ctx, planning)
	if err := r.ForgetPage(ctx, planning, "t4"); err != nil {
		t.Fatal(err)
	}
	if d := mustDoc(t, r, ctx, planning); d.PageID != "" || d.Version != 0 || d.Body != "<p>v1</p>" || d.Status != ritualrepo.StatusLocal {
		t.Fatalf("forgotten = %+v", d)
	}
}

func TestListingByBoardSprintAndProfile(t *testing.T) {
	r, ctx := newDocs(t)
	for _, k := range []ritualrepo.Key{
		planning,
		{ProfileID: "p1", BoardID: 1, SprintID: 14, RitualType: "_sprint"},
		{ProfileID: "p1", BoardID: 1, SprintID: 12, RitualType: "retro"},
		{ProfileID: "p1", BoardID: 2, SprintID: 14, RitualType: "retro"},
	} {
		_ = r.WriteTemplate(ctx, k, "T", "<p/>", "t")
	}
	docs, _ := r.Documents(ctx, "p1", 1, 14)
	if len(docs) != 2 {
		t.Fatalf("sprint 14 on board 1 = %d", len(docs))
	}
	ids, _ := r.BoardSprintIDs(ctx, "p1", 1)
	if len(ids) != 2 || ids[0] != 12 || ids[1] != 14 {
		t.Fatalf("sprint ids = %v", ids)
	}
	_ = r.ApplyCreated(ctx, planning, "42", "<p/>", 1, "t")
	all, _ := r.ProfileDocuments(ctx, "p1")
	if len(all) != 4 {
		t.Fatalf("profile documents = %d", len(all))
	}
}
