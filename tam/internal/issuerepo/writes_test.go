package issuerepo_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

const v1 = "2026-09-01T00:00:00Z"

func seedOne(t *testing.T, repo *issuerepo.Repository, profileID string) {
	t.Helper()
	rows := []backend.Issue{{
		Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "To Do",
		Priority: "Medium", Labels: []string{"a"}, StoryPoints: pts(3), Rank: "0|a", Updated: v1,
	}}
	if err := repo.UpsertPage(context.Background(), profileID, rows, time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestEditFieldWritesTheRowAndJournalsAgainstUpdated(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	if err := repo.EditField(ctx, "p1", "PLAT-1", "summary", "uno"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.Summary != "uno" || !iss.Pending {
		t.Errorf("row after edit: %+v", iss)
	}
	pend, _ := repo.ListPendingChanges(ctx, "p1")
	if len(pend) != 1 || pend[0].Field != "summary" || pend[0].BeforeVal != "one" || pend[0].AfterVal != "uno" || pend[0].BaseVersion != v1 || pend[0].EntityType != issuerepo.EntityIssue {
		t.Errorf("journal: %+v", pend)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	if len(act) != 1 || act[0].Action != "edit" || act[0].AfterVal != "uno" {
		t.Errorf("audit: %+v", act)
	}
	// A second edit coalesces; a return to the original drops the row.
	if err := repo.EditField(ctx, "p1", "PLAT-1", "summary", "eins"); err != nil {
		t.Fatal(err)
	}
	pend, _ = repo.ListPendingChanges(ctx, "p1")
	if len(pend) != 1 || pend[0].BeforeVal != "one" || pend[0].AfterVal != "eins" {
		t.Errorf("coalesced: %+v", pend)
	}
	if err := repo.EditField(ctx, "p1", "PLAT-1", "summary", "one"); err != nil {
		t.Fatal(err)
	}
	pend, _ = repo.ListPendingChanges(ctx, "p1")
	iss, _ = repo.GetIssue(ctx, "p1", "PLAT-1")
	if len(pend) != 0 || iss.Pending || iss.Summary != "one" {
		t.Errorf("revert: pending=%+v row=%+v", pend, iss)
	}
}

func TestEditFieldHandlesEveryEditableField(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	if err := repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Description: "old text"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	for field, value := range map[string]string{
		"labels": "b, c", "storyPoints": "8", "priority": "High", "assignee": "jdoe", "description": "new text",
	} {
		if err := repo.EditField(ctx, "p1", "PLAT-1", field, value); err != nil {
			t.Fatalf("edit %s: %v", field, err)
		}
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if strings.Join(iss.Labels, ",") != "b,c" || iss.StoryPoints == nil || *iss.StoryPoints != 8 || iss.Priority != "High" || iss.Assignee != "jdoe" {
		t.Errorf("row: %+v", iss)
	}
	d, _, ok, err := repo.ReadDetail(ctx, "p1", "PLAT-1")
	if err != nil || !ok || d.Description != "new text" {
		t.Errorf("description lives in the detail cache: %+v %v %v", d, ok, err)
	}
	pend, _ := repo.ListPendingChanges(ctx, "p1")
	if len(pend) != 5 {
		t.Errorf("one journal row per field: %d", len(pend))
	}
	for _, p := range pend {
		switch p.Field {
		case "labels":
			if p.BeforeVal != "a" || p.AfterVal != "b, c" {
				t.Errorf("labels journal as a comma list: %+v", p)
			}
		case "storyPoints":
			if p.BeforeVal != "3" || p.AfterVal != "8" {
				t.Errorf("points journal as text: %+v", p)
			}
		case "description":
			if p.BeforeVal != "old text" {
				t.Errorf("description before: %+v", p)
			}
		}
	}
	if err := repo.EditField(ctx, "p1", "PLAT-1", "storyPoints", ""); err != nil {
		t.Fatal(err)
	}
	if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1"); iss.StoryPoints != nil {
		t.Errorf("empty points clears the column: %+v", iss)
	}
	if err := repo.EditField(ctx, "p1", "PLAT-1", "storyPoints", "eight"); err == nil {
		t.Error("non-numeric points must be rejected")
	}
	if err := repo.EditField(ctx, "p1", "PLAT-1", "status", "Done"); err == nil {
		t.Error("status is not editable")
	}
	if err := repo.EditField(ctx, "p1", "PLAT-1", "summary", "  "); err == nil {
		t.Error("summary cannot be blank")
	}
	if err := repo.EditField(ctx, "p1", "PLAT-9", "summary", "x"); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("missing issue: %v", err)
	}
}

func TestCreateDraftNumbersPerProfileAndEditsUpdateItsJSON(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	d := backend.IssueDraft{Type: backend.TypeTask, Summary: "Add a retry", Priority: "Medium", Labels: []string{"payments"}, Assignee: "mortiz", StoryPoints: pts(3)}
	k1, err := repo.CreateDraft(ctx, "p1", "PLAT", d)
	if err != nil || k1 != "TAM-NEW-1" {
		t.Fatalf("first draft: %q %v", k1, err)
	}
	k2, _ := repo.CreateDraft(ctx, "p1", "PLAT", d)
	other, _ := repo.CreateDraft(ctx, "p2", "PLAT", d)
	if k2 != "TAM-NEW-2" || other != "TAM-NEW-1" {
		t.Errorf("numbering: %s %s", k2, other)
	}
	iss, err := repo.GetIssue(ctx, "p1", k1)
	if err != nil || !iss.Draft || !iss.Pending || iss.Status != issuerepo.StatusDraft || iss.Project != "PLAT" || iss.Summary != "Add a retry" || *iss.StoryPoints != 3 {
		t.Errorf("draft row: %+v %v", iss, err)
	}
	pend, _ := repo.PendingForKey(ctx, "p1", k1)
	if len(pend) != 1 || pend[0].EntityType != issuerepo.EntityIssueCreate || pend[0].Field != issuerepo.FieldCreate {
		t.Fatalf("create row: %+v", pend)
	}
	var stored backend.IssueDraft
	if err := json.Unmarshal([]byte(pend[0].AfterVal), &stored); err != nil || stored.Summary != "Add a retry" || stored.Type != backend.TypeTask {
		t.Errorf("create JSON: %s %v", pend[0].AfterVal, err)
	}
	if err := repo.EditField(ctx, "p1", k1, "summary", "Add a retry to the consumer"); err != nil {
		t.Fatal(err)
	}
	pend, _ = repo.PendingForKey(ctx, "p1", k1)
	_ = json.Unmarshal([]byte(pend[0].AfterVal), &stored)
	if len(pend) != 1 || stored.Summary != "Add a retry to the consumer" {
		t.Errorf("editing a draft rewrites its JSON instead of adding a row: %+v", pend)
	}
	if _, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeEpic, Summary: "x"}); err == nil {
		t.Error("only task, story, and bug can be drafted in 1b")
	}
	if _, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask}); err == nil {
		t.Error("a draft needs a summary")
	}
	det, _, ok, _ := repo.ReadDetail(ctx, "p1", k1)
	if !ok || det.Description != "" {
		t.Errorf("a draft has a detail cache so the panel never asks the backend: %+v %v", det, ok)
	}
}

