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

// A rewriting bug that leaves a placeholder in a payload must end as a
// failure TAM owns, not as a 400 Jira answers.
func TestTheFirewallStopsAPlaceholderRewritingMissed(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	temp, err := h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "Leaky"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Exec(`UPDATE pending_change SET after_val = ? WHERE profile_id = 'p1' AND entity_key = ?`,
		`{"type":"task","summary":"Leaky","labels":[],"extra":{"customfield_10001":"TAM-NEW-77"}}`, temp); err != nil {
		t.Fatal(err)
	}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.creates) != 0 {
		t.Fatalf("the payload reached Jira: %+v", h.jira.creates)
	}
	if len(res.Failures) != 1 || res.Failures[0].Retryable || !strings.HasPrefix(res.Failures[0].Error, "internal error: ") {
		t.Errorf("failures = %+v", res.Failures)
	}
}
