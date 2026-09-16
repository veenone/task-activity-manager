package committer_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// An epic Jira refuses holds the edit parenting a real story under it, the
// link drafted from it, and the link drafted to it. None of them is a
// failure: nothing about those rows was sent, and the next Commit sends them
// once the epic exists.
func TestAnEditAndTheLinksWaitingOnARefusedEpicAreHeldNotSent(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	epic, err := h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeEpic, Summary: "Epic"})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.repo.EditField(ctx, "p1", "PLAT-2", "parentKey", epic); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.AddLink(ctx, "p1", epic, backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: "XT-9"}); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.AddLink(ctx, "p1", "PLAT-1", backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: epic}); err != nil {
		t.Fatal(err)
	}
	h.jira.createErrFor["Epic"] = errors.New("POST failed: 400 Epic Name is required")

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 1 || res.Failures[0].Key != epic {
		t.Errorf("failures = %+v, want only the epic's", res.Failures)
	}
	if len(h.jira.updates) != 0 || len(h.jira.links) != 0 {
		t.Errorf("nothing naming the epic was sent: updates %v, links %v", h.jira.updates, h.jira.links)
	}
	want := "waits for " + epic + ", which Jira refused"
	var sawEdit, sawFrom, sawTo bool
	for _, held := range res.Held {
		if held.Reason != want {
			t.Errorf("held %+v, want reason %q", held, want)
		}
		switch {
		case held.Key == "PLAT-2" && held.EntityType == issuerepo.EntityIssue && held.RowID == 0:
			sawEdit = true
		case held.Key == epic && held.EntityType == issuerepo.EntityLink:
			sawFrom = true
		case held.Key == "PLAT-1" && held.EntityType == issuerepo.EntityLink && held.RowID != 0:
			sawTo = true
		}
	}
	if !sawEdit || !sawFrom || !sawTo {
		t.Errorf("held = %+v, want the edit and both links", res.Held)
	}
}

// A rewriting bug that leaves a placeholder in a reference must end as a
// failure TAM owns, not as a 400 Jira answers. The board order never holds a
// draft, so one that does stands in for such a bug: the rank would anchor
// against it.
func TestTheFirewallStopsAPlaceholderRewritingMissed(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.RankIssue(ctx, "p1", "PLAT-1", "PLAT-2", false, demoBoard); err != nil {
		t.Fatal(err)
	}
	h.order.order[demoBoard] = []string{"TAM-NEW-77", "PLAT-1"}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.pushed) != 0 {
		t.Fatalf("the rank reached Jira: %v", h.jira.pushed)
	}
	if len(res.Failures) != 1 || res.Failures[0].Retryable || !strings.HasPrefix(res.Failures[0].Error, "internal error: ") {
		t.Errorf("failures = %+v", res.Failures)
	}
}

// Free text is not a reference: a summary that happens to read like a draft
// key is created like any other.
func TestADraftWhoseSummaryReadsLikeAPlaceholderCommits(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, err := h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "TAM-NEW-12 follow-up"}); err != nil {
		t.Fatal(err)
	}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) != 1 || len(res.Failures) != 0 || len(res.Held) != 0 || res.Remaining != 0 {
		t.Errorf("result = %+v", res)
	}
}

// discardCreate drops a draft's create row the way the Pending changes
// dialog does, leaving every row that names the draft behind.
func discardCreate(t *testing.T, h harness, key string) {
	t.Helper()
	ctx := context.Background()
	pend, err := h.repo.PendingForKey(ctx, "p1", key)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pend {
		if p.EntityType == issuerepo.EntityIssueCreate {
			if err := h.repo.DiscardPendingChange(ctx, "p1", p.ID); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no create row under %s: %+v", key, pend)
}

func TestAnEditParentingUnderADiscardedDraftFailsWithWhatToDo(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	epic, err := h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeEpic, Summary: "Epic"})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.repo.EditField(ctx, "p1", "PLAT-2", "parentKey", epic); err != nil {
		t.Fatal(err)
	}
	discardCreate(t, h, epic)

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	want := "its parent " + epic + " is not a draft this Commit could create; set the parent again"
	if len(res.Failures) != 1 || res.Failures[0].Key != "PLAT-2" || res.Failures[0].Error != want || res.Failures[0].Retryable {
		t.Errorf("failures = %+v", res.Failures)
	}
	if len(h.jira.updates) != 0 {
		t.Errorf("updates = %v", h.jira.updates)
	}
}

func TestALinkToADiscardedDraftFailsWithWhatToDo(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	target, err := h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "Target"})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.repo.AddLink(ctx, "p1", "PLAT-1", backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: target}); err != nil {
		t.Fatal(err)
	}
	discardCreate(t, h, target)

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	want := "its target " + target + " is not a draft this Commit could create; add the link again"
	if len(res.Failures) != 1 || res.Failures[0].Key != "PLAT-1" || res.Failures[0].Error != want || res.Failures[0].Retryable {
		t.Errorf("failures = %+v", res.Failures)
	}
	if len(h.jira.links) != 0 {
		t.Errorf("links = %v", h.jira.links)
	}
}