func TestSyncKeepsPendingColumnsAndADraft(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	if _, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.EditField(ctx, "p1", "PLAT-1", "summary", "mine"); err != nil {
		t.Fatal(err)
	}
	if err := repo.EditField(ctx, "p1", "PLAT-1", "labels", "x, y"); err != nil {
		t.Fatal(err)
	}
	remote := []backend.Issue{{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "theirs", Status: "Done", Labels: []string{"z"}, Updated: "2026-09-02T00:00:00Z"}}
	for _, full := range []bool{false, true} {
		if err := repo.UpsertPage(ctx, "p1", remote, time.Now(), full); err != nil {
			t.Fatalf("sync full=%v: %v", full, err)
		}
		iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
		if iss.Summary != "mine" || strings.Join(iss.Labels, ",") != "x,y" {
			t.Errorf("full=%v: pending columns must win: %+v", full, iss)
		}
		if iss.Status != "Done" || iss.Updated != "2026-09-02T00:00:00Z" {
			t.Errorf("full=%v: untouched columns follow Jira: %+v", full, iss)
		}
		if _, err := repo.GetIssue(ctx, "p1", "TAM-NEW-1"); err != nil {
			t.Errorf("full=%v: the draft survives: %v", full, err)
		}
	}
	pend, _ := repo.PendingForKey(ctx, "p1", "PLAT-1")
	for _, p := range pend {
		if p.BaseVersion != v1 {
			t.Errorf("a sync never moves the base: %+v", p)
		}
	}
}

