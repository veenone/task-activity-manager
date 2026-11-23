package committer_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/committer"
)

// A Commit of 100 rows, the size the issue asks about: 80 edits on 60
// issues, 10 drafts and 10 links, with two issues Jira refuses and one that
// moved remotely. Every row has to end up pushed exactly once or reported,
// and the progress frames have to count up to the 100 the dialog's title
// shows.
func TestCommitOfAHundredRowsPushesOrReportsEveryRowOnce(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()

	var seeded []backend.Issue
	for i := 10; i < 70; i++ {
		key := fmt.Sprintf("PLAT-%d", i)
		iss := backend.Issue{Key: key, ID: fmt.Sprint(i), Project: "PLAT", Type: backend.TypeTask, Summary: "s" + key, Status: "To Do", StatusID: "1", Labels: []string{}, Updated: "2026-09-01T00:00:00Z"}
		seeded = append(seeded, iss)
		f.rows[key] = iss
	}
	if err := repo.UpsertPage(ctx, "p1", seeded, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	rows := 0
	journal := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		rows++
	}
	for i := 10; i < 70; i++ {
		key := fmt.Sprintf("PLAT-%d", i)
		journal(repo.EditField(ctx, "p1", key, "summary", "new "+key))
		if i < 30 {
			journal(repo.EditField(ctx, "p1", key, "labels", "big"))
		}
	}
	for i := 0; i < 10; i++ {
		_, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeStory, Summary: fmt.Sprintf("draft %d", i)})
		journal(err)
		journal(repo.AddLink(ctx, "p1", fmt.Sprintf("PLAT-%d", 40+i), backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: fmt.Sprintf("PLAT-%d", 50+i)}))
	}
	if rows != 100 {
		t.Fatalf("journalled %d rows, want 100", rows)
	}

	// Two refusals, one with two rows, and a conflict.
	f.updateErr["PLAT-11"] = errors.New("refused")
	f.updateErr["PLAT-60"] = errors.New("refused")
	moved := f.rows["PLAT-61"]
	moved.Updated = "2026-09-05T00:00:00Z"
	f.rows["PLAT-61"] = moved

	var frames []committer.Progress
	eng.Progress = func(p committer.Progress) { frames = append(frames, p) }

	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}

	pushes := map[string]int{}
	for _, u := range f.updates {
		pushes[strings.SplitN(u, " ", 2)[0]]++
	}
	for key, n := range pushes {
		if n != 1 {
			t.Errorf("%s pushed %d times", key, n)
		}
	}
	if len(f.updates) != 57 || len(res.Committed) != 57 {
		t.Errorf("57 issues pushed, got %d updates and %d committed", len(f.updates), len(res.Committed))
	}
	if len(f.creates) != 10 || len(res.Created) != 10 || len(f.links) != 10 || len(res.Linked) != 10 {
		t.Errorf("creates %d/%d, links %d/%d", len(f.creates), len(res.Created), len(f.links), len(res.Linked))
	}
	failed := map[string]int{}
	for _, fl := range res.Failures {
		failed[fl.Key]++
	}
	if len(res.Failures) != 2 || failed["PLAT-11"] != 1 || failed["PLAT-60"] != 1 {
		t.Errorf("failures: %+v", res.Failures)
	}
	if len(res.Conflicts) != 1 || res.Conflicts[0].Key != "PLAT-61" {
		t.Errorf("conflicts: %+v", res.Conflicts)
	}
	// What is left is exactly the reported rows: PLAT-11 has two, PLAT-60
	// and PLAT-61 one each.
	left, err := repo.ListPendingChanges(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range left {
		if p.EntityKey != "PLAT-11" && p.EntityKey != "PLAT-60" && p.EntityKey != "PLAT-61" {
			t.Errorf("row %d on %s left in the journal without being reported", p.ID, p.EntityKey)
		}
	}
	if len(left) != 4 || res.Remaining != 4 {
		t.Errorf("left %d rows, Remaining %d, want 4", len(left), res.Remaining)
	}

	if len(frames) < 20 {
		t.Fatalf("only %d progress frames for 100 rows", len(frames))
	}
	sawKey := false
	for i, p := range frames {
		if p.Total != 100 {
			t.Fatalf("frame %d total %d, want 100", i, p.Total)
		}
		if i > 0 && p.Done < frames[i-1].Done {
			t.Fatalf("frame %d went back from %d to %d", i, frames[i-1].Done, p.Done)
		}
		if p.Stage == "edits" && p.Key == "PLAT-30" {
			sawKey = true
		}
	}
	if !sawKey {
		t.Error("no frame named the issue being pushed")
	}
	last := frames[len(frames)-1]
	if !last.Finished || last.Done != 100 {
		t.Errorf("last frame: %+v", last)
	}
}

// A Commit that stops before its first phase still closes the bar.
func TestCommitThatCannotReadTheJournalStillSendsAFinishedFrame(t *testing.T) {
	h := newHarness(t)
	var frames []committer.Progress
	h.eng.Progress = func(p committer.Progress) { frames = append(frames, p) }
	_ = h.db.Close()
	if _, err := h.eng.Commit(context.Background(), "p1", "PLAT"); err == nil {
		t.Fatal("a closed store should fail the commit")
	}
	if len(frames) != 1 || !frames[0].Finished {
		t.Errorf("frames: %+v", frames)
	}
}