func TestWriteDetailKeepsAPendingDescription(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Description: "old"}, time.Now())
	if err := repo.EditField(ctx, "p1", "PLAT-1", "description", "mine"); err != nil {
		t.Fatal(err)
	}
	_ = repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Description: "theirs", Links: []backend.Link{{Direction: "outward", Type: "Relates", Key: "PLAT-2"}}}, time.Now())
	d, _, _, _ := repo.ReadDetail(ctx, "p1", "PLAT-1")
	if d.Description != "mine" || len(d.Links) != 1 {
		t.Errorf("refresh keeps the pending description and takes the links: %+v", d)
	}
}

func TestRekeyMovesEveryTable(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	k, _ := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "d"})
	if err := repo.Rekey(ctx, "p1", k, "PLAT-501"); err != nil {
		t.Fatalf("rekey: %v", err)
	}
	if _, err := repo.GetIssue(ctx, "p1", k); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("old key gone: %v", err)
	}
	iss, err := repo.GetIssue(ctx, "p1", "PLAT-501")
	if err != nil || iss.Draft || !iss.Pending {
		t.Errorf("new key carries the row and its create row: %+v %v", iss, err)
	}
	if pend, _ := repo.PendingForKey(ctx, "p1", "PLAT-501"); len(pend) != 1 {
		t.Errorf("pending moved: %+v", pend)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-501", 0)
	if len(act) != 2 || act[0].Action != "created" || act[0].BeforeVal != k || act[1].Action != "create" {
		t.Errorf("audit moved and gained the created entry: %+v", act)
	}
}

func TestReplaceRowOverwritesAndClearsTheDetailCache(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Description: "cached"}, time.Now())
	fresh := backend.Issue{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "fresh", Status: "Done", Labels: []string{}, Updated: "2026-09-03T00:00:00Z"}
	if err := repo.ReplaceRow(ctx, "p1", fresh); err != nil {
		t.Fatalf("replace: %v", err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.Summary != "fresh" || iss.Status != "Done" || iss.Updated != "2026-09-03T00:00:00Z" || iss.StoryPoints != nil {
		t.Errorf("row: %+v", iss)
	}
	if _, _, ok, _ := repo.ReadDetail(ctx, "p1", "PLAT-1"); ok {
		t.Error("the detail cache is dropped so the panel refetches")
	}
}

func TestDiscardRevertsAnEditOrDropsADraft(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.EditField(ctx, "p1", "PLAT-1", "priority", "High")
	_ = repo.EditField(ctx, "p1", "PLAT-1", "labels", "q")
	k, _ := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "d"})
	pend, _ := repo.PendingForKey(ctx, "p1", "PLAT-1")
	var prio journal.PendingChange
	for _, p := range pend {
		if p.Field == "priority" {
			prio = p
		}
	}
	if err := repo.DiscardPendingChange(ctx, "p1", prio.ID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.Priority != "Medium" || strings.Join(iss.Labels, ",") != "q" || !iss.Pending {
		t.Errorf("only the discarded field reverts: %+v", iss)
	}
	if err := repo.DiscardPendingChange(ctx, "p1", 9999); !errors.Is(err, journal.ErrNotFound) {
		t.Errorf("unknown id: %v", err)
	}
	n, err := repo.DiscardAllPendingChanges(ctx, "p1")
	if err != nil || n != 2 {
		t.Fatalf("discard all: %d %v", n, err)
	}
	if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1"); iss.Pending || strings.Join(iss.Labels, ",") != "a" {
		t.Errorf("labels reverted: %+v", iss)
	}
	if _, err := repo.GetIssue(ctx, "p1", k); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("discarding a create row removes the draft: %v", err)
	}
	act, _ := repo.ListActivity(ctx, "p1", "", 0)
	discards := 0
	for _, a := range act {
		if a.Action == "discard" {
			discards++
		}
	}
	if discards != 3 {
		t.Errorf("every discard is audited: %d of %+v", discards, act)
	}
}

func TestMarkCommittedSetBaseVersionAndDiscardKey(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.EditField(ctx, "p1", "PLAT-1", "summary", "mine")
	_ = repo.EditField(ctx, "p1", "PLAT-1", "assignee", "me")
	if err := repo.SetBaseVersion(ctx, "p1", "PLAT-1", "v9"); err != nil {
		t.Fatal(err)
	}
	pend, _ := repo.PendingForKey(ctx, "p1", "PLAT-1")
	for _, p := range pend {
		if p.BaseVersion != "v9" {
			t.Errorf("rebased: %+v", p)
		}
	}
	if err := repo.MarkCommitted(ctx, "p1", pend[:1]); err != nil {
		t.Fatal(err)
	}
	if rest, _ := repo.PendingForKey(ctx, "p1", "PLAT-1"); len(rest) != 1 {
		t.Errorf("one row committed, one left: %+v", rest)
	}
	n, err := repo.DiscardKey(ctx, "p1", "PLAT-1")
	if err != nil || n != 1 {
		t.Errorf("DiscardKey: %d %v", n, err)
	}
	if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1"); iss.Pending {
		t.Errorf("nothing pending after DiscardKey: %+v", iss)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	seen := map[string]int{}
	for _, a := range act {
		seen[a.Action]++
	}
	if seen["override"] != 1 || seen["commit"] != 1 || seen["discard"] != 1 {
		t.Errorf("audit actions: %v", seen)
	}
}

func TestFieldValueAndSplitLabels(t *testing.T) {
	iss := backend.Issue{Summary: "s", Priority: "p", Assignee: "a", Labels: []string{"x", "y"}, StoryPoints: pts(2.5)}
	for field, want := range map[string]string{"summary": "s", "priority": "p", "assignee": "a", "labels": "x, y", "storyPoints": "2.5", "description": "text"} {
		if got := issuerepo.FieldValue(iss, "text", field); got != want {
			t.Errorf("FieldValue(%s) = %q, want %q", field, got, want)
		}
	}
	if got := issuerepo.FieldValue(backend.Issue{}, "", "storyPoints"); got != "" {
		t.Errorf("nil points = %q", got)
	}
	if got := backend.SplitLabels(" a ,, b,c "); strings.Join(got, "|") != "a|b|c" {
		t.Errorf("SplitLabels = %v", got)
	}
	if got := backend.SplitLabels(""); len(got) != 0 {
		t.Errorf("empty = %v", got)
	}
	if got := backend.FormatPoints(pts(2.5)); got != "2.5" {
		t.Errorf("FormatPoints = %q", got)
	}
	if _, err := backend.ParsePoints("eight"); err == nil {
		t.Error("ParsePoints must reject text")
	}
}
